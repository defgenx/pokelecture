package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// audit flags sound and syllable clips whose length betrays a mispronunciation:
// a neural voice that spells a chunk out ("k, s, e" / "e accent grave") takes
// several times longer than one that says it. Listen to what it lists.
func audit(cur *curriculum.Curriculum, root string, tsv bool) error {
	type clip struct {
		text  string
		style speech.Style
		secs  float64
	}
	durRe := regexp.MustCompile(`estimated duration: ([0-9.]+)`)
	var clips []clip
	for _, it := range cur.SpeechTexts() {
		if it.Style != speech.StyleSound && it.Style != speech.StyleSyllable {
			continue
		}
		path := filepath.Join(root, "web", "audio", speech.Key(it.Text, it.Style)+speech.Ext)
		if tsv {
			fmt.Printf("%s\t%s\t%s\n", it.Style, it.Text, path)
			continue
		}
		out, err := exec.Command("afinfo", path).Output()
		if err != nil {
			fmt.Printf("  ? %-10s %q : fichier absent\n", it.Style, it.Text)
			continue
		}
		m := durRe.FindSubmatch(out)
		if m == nil {
			continue
		}
		secs, _ := strconv.ParseFloat(string(m[1]), 64)
		clips = append(clips, clip{it.Text, it.Style, secs})
	}
	if tsv {
		return nil
	}
	sort.Slice(clips, func(i, j int) bool { return clips[i].secs > clips[j].secs })

	// One spoken syllable at the slow rate lasts ~0.4–0.8 s; budget a little per
	// extra letter so "chien" is not flagged next to "a".
	var flagged int
	for _, c := range clips {
		budget := 0.85 + 0.06*float64(utf8.RuneCountInString(c.text))
		mark := " "
		if c.secs > budget {
			mark = "✗"
			flagged++
		}
		fmt.Printf("  %s %5.2fs  %-9s %q\n", mark, c.secs, c.style, c.text)
	}
	fmt.Printf("\n  %d clip(s) suspect(s) sur %d\n", flagged, len(clips))
	return nil
}
