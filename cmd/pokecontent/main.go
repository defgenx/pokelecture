// Command pokecontent prepares the offline assets: the French voice track and
// the Pokémon sprites. Run it after every change to data/.
//
//	pokecontent check                        # validate the curriculum, report missing audio
//	pokecontent audio [-voice V] [-force] [-budget N]  # generate the missing .mp3 (ElevenLabs)
//	pokecontent sprites                      # download sprites for every Pokémon used
//	pokecontent audit                        # list sound/syllable clips by length, flag spelled-out ones
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

const spriteBase = "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites/pokemon"

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
	}

	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	root := fs.String("root", ".", "racine du projet")
	model := fs.String("model", elevenModel, "elevenlabs : modèle (eleven_v4, eleven_v3, eleven_flash_v2_5, eleven_multilingual_v2)")
	voice := fs.String("voice", os.Getenv("ELEVENLABS_VOICE_ID"), "identifiant de voix ElevenLabs (défaut : ELEVENLABS_VOICE_ID)")
	workers := fs.Int("workers", 6, "générations en parallèle")
	force := fs.Bool("force", false, "régénérer même si le fichier existe")
	budget := fs.Int("budget", 0, "audio : nombre maximum de caractères envoyés (0 = pas de limite)")
	dryRun := fs.Bool("dry-run", false, "audio : affiche ce qui serait généré et son coût, sans rien envoyer")
	tsv := fs.Bool("tsv", false, "audit : liste style/texte/fichier en TSV (pour un contrôle par reconnaissance vocale)")
	_ = fs.Parse(os.Args[2:])
	elevenModel = *model

	cur, err := curriculum.Load(filepath.Join(*root, "data"))
	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "check":
		check(cur, *root)
	case "audio":
		if err := genAudio(cur, *root, *voice, *workers, *force, *budget, *dryRun); err != nil {
			log.Fatal(err)
		}
	case "sprites":
		if err := genSprites(cur, *root, *force); err != nil {
			log.Fatal(err)
		}
		if err := genIcons(*root); err != nil {
			log.Fatal(err)
		}
	case "prune":
		if err := prune(cur, *root); err != nil {
			log.Fatal(err)
		}
	case "audit":
		if err := audit(cur, *root, *tsv); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pokecontent {check|audio|sprites|prune|audit} [options]")
	os.Exit(2)
}

