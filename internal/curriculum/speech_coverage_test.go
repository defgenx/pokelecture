package curriculum

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

// The child cannot read: every sentence the frontend shows as an instruction
// or speaks must have a generated voice line. This scans web/*.js for literal
// hint('…'), say('…'), spoken('…') and Arcade screen('title', '…') calls and fails on any
// sentence missing from UIPhrases.
var spokenLiteral = regexp.MustCompile(`(?:\bhint|\bsay|\bspoken|this\.screen\('(?:[^'\\]|\\.)*',\s*)\(?\s*'((?:[^'\\]|\\.)*)'`)

func TestEveryShownInstructionIsSpoken(t *testing.T) {
	known := map[string]bool{}
	for _, p := range UIPhrases {
		known[p] = true
	}
	files, _ := filepath.Glob("../../web/*.js")
	if len(files) == 0 {
		t.Fatal("no web/*.js found")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range spokenLiteral.FindAllStringSubmatch(string(b), -1) {
			text := strings.ReplaceAll(m[1], `\'`, "'")
			if text != "" && !known[text] {
				t.Errorf("%s: %q est affiché ou dit, mais absent de UIPhrases — il n'aura pas de voix", filepath.Base(f), text)
			}
		}
	}
}

// Every server-side activity instruction is read out too (web/app.js hint()).
func TestEveryInstructionIsSpoken(t *testing.T) {
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, p := range UIPhrases {
		known[p] = true
	}
	for _, ep := range c.Episodes {
		s := c.BuildSession(&ep, emptyState(), testNow)
		for _, a := range s.Activities {
			if a.Instruction != "" && !known[a.Instruction] {
				t.Errorf("%s/%s: consigne %q absente de UIPhrases", ep.ID, a.Kind, a.Instruction)
			}
		}
	}
}

var testNow = time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)

func emptyState() progress.State {
	return progress.State{Episodes: map[string]progress.EpisodeStat{}, Items: map[string]progress.Item{}}
}
