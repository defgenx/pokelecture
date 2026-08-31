// Command pokecontent prepares the offline assets: the French voice track and
// the Pokémon sprites. Run it after every change to data/.
//
//	pokecontent check                 # validate the curriculum, report missing audio
//	pokecontent audio [-voice Thomas] # generate the missing .m4a with macOS `say`
//	pokecontent sprites               # download sprites for every Pokémon used
package main

import (
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
	"sync"
	"time"

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
	voice := fs.String("voice", "Thomas", "voix macOS française (say -v '?')")
	workers := fs.Int("workers", 6, "générations en parallèle")
	force := fs.Bool("force", false, "régénérer même si le fichier existe")
	_ = fs.Parse(os.Args[2:])

	cur, err := curriculum.Load(filepath.Join(*root, "data"))
	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "check":
		check(cur, *root)
	case "audio":
		if err := genAudio(cur, *root, *voice, *workers, *force); err != nil {
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
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pokecontent {check|audio|sprites|prune} [options]")
	os.Exit(2)
}

// prune deletes generated audio no longer referenced by the curriculum — the
// leftovers from every reworded story or retuned sound. Recordings made by a
// parent live in audio/recorded/ and are never touched.
func prune(cur *curriculum.Curriculum, root string) error {
	dir := filepath.Join(root, "web", "audio")

	keep := map[string]bool{}
	for _, it := range cur.SpeechTexts() {
		keep[speech.Key(it.Text, it.Style)+".m4a"] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var removed int
	var freed int64
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".m4a" || keep[e.Name()] {
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
	fmt.Printf("  ✓ %d fichier(s) obsolète(s) supprimés (%.1f Mo libérés), %d conservés\n",
		removed, float64(freed)/(1<<20), len(keep))
	return nil
}

func check(cur *curriculum.Curriculum, root string) {
	errs := cur.Validate()
	for _, e := range errs {
		fmt.Printf("  ✗ %v\n", e)
	}

	texts := cur.SpeechTexts()
	var missing int
	for _, it := range texts {
		path := filepath.Join(root, "web", "audio", speech.Key(it.Text, it.Style)+".m4a")
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
}

func genAudio(cur *curriculum.Curriculum, root, voice string, workers int, force bool) error {
	if _, err := exec.LookPath("say"); err != nil {
		return fmt.Errorf("`say` introuvable : la génération audio nécessite macOS (le navigateur prendra le relais sinon)")
	}
	outDir := filepath.Join(root, "web", "audio")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "pokelecture-say")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	texts := cur.SpeechTexts()
	var jobs []job
	for _, it := range texts {
		if !force {
			if _, err := os.Stat(filepath.Join(outDir, speech.Key(it.Text, it.Style)+".m4a")); err == nil {
				continue
			}
		}
		jobs = append(jobs, job{text: it.Text, style: it.Style})
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].text < jobs[j].text })

	if len(jobs) == 0 {
		fmt.Println("  ✓ audio déjà complet")
		return nil
	}
	fmt.Printf("  génération de %d fichier(s) avec la voix %s…\n", len(jobs), voice)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		done  int
		fails []string
		ch    = make(chan job)
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := range ch {
				err := synth(j, voice, tmpDir, outDir, worker)
				mu.Lock()
				done++
				if err != nil {
					fails = append(fails, fmt.Sprintf("%q: %v", j.text, err))
				} else if done%25 == 0 || done == len(jobs) {
					fmt.Printf("  … %d/%d\n", done, len(jobs))
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

	for _, f := range fails {
		fmt.Printf("  ✗ %s\n", f)
	}
	fmt.Printf("  ✓ %d fichier(s) écrits dans %s\n", len(jobs)-len(fails), outDir)
	return nil
}

// synth speaks one text to AIFF then transcodes to AAC/m4a, which Chrome on
// Android plays without a hiccup and which is ~10x smaller than the AIFF.
func synth(j job, voice, tmpDir, outDir string, worker int) error {
	aiff := filepath.Join(tmpDir, fmt.Sprintf("w%d.aiff", worker))
	out := filepath.Join(outDir, speech.Key(j.text, j.style)+".m4a")

	say := exec.Command("say",
		"-v", voice,
		"-r", fmt.Sprint(j.style.Rate()),
		"-o", aiff,
		j.text,
	)
	if b, err := say.CombinedOutput(); err != nil {
		return fmt.Errorf("say: %v: %s", err, b)
	}
	conv := exec.Command("afconvert", "-f", "m4af", "-d", "aac", "-b", "48000", aiff, out)
	if b, err := conv.CombinedOutput(); err != nil {
		return fmt.Errorf("afconvert: %v: %s", err, b)
	}
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
	fmt.Printf("  ✓ %d sprite(s) téléchargés, %d déjà présents\n", got, skipped)
	return nil
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
