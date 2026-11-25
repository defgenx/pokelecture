package curriculum

import (
	"fmt"
	"strings"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

// The tall grass ("les hautes herbes"): arcade Pokémon with a syllable split
// in data/arcade.json can be met and caught by reading their name. A name
// enters the grass at the first episode whose taught graphemes read it the
// way the full alphabet does — before that, greedy segmentation would teach a
// wrong reading (Loupio as l-o-u-p-i-o before "ou" exists).

// Wild reports whether an arcade Pokémon can appear in the tall grass.
func (p Pokemon) Wild() bool { return len(p.Syllables) > 0 && !p.Water }

// Fish reports whether an arcade Pokémon lives in the water: met only with
// the fishing rod (README « La pêche »). Same reading rules as the grass.
func (p Pokemon) Fish() bool { return len(p.Syllables) > 0 && p.Water }

// RodFrom is the first route that can earn the fishing rod; RodStars the
// stars needed on it or any later route — 4 is at least 90 % right first try.
const (
	RodFrom  = "ep13"
	RodStars = 4
)

// HasRod reports whether the child has earned the fishing rod.
func (c *Curriculum) HasRod(st progress.State) bool {
	from := c.Index(RodFrom)
	if from < 0 {
		return false
	}
	for _, ep := range c.Episodes[from:] {
		if st.Episodes[ep.ID].BestStars >= RodStars {
			return true
		}
	}
	return false
}

// allGraphemes is every grapheme, longest first, as KnownGraphemes orders them.
func (c *Curriculum) allGraphemes() []string {
	return c.KnownGraphemes(len(c.Episodes) - 1)
}

// WildUnlock is the index of the first episode after which the name reads
// correctly, or -1 if no episode ever teaches it.
func (c *Curriculum) WildUnlock(p Pokemon) int {
	full, err := Segment(p.Name, c.allGraphemes())
	if err != nil {
		return -1
	}
	for i := range c.Episodes {
		got, err := Segment(p.Name, c.KnownGraphemes(i))
		if err == nil && strings.Join(got, "|") == strings.Join(full, "|") {
			return i
		}
	}
	return -1
}

// WildPool is every tall-grass Pokémon readable once episode lastDone is done.
func (c *Curriculum) WildPool(lastDone int) []Pokemon {
	var out []Pokemon
	for _, p := range c.Arcade {
		if !p.Wild() {
			continue
		}
		if at := c.WildUnlock(p); at >= 0 && at <= lastDone {
			out = append(out, p)
		}
	}
	return out
}

// WildTotal counts the Pokémon the tall grass can ever offer.
func (c *Curriculum) WildTotal() int {
	n := 0
	for _, p := range c.Arcade {
		if p.Wild() && c.WildUnlock(p) >= 0 {
			n++
		}
	}
	return n
}

var vowelGraphemes = map[string]bool{
	"a": true, "i": true, "o": true, "u": true, "é": true, "è": true, "ê": true, "e": true, "y": true,
	"ou": true, "oo": true, "oi": true, "on": true, "om": true, "an": true, "am": true, "en": true,
	"in": true, "ain": true, "oin": true, "ien": true, "ui": true, "au": true, "eau": true, "eu": true,
	"ai": true, "ei": true, "er": true, "ge": true, "gi": true, "ce": true, "ci": true,
}

var nasalGraphemes = map[string]bool{
	"on": true, "om": true, "an": true, "am": true, "en": true, "in": true, "ain": true, "oin": true, "ien": true,
}

// readingTraps lists why a correctly segmented name would still be read
// wrong: the classes of mistakes the decodability check cannot see.
func readingTraps(gs []string) []string {
	var out []string
	for k, g := range gs {
		var prev, next string
		if k > 0 {
			prev = gs[k-1]
		}
		if k+1 < len(gs) {
			next = gs[k+1]
		}
		switch {
		case nasalGraphemes[g] && next != "" && (vowelGraphemes[next] || strings.ContainsAny(next[:1], "aeiouy")):
			out = append(out, "nasale suivie d'une voyelle ("+g+next+")")
		case g == "er" && next != "":
			out = append(out, "er au milieu du mot")
		case g == "s" && vowelGraphemes[prev] && vowelGraphemes[next]:
			out = append(out, "s entre deux voyelles (= z)")
		case (g == "c" || g == "g") && next != "" && strings.ContainsAny(next, "eiyéèê") && strings.IndexAny(next, "eiyéèê") == 0:
			out = append(out, g+" devant e/i")
		case g == "y" && vowelGraphemes[prev]:
			out = append(out, "y après une voyelle")
		}
	}
	return out
}

func (c *Curriculum) validateWild() []error {
	var errs []error
	all := c.allGraphemes()
	for _, p := range c.Arcade {
		if !p.Wild() && !p.Fish() {
			continue
		}
		where := fmt.Sprintf("arcade.json: %s (#%d)", p.Name, p.ID)
		if strings.ToLower(strings.Join(p.Syllables, "")) != strings.ToLower(p.Name) {
			errs = append(errs, fmt.Errorf("%s: découpage %v ne recompose pas le nom", where, p.Syllables))
		}
		if p.SayAs != "" {
			errs = append(errs, fmt.Errorf("%s: un nom qui a besoin d'un say_as ne se lit pas tel quel, retire ses syllabes", where))
		}
		gs, err := Segment(p.Name, all)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", where, err))
			continue
		}
		for _, t := range readingTraps(gs) {
			errs = append(errs, fmt.Errorf("%s: piège de lecture — %s", where, t))
		}
	}
	return errs
}