// prune deletes generated audio no longer referenced by the curriculum — the
// leftovers from every reworded story or retuned sound. Recordings made by a
// parent live in audio/recorded/ and are never touched.
func prune(cur *curriculum.Curriculum, root string) error {
	dir := filepath.Join(root, "web", "audio")

	keep := map[string]bool{}
	for _, it := range cur.SpeechTexts() {
		keep[speech.Key(it.Text, it.Style)+speech.Ext] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var removed int
	var freed int64
	for _, e := range entries {
		// Anything not in the track goes — including every clip of an older
		// format or voice (.m4a): the track is one voice, one format.
		if ext := filepath.Ext(e.Name()); e.IsDir() || (ext != ".mp3" && ext != ".m4a") || keep[e.Name()] {
			continue
		}
		info, err := e.Info()
		if err == nil {
			freed += info.Size()
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
		removed++
	}
	man := loadManifest(dir)
	man.keep(map[string]bool(stems(keep)))
	if err := man.save(); err != nil {
		return err
	}
	fmt.Printf("  ✓ %d fichier(s) obsolète(s) supprimés (%.1f Mo libérés), %d conservés\n",
		removed, float64(freed)/(1<<20), len(keep))
	return nil
}

// stems turns {"x-1a2b3c.mp3": true} into {"x-1a2b3c": true}.
func stems(files map[string]bool) map[string]bool {
	out := make(map[string]bool, len(files))
	for f := range files {
		out[strings.TrimSuffix(f, speech.Ext)] = true
	}
	return out
}

func check(cur *curriculum.Curriculum, root string) {
	errs := cur.Validate()
	for _, e := range errs {
		fmt.Printf("  ✗ %v\n", e)
	}

	texts := cur.SpeechTexts()
	var missing int
	for _, it := range texts {
		path := filepath.Join(root, "web", "audio", speech.Key(it.Text, it.Style)+speech.Ext)
		if _, err := os.Stat(path); err != nil {
			missing++
		}
	}

	fmt.Printf("\n  %d épisode(s), %d grapheme(s), %d pokémon\n", len(cur.Episodes), len(cur.Graphemes), len(cur.Pokemon))
	for _, ep := range cur.Episodes {
		fmt.Printf("  · %-42s %2d syllabes  %2d mots  %d phrase(s)\n",
			ep.Fmt(), len(ep.Syllables), len(ep.Words), len(ep.Sentences))
	}
	fmt.Printf("\n  audio : %d/%d présents", len(texts)-missing, len(texts))
	if missing > 0 {
		fmt.Printf("  → lance `pokecontent audio`")
	}
	fmt.Println()

	if len(errs) > 0 {
		fmt.Printf("\n  %d problème(s) de contenu\n", len(errs))
		os.Exit(1)
	}
	fmt.Println("\n  ✓ contenu valide")
}

type job struct {
	text  string
	style speech.Style
	key   string
}

// styleRank orders the work when a budget cannot cover it all: the sounds and
// syllables are where the voice teaches, sentences come last.
var styleRank = map[speech.Style]int{
	speech.StyleSound: 0, speech.StyleSyllable: 1, speech.StyleWord: 2, speech.StyleNormal: 3,
}

// genAudio fills web/audio. Without -force it only makes missing clips; with
// -force it remakes every clip the chosen voice did not make yet — a clip
// already in voices.json under the same signature is never requested again,
// so a paid engine is paid once per line. budget caps the characters sent
// (0 = no cap); the most useful lines go first.
func genAudio(cur *curriculum.Curriculum, root, voice string, workers int, force bool, budget int, dryRun bool) error {
	if voice == "" {
		return fmt.Errorf("aucune voix : définis ELEVENLABS_VOICE_ID dans .env.local (scripts/voice-elevenlabs.sh --voices)")
	}
	if !dryRun {
		if os.Getenv("ELEVENLABS_API_KEY") == "" {
			return fmt.Errorf("ELEVENLABS_API_KEY n'est pas défini")
		}
		if _, err := exec.LookPath("afconvert"); err != nil {
			return fmt.Errorf("`afconvert` introuvable : le rognage du silence a besoin de macOS")
		}
	}
	outDir := filepath.Join(root, "web", "audio")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	man := loadManifest(outDir)
	sig := signature(voice)

	// Teaching order inside each style (SpeechTexts walks route 1 first), so a
	// budget that runs out leaves the late content for next time.
	var jobs []job
	for _, it := range cur.SpeechTexts() {
		key := speech.Key(it.Text, it.Style)
		_, err := os.Stat(filepath.Join(outDir, key+speech.Ext))
		exists := err == nil
		if exists && (!force || man.get(key) == sig) {
			continue
		}
		jobs = append(jobs, job{text: it.Text, style: it.Style, key: key})
	}
	sort.SliceStable(jobs, func(i, j int) bool { return styleRank[jobs[i].style] < styleRank[jobs[j].style] })

	total, pending := 0, len(jobs)
	for _, j := range jobs {
		total += utf8.RuneCountInString(j.text)
	}
	if budget > 0 {
		kept, spent := jobs[:0], 0
		for _, j := range jobs {
			n := utf8.RuneCountInString(j.text)
			if spent+n > budget {
				break
			}
			kept, spent = append(kept, j), spent+n
		}
		jobs = kept
	}
	planned := 0
	for _, j := range jobs {
		planned += utf8.RuneCountInString(j.text)
	}
	if dryRun {
		fmt.Printf("  %s : %d ligne(s) à faire (%d caractères) ; ce passage : %d ligne(s), %d caractères\n",
			sig, pending, total, len(jobs), planned)
		return nil
	}
	if len(jobs) == 0 {
		fmt.Println("  ✓ audio déjà complet pour cette voix")
		return nil
	}
	fmt.Printf("  génération de %d fichier(s), %d caractères (reste à faire après : %d) — %s…\n",
		len(jobs), planned, total-planned, sig)

	tmpDir, err := os.MkdirTemp("", "pokelecture-voice")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		done    int
		fails   []string
		stopped atomic.Bool
		ch      = make(chan job)
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range ch {
				if stopped.Load() {
					continue
				}
				err := synth(j, voice, tmpDir, outDir, worker)
				if errors.Is(err, errQuota) {
					stopped.Store(true)
				}
				if err == nil {
					man.set(j.key, sig)
				}
				mu.Lock()
				done++
				if err != nil {
					fails = append(fails, fmt.Sprintf("%q: %v", j.text, err))
				} else if done%25 == 0 {
					fmt.Printf("  … %d/%d\n", done, len(jobs))
					_ = man.save() // a run cut short keeps what it paid for
				}
				mu.Unlock()
			}
		}(i)
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
	if err := man.save(); err != nil {
		return err
	}

	for _, f := range fails {
		fmt.Printf("  ✗ %s\n", f)
	}
	if stopped.Load() {
		fmt.Println("  ⏸ quota atteint : arrêt propre. Ce qui est fait est gardé ; relance quand les crédits reviennent.")
	}
	fmt.Printf("  ✓ %d fichier(s) écrits dans %s\n", len(jobs)-len(fails), outDir)
	return nil
}

func genSprites(cur *curriculum.Curriculum, root string, force bool) error {
	dir := filepath.Join(root, "web", "sprites")
	artDir := filepath.Join(dir, "art")
	for _, d := range []string{dir, artDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	ids := make([]int, 0, len(cur.Pokemon))
	for id := range cur.Pokemon {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	client := &http.Client{Timeout: 30 * time.Second}
	var got, skipped int
	for _, id := range ids {
		targets := []struct{ url, path string }{
			{fmt.Sprintf("%s/%d.png", spriteBase, id), filepath.Join(dir, fmt.Sprintf("%d.png", id))},
			{fmt.Sprintf("%s/other/official-artwork/%d.png", spriteBase, id), filepath.Join(artDir, fmt.Sprintf("%d.png", id))},
		}
		for _, t := range targets {
			if !force {
				if fi, err := os.Stat(t.path); err == nil && fi.Size() > 0 {
					skipped++
					continue
				}
			}
			if err := download(client, t.url, t.path); err != nil {
				fmt.Printf("  ✗ #%d: %v\n", id, err)
				continue
			}
			got++
		}
	}
	// The official shiny artwork, for every Pokémon that can turn up shiny
	// (README « Les chromatiques »): full size for the course, arcade size for the rest.
	shinyDir := filepath.Join(artDir, "shiny")
	if err := os.MkdirAll(shinyDir, 0o755); err != nil {
		return err
	}
	shinies := map[int]int{} // id → side (0 = as published)
	for _, id := range ids {
		shinies[id] = 0
	}
	for _, p := range cur.Arcade {
		shinies[p.ID] = arcadeArt
	}
	for id, side := range shinies {
		path := filepath.Join(shinyDir, fmt.Sprintf("%d.png", id))
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 && !force {
			skipped++
			continue
		}
		url := fmt.Sprintf("%s/other/official-artwork/shiny/%d.png", spriteBase, id)
		var err error
		if side > 0 {
			err = downloadScaled(client, url, path, side)
		} else {
			err = download(client, url, path)
		}
		if err != nil {
			fmt.Printf("  ✗ chromatique #%d: %v\n", id, err)
			continue
		}
		got++
	}
	// The real Gigamax forms: the bosses' last-hit-point pictures.
	for _, g := range curriculum.Gigamax {
		path := filepath.Join(artDir, fmt.Sprintf("%d.png", g))
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 && !force {
			skipped++
			continue
		}
		if err := download(client, fmt.Sprintf("%s/other/official-artwork/%d.png", spriteBase, g), path); err != nil {
			fmt.Printf("  ✗ Gigamax #%d: %v\n", g, err)
			continue
		}
		got++
	}
	// Arcade extras only ever show as artwork in the mini-games, shrunk so the
	// whole roster stays light on the tablet.
	for _, p := range cur.Arcade {
		path := filepath.Join(artDir, fmt.Sprintf("%d.png", p.ID))
		if !force {
			if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
				skipped++
				continue
			}
		}
		if err := downloadScaled(client, fmt.Sprintf("%s/other/official-artwork/%d.png", spriteBase, p.ID), path, arcadeArt); err != nil {
			fmt.Printf("  ✗ #%d: %v\n", p.ID, err)
			continue
		}
		got++
	}
	fmt.Printf("  ✓ %d sprite(s) téléchargés, %d déjà présents\n", got, skipped)
	return nil
}

// arcadeArt is the side, in pixels, of an arcade-only Pokémon's artwork.
const arcadeArt = 240

// downloadScaled fetches a PNG and writes it shrunk to a size×size box.
func downloadScaled(client *http.Client, url, path string, size int) error {
	tmp := path + ".orig"
	if err := download(client, url, tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(out, scaleBox(img, size, size)); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// genIcons composes the app icons from Pikachu's artwork on the brand yellow.
// Tablets need real icons: iOS shows apple-touch-icon on the home screen (a
// transparent PNG gets a black plate), and Android wants a maskable icon whose
// subject survives the launcher cropping it to a circle.
func genIcons(root string) error {
	src := filepath.Join(root, "web", "sprites", "art", "25.png")
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("icônes: %w (lance `pokecontent sprites` d'abord)", err)
	}
	art, err := png.Decode(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("icônes: %s: %w", src, err)
	}

	brand := color.RGBA{R: 0xff, G: 0xcb, B: 0x05, A: 0xff}
	targets := []struct {
		name string
		size int
		art  int // the artwork's box inside the icon; small for maskable safe zone
	}{
		{"icon-180.png", 180, 150},
		{"icon-512.png", 512, 448},
		{"icon-mask.png", 512, 300},
	}
	for _, t := range targets {
		dst := image.NewRGBA(image.Rect(0, 0, t.size, t.size))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(brand), image.Point{}, draw.Src)
		scaled := scaleBox(art, t.art, t.art)
		off := (t.size - t.art) / 2
		draw.Draw(dst, image.Rect(off, off, off+t.art, off+t.art), scaled, image.Point{}, draw.Over)

		out, err := os.Create(filepath.Join(root, "web", t.name))
		if err != nil {
			return err
		}
		if err := png.Encode(out, dst); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	fmt.Printf("  ✓ %d icône(s) d'application générées\n", len(targets))
	return nil
}

// scaleBox downscales with a box filter — the right filter for shrinking, and
// small enough to keep the zero-dependency rule.
func scaleBox(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0 := sb.Min.Y + y*sb.Dy()/h
		y1 := sb.Min.Y + (y+1)*sb.Dy()/h
		if y1 == y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := sb.Min.X + x*sb.Dx()/w
			x1 := sb.Min.X + (x+1)*sb.Dx()/w
			if x1 == x0 {
				x1 = x0 + 1
			}
			var r, g, b, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					b += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			dst.Set(x, y, color.RGBA64{
				R: uint16(r / n), G: uint16(g / n), B: uint16(b / n), A: uint16(a / n),
			})
		}
	}
	return dst
}

func download(client *http.Client, url, path string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
