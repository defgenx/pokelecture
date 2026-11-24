package curriculum

import (
	"strings"
	"sync"
)

// allSigns is every grapheme, longest first, set by Load: the item builders
// are package functions and need it to tell two syllables that sound alike.
var allSigns []string

// soundKeys caches soundKey: content is fixed while the server runs, and the
// minimal-pair list compares every syllable with every other. Reset by Load.
var soundKeys sync.Map

// soundKey is a syllable's sound: each sign replaced by the first of its
// homophone group — "san" and "sen" share "s·an", "ca" and "ka" "c·a".
func soundKey(syl string) []string {
	if v, ok := soundKeys.Load(syl); ok {
		return v.([]string)
	}
	k := computeSoundKey(syl)
	soundKeys.Store(syl, k)
	return k
}

func computeSoundKey(syl string) []string {
	gs, err := Segment(syl, allSigns)
	if err != nil {
		return nil
	}
	for i, g := range gs {
		gs[i] = soundRep(g)
	}
	return gs
}

func soundRep(g string) string {
	for _, grp := range Homophones {
		for _, x := range grp {
			if x == g {
				return grp[0]
			}
		}
	}
	return g
}

// sameSound: two syllables the ear cannot tell apart.
func sameSound(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ka, kb := soundKey(strings.ToLower(a)), soundKey(strings.ToLower(b))
	return ka != nil && strings.Join(ka, "·") == strings.Join(kb, "·")
}

// minimalPairs are the consonants a young ear mixes up (b/d, p/b, f/v, ch/j…):
// syllables differing by one of these, and only that, train discrimination.
var minimalPairs = [][2]string{
	{"b", "d"}, {"p", "b"}, {"f", "v"}, {"ch", "j"}, {"t", "d"},
	{"c", "g"}, {"s", "ch"}, {"m", "n"}, {"s", "z"}, {"p", "t"},
}

// MinimalPair: same sounds but one, and that one a confusable consonant pair.
func MinimalPair(a, b string) bool {
	ka, kb := soundKey(strings.ToLower(a)), soundKey(strings.ToLower(b))
	if ka == nil || kb == nil || len(ka) != len(kb) {
		return false
	}
	diff := -1
	for i := range ka {
		if ka[i] != kb[i] {
			if diff >= 0 {
				return false
			}
			diff = i
		}
	}
	if diff < 0 {
		return false
	}
	for _, p := range minimalPairs {
		if (ka[diff] == p[0] && kb[diff] == p[1]) || (ka[diff] == p[1] && kb[diff] == p[0]) {
			return true
		}
	}
	return false
}

// SoundKey exposes soundKey to the mini-games ("s·an"), "" if unreadable.
func SoundKey(syl string) string { return strings.Join(soundKey(strings.ToLower(syl)), "·") }
