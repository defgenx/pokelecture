package curriculum

import (
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

func towerFloorWords(t *testing.T, c *Curriculum, st progress.State, floor int) []string {
	t.Helper()
	s, ok := c.BuildTower(st, floor, time.Now())
	if !ok {
		t.Fatal("no tower")
	}
	var out []string
	for _, r := range s.Activities[0].Fight.Rounds {
		out = append(out, r.Prompt)
	}
	return out
}

// Floors must not replay the same words, and a high floor draws longer words
// than floor 1.
func TestTowerWordsVaryAndClimb(t *testing.T) {
	c := loadCur(t)
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	for _, ep := range c.Episodes[:20] {
		st.Episodes[ep.ID] = progress.EpisodeStat{Completions: 1}
	}
	// Climbing floor after floor, as played: each floor's words are answered.
	st.Items = map[string]progress.Item{}
	start := time.Now()
	seen := map[string]int{}
	for f := 1; f <= 10; f++ {
		for _, w := range towerFloorWords(t, c, st, f) {
			seen[w]++
			st.Items["mot:"+w] = progress.Item{LastSeen: start.Add(time.Duration(f) * time.Minute)}
		}
	}
	for w, n := range seen {
		if n > 3 {
			t.Errorf("%s on %d of 10 floors", w, n)
		}
	}
	syl := func(floor int) float64 {
		total, n := 0, 0
		for i := 0; i < 30; i++ {
			s, _ := c.BuildTower(st, floor, time.Now())
			for _, r := range s.Activities[0].Fight.Rounds {
				total += len(r.Syllables)
				n++
			}
		}
		return float64(total) / float64(n)
	}
	if lo, hi := syl(1), syl(TowerClimb); hi <= lo {
		t.Errorf("floor %d averages %.2f syllables, floor 1 %.2f", TowerClimb, hi, lo)
	}
}

func TestTowerDicteesGrow(t *testing.T) {
	for floor, want := range map[int]int{9: 0, 10: 1, 19: 1, 20: 2, 30: 3, 90: 3} {
		if got := towerDictees(floor); got != want {
			t.Errorf("floor %d: %d dictations, want %d", floor, got, want)
		}
	}
	rounds := []Item{{Tray: []string{"ma", "to"}}, {Tray: []string{"pi", "lu"}}, {Tray: []string{"ra", "vé"}}}
	dictateLastN(rounds, 2)
	if rounds[0].Dictee || !rounds[1].Dictee || !rounds[2].Dictee {
		t.Errorf("want the last two dictated: %+v", rounds)
	}
}
