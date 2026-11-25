package curriculum

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

type owRegion struct {
	tree string
	rows []string
}

// overworldRegions reads OW_REGIONS out of web/overworld.js.
func overworldRegions(t *testing.T) map[string]owRegion {
	t.Helper()
	b, err := os.ReadFile("../../web/overworld.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "const OW_REGIONS = {")
	end := strings.Index(src[start:], "\n};")
	if start < 0 || end < 0 {
		t.Fatal("OW_REGIONS not found in web/overworld.js")
	}
	block := src[start : start+end]
	entry := regexp.MustCompile(`(?s)'([^']+)': \{\s*look: '[^']+', tree: '([^']+)',\s*map: \[(.*?)\]`)
	row := regexp.MustCompile(`'([^']*)'`)
	out := map[string]owRegion{}
	for _, m := range entry.FindAllStringSubmatch(block, -1) {
		r := owRegion{tree: m[2]}
		for _, rm := range row.FindAllStringSubmatch(m[3], -1) {
			r.rows = append(r.rows, rm[1])
		}
		out[m[1]] = r
	}
	return out
}

// checkMap lists what is wrong with one region map: its size, its places,
// everything reachable from the entrance, the stops met in order, and no
// trainer standing next to an exit or the professor (his dialog would win).
func checkMap(m []string) []string {
	const w, h = 13, 9
	if len(m) != h {
		return []string{fmt.Sprintf("%d rows, want %d", len(m), h)}
	}
	var errs []string
	pos := map[rune][2]int{}
	for y, r := range m {
		if len([]rune(r)) != w {
			return []string{fmt.Sprintf("row %d: %d columns, want %d", y, len([]rune(r)), w)}
		}
		for x, c := range []rune(r) {
			if !strings.ContainsRune("TW.=Fg<>OAPDEXK12345", c) {
				errs = append(errs, fmt.Sprintf("unknown tile %q at %d,%d", c, x, y))
			}
			if strings.ContainsRune("<>OAPDEXK12345", c) {
				if _, dup := pos[c]; dup {
					errs = append(errs, fmt.Sprintf("two %q", c))
				}
				pos[c] = [2]int{x, y}
			}
		}
	}
	for _, c := range "<>OAPDEX12345" {
		if _, ok := pos[c]; !ok {
			errs = append(errs, fmt.Sprintf("no %q", c))
		}
	}
	if len(errs) > 0 {
		return errs
	}
	if pos['<'][0] != 0 || pos['>'][0] != w-1 {
		errs = append(errs, "the exits must sit on the left and right edges")
	}
	at := func(x, y int) rune {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 'T'
		}
		return []rune(m[y])[x]
	}
	free := func(x, y int) bool { return !strings.ContainsRune("TWDE", at(x, y)) }
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	dist := map[[2]int]int{pos['<']: 0}
	queue := [][2]int{pos['<']}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if _, seen := dist[n]; !seen && free(n[0], n[1]) {
				dist[n] = dist[p] + 1
				queue = append(queue, n)
			}
		}
	}
	for y := range h {
		for x := range w {
			if _, ok := dist[[2]int{x, y}]; free(x, y) && !ok {
				errs = append(errs, fmt.Sprintf("%q at %d,%d cannot be reached", at(x, y), x, y))
			}
		}
	}
	prev := -1
	for _, c := range "12345" {
		d := dist[pos[c]]
		if d <= prev {
			errs = append(errs, fmt.Sprintf("stop %c is met before the one before it", c))
		}
		prev = d
	}
	if dist[pos['>']] < dist[pos['4']] {
		errs = append(errs, "the exit comes before stop 4")
	}
	for _, c := range "DE" {
		p, reach := pos[c], false
		for _, d := range dirs {
			n := at(p[0]+d[0], p[1]+d[1])
			if free(p[0]+d[0], p[1]+d[1]) {
				reach = true
			}
			if strings.ContainsRune("<>X", n) {
				errs = append(errs, fmt.Sprintf("trainer %c stands next to %q", c, n))
			}
		}
		if !reach {
			errs = append(errs, fmt.Sprintf("trainer %c cannot be reached", c))
		}
	}
	if g := strings.Count(strings.Join(m, ""), "g"); g < 4 {
		errs = append(errs, fmt.Sprintf("only %d tall grass tiles", g))
	}
	return errs
}

// Every region has its own map and tree, and every map is playable.
func TestEveryRegionHasItsOwnMap(t *testing.T) {
	c := loadCur(t)
	regions := overworldRegions(t)
	stops := map[string]int{}
	for _, ep := range c.Episodes {
		stops[ep.Region]++
	}
	seenMap, seenTree := map[string]string{}, map[string]string{}
	for name, n := range stops {
		r, ok := regions[name]
		if !ok {
			t.Errorf("%s: no map in OW_REGIONS", name)
			continue
		}
		if n > 5 {
			t.Errorf("%s: %d routes, a map has 5 stops", name, n)
		}
		for _, e := range checkMap(r.rows) {
			t.Errorf("%s: %s", name, e)
		}
		key := strings.Join(r.rows, "/")
		if other, dup := seenMap[key]; dup {
			t.Errorf("%s has the same map as %s", name, other)
		}
		seenMap[key] = name
		if other, dup := seenTree[r.tree]; dup {
			t.Errorf("%s has the same obstacle %s as %s", name, r.tree, other)
		}
		seenTree[r.tree] = name
	}
}

// The fisherman who gives the rod stands on the Côte Saphir, where RodFrom
// is: his line and the rod's voice line both name that region.
func TestFishermanOnTheCoteSaphir(t *testing.T) {
	c := loadCur(t)
	if !strings.Contains(strings.Join(overworldRegions(t)["Côte Saphir"].rows, ""), "K") {
		t.Error("no fisherman (K) on the Côte Saphir map")
	}
	if i := c.Index(RodFrom); i < 0 || c.Episodes[i].Region != "Côte Saphir" {
		t.Errorf("RodFrom %s is not a Côte Saphir route", RodFrom)
	}
}

// The checker itself: a blocked stop and out-of-order stops are caught.
func TestCheckMapCatchesBrokenMaps(t *testing.T) {
	good := []string{
		"TTTTTTTTTTTTT", "TT1==XTT.ATTT", "TT=T=T3==T5=T", "TT=D=W=T=T==T", "<==T=W=O=E==>",
		"Tggg=W=T=T=FT", "TggP=T=T4==FT", "Tggg2==T.F.TT", "TTTTTTTTTTTTT",
	}
	if errs := checkMap(good); len(errs) > 0 {
		t.Fatalf("good map rejected: %v", errs)
	}
	walled := append([]string(nil), good...)
	walled[1] = "TTT==XTT.ATTT" // stop 1 gone
	walled[2] = "TTTT=T3==T5=T"
	if len(checkMap(walled)) == 0 {
		t.Error("a map without stop 1 passed")
	}
	swapped := append([]string(nil), good...)
	swapped[1] = "TT2==XTT.ATTT"
	swapped[7] = "Tggg1==T.F.TT"
	if len(checkMap(swapped)) == 0 {
		t.Error("stops out of order passed")
	}
}
