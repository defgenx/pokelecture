package curriculum

import (
	"os"
	"regexp"
	"testing"
)

// CSS keyframe names are global: a second definition silently replaces the
// first for every element using it.
func TestNoDuplicateKeyframes(t *testing.T) {
	b, err := os.ReadFile("../../web/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`@keyframes\s+([\w-]+)`).FindAllStringSubmatch(string(b), -1) {
		if seen[m[1]] {
			t.Errorf("@keyframes %s is defined twice in web/styles.css", m[1])
		}
		seen[m[1]] = true
	}
}
