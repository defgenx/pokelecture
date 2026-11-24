package curriculum

import (
	"strings"
	"testing"
)

func TestPartnerLevelCurve(t *testing.T) {
	if l, into, need := PartnerLevel(0); l != PartnerStartLevel || into != 0 || need != XPToNext(PartnerStartLevel) {
		t.Fatalf("PartnerLevel(0) = %d %d %d", l, into, need)
	}
	l, _, _ := PartnerLevel(XPToNext(5))
	if l != 6 {
		t.Fatalf("one level's worth of XP should give level 6, got %d", l)
	}
	if l, _, _ := PartnerLevel(1 << 30); l != PartnerMaxLevel {
		t.Fatalf("level should cap at %d, got %d", PartnerMaxLevel, l)
	}
}

func TestPartnerEvolutions(t *testing.T) {
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	line, ok := c.LineOf(4)
	if !ok || !c.IsStarter(4) || c.IsStarter(5) {
		t.Fatal("Salamèche must be a starter, Reptincel not")
	}
	for _, tc := range []struct{ level, want int }{{5, 4}, {15, 4}, {16, 5}, {31, 5}, {32, 6}, {80, 6}} {
		if got, _ := line.PartnerForm(tc.level); got != tc.want {
			t.Errorf("level %d: form #%d, want #%d", tc.level, got, tc.want)
		}
	}
	if next, at := line.NextEvolution(20); next != 6 || at != 32 {
		t.Errorf("next evolution from 20 = #%d at %d", next, at)
	}
	if next, _ := line.NextEvolution(40); next != 0 {
		t.Errorf("a final form has no next evolution, got #%d", next)
	}
}

func TestWildPoolGrowsWithTheCourse(t *testing.T) {
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	prev := -1
	for i := range c.Episodes {
		n := len(c.WildPool(i))
		if n < prev {
			t.Fatalf("the tall grass lost Pokémon after %s", c.Episodes[i].ID)
		}
		prev = n
	}
	if len(c.WildPool(0)) == 0 {
		t.Fatal("the tall grass should hold someone after route 1")
	}
	// A name only enters the grass once it reads right: Loupio needs "ou".
	for _, p := range c.WildPool(0) {
		if strings.Contains(strings.ToLower(p.Name), "ou") {
			t.Errorf("%s is in the route-1 grass but needs « ou »", p.Name)
		}
	}
}

func TestReadingTraps(t *testing.T) {
	for _, tc := range []struct {
		gs   []string
		trap bool
	}{
		{[]string{"p", "on", "y", "t", "a"}, true},       // Ponyta: on + vowel
		{[]string{"p", "oi", "s", "o", "n"}, true},       // s between vowels
		{[]string{"m", "a", "l", "o", "ss", "e"}, false}, // Malosse
	} {
		if got := len(readingTraps(tc.gs)) > 0; got != tc.trap {
			t.Errorf("%v: trap=%v, want %v", tc.gs, got, tc.trap)
		}
	}
}
