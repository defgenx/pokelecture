// Package speech maps a piece of French text to a stable audio asset key.
//
// The key is used both by the generator (cmd/pokecontent audio) that writes
// web/audio/<key>.m4a and by the server that hands URLs to the tablet. When a
// file is missing the frontend falls back to the browser's own French voice, so
// a missing key is a quality regression, never a crash.
package speech

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// Style controls how fast the voice reads. Syllables and single sounds need to
// be much slower than sentences or a five-year-old cannot hear the phoneme.
type Style string

const (
	StyleSound    Style = "sound"    // a single grapheme sound: "mmm"
	StyleSyllable Style = "syllable" // "ma", "chou"
	StyleWord     Style = "word"     // "tomate"
	StyleNormal   Style = "normal"   // sentences, story, Pokédex entries
)

// Rate is the `say -r` words-per-minute used for a style.
func (s Style) Rate() int {
	switch s {
	case StyleSound:
		// Sounds are single short utterances ("che", "te"), not drawn-out letter
		// runs, so they do not need the crawl a prolonged sound would.
		return 110
	case StyleSyllable:
		return 110
	case StyleWord:
		return 135
	default:
		return 165
	}
}

var accents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i",
	"ô", "o", "ö", "o",
	"ù", "u", "û", "u", "ü", "u",
	"ç", "c", "œ", "oe", "æ", "ae",
	"’", "'",
)

// Key returns the filename stem for text spoken in the given style.
func Key(text string, style Style) string {
	norm := accents.Replace(strings.ToLower(strings.TrimSpace(text)))

	var b strings.Builder
	dash := false
	for _, r := range norm {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 36 {
		slug = strings.Trim(slug[:36], "-")
	}
	if slug == "" {
		slug = "x"
	}

	sum := sha1.Sum([]byte(string(style) + "\x00" + text))
	return slug + "-" + hex.EncodeToString(sum[:])[:6]
}

// URL is the path the tablet fetches for this text. Relative on purpose: the
// game may be served under a sub-path (behind delvecch.io/pokelecture/), and a
// relative URL resolves against the document's <base href> while an absolute one
// would escape the mount point.
func URL(text string, style Style) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "audio/" + Key(text, style) + ".m4a"
}
