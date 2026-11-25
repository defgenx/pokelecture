package curriculum

import (
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

// Fights against people Téracristallise, the others do not; a boss with a
// real Gigamax form shows it, the rest Dynamax.
func TestGigamaxAndTera(t *testing.T) {
	c := loadCur(t)
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	for _, ep := range c.Episodes {
		st.Episodes[ep.ID] = progress.EpisodeStat{Completions: 1}
	}
	fightOf := func(id string) *Fight {
		ep, _ := c.Episode(id)
		for _, a := range c.BuildSession(ep, st, time.Now()).Activities {
			if a.Fight != nil {
				return a.Fight
			}
		}
		t.Fatalf("%s: no fight", id)
		return nil
	}
	if f := fightOf("ep11"); f.Gmax != "sprites/art/10196.png" || f.Tera { // Dracaufeu, a route boss
		t.Errorf("ep11: gmax %q tera %v", f.Gmax, f.Tera)
	}
	if f := fightOf("ep03b"); f.Gmax != "" || f.Tera { // Artikodor: no Gigamax form
		t.Errorf("ep03b: gmax %q tera %v", f.Gmax, f.Tera)
	}
	if f := fightOf("ep29"); !f.Tera { // the Champion is a person
		t.Error("ep29: the Champion's fight has no Téracristal")
	}
	s, ok := c.BuildTrainer(st, 0, 0, time.Now())
	if !ok || !s.Activities[0].Fight.Tera {
		t.Error("a map trainer's fight has no Téracristal")
	}
}
