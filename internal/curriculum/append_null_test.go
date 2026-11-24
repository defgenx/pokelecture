package curriculum

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The DOM's own append() prints a null as the text "null" (and false as
// "false"). Anything that may be empty goes through put() (web/app.js) or
// inside el(), which both skip it. This flags a native .append( whose
// argument may be empty: a "… ? null :" or "… : null" ternary, a bare null,
// an "a && b", or a helper that returns null when there is nothing to show.
func TestNoNullInNativeAppend(t *testing.T) {
	files, _ := filepath.Glob("../../web/*.js")
	if len(files) == 0 {
		t.Fatal("no web/*.js found")
	}
	call := regexp.MustCompile(`[\w$\])]\.append\(`)
	empty := regexp.MustCompile(`(:\s*(null|undefined|false)$)|\?\s*(null|undefined|false)\s*:|^(null|undefined)$|^[^?]*&&|^(ballsWon|Partner\.gainRow|Trainer\.chip)\(`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, loc := range call.FindAllStringIndex(src, -1) {
			for _, arg := range topLevelArgs(src, loc[1]) {
				a := strings.TrimSpace(arg)
				if strings.HasPrefix(a, "...") { // a spread: filtered by its author
					continue
				}
				if empty.MatchString(a) {
					line := strings.Count(src[:loc[0]], "\n") + 1
					t.Errorf("%s:%d: append() would print an empty value as text — use put(): %.70q",
						filepath.Base(f), line, a)
				}
			}
		}
	}
}

// topLevelArgs splits the call arguments starting at src[i] (just after the
// opening parenthesis) on top-level commas, skipping strings, template
// literals, comments and regex literals.
func topLevelArgs(src string, i int) []string {
	var args []string
	depth, start := 0, i
	prev := byte('(') // last significant character, to tell a regex from a division
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			if e := strings.Index(src[i+2:], "*/"); e >= 0 {
				i += e + 4
				continue
			}
			return args
		case c == '\'' || c == '"' || c == '`':
			i = skipString(src, i)
			prev = c
			continue
		case c == '/' && strings.IndexByte("(,=:!&|?{;[+", prev) >= 0:
			i = skipString(src, i) // a regex literal
			prev = 'r'
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				return append(args, src[start:i])
			}
			depth--
		case c == ',' && depth == 0:
			args = append(args, src[start:i])
			start = i + 1
		}
		if c != ' ' && c != '\n' && c != '\t' && c != '\r' {
			prev = c
		}
		i++
	}
	return args
}

// skipString returns the index just past the string, template or regex
// literal opening at src[i]. A template's ${…} may hold nested literals.
func skipString(src string, i int) int {
	q := src[i]
	i++
	for i < len(src) {
		switch c := src[i]; {
		case c == '\\':
			i += 2
			continue
		case q == '/' && c == '[': // a class: [/] does not end the regex
			for i < len(src) && src[i] != ']' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
		case q == '`' && c == '$' && i+1 < len(src) && src[i+1] == '{':
			depth := 1
			i += 2
			for i < len(src) && depth > 0 {
				switch src[i] {
				case '{':
					depth++
				case '}':
					depth--
				case '\'', '"', '`':
					i = skipString(src, i) - 1
				}
				i++
			}
			continue
		case c == q:
			return i + 1
		case c == '\n' && q != '`':
			return i + 1 // unterminated: give up on this literal
		}
		i++
	}
	return i
}
