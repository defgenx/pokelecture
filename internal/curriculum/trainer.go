package curriculum

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// Map trainers: two per region (web/overworld.js), each one a short reading
// battle a day — three words against a Pokémon that changes every day.

// TrainerPrefix starts a trainer battle's session id: "dresseur-<region>-<n>".
const TrainerPrefix = "dresseur-"

// TrainerTaunt opens a trainer battle; TrainerBeaten is said by a trainer
// already beaten today.
const (
	TrainerTaunt  = "Un Dresseur veut se battre ! Lis vite pour gagner !"
	TrainerBeaten = "Bravo, tu m'as battu ! Reviens demain pour une revanche."
	TrainerWon    = "Tu as battu le Dresseur ! Voici ta récompense."
)

// TrainerInfo identifies the trainer of a battle session.
type TrainerInfo struct {
	Region int    `json:"region"`
	N      int    `json:"n"`
	Key    string `json:"key"` // "region:n", as progress.Today.Beaten stores it
}

// ParseTrainer reads a trainer session id.
func ParseTrainer(id string) (region, n int, ok bool) {
	rest, found := strings.CutPrefix(id, TrainerPrefix)
	if !found {
		return 0, 0, false
	}
	a, b, found := strings.Cut(rest, "-")
	if !found {
		return 0, 0, false
	}
	region, err1 := strconv.Atoi(a)
	n, err2 := strconv.Atoi(b)
	return region, n, err1 == nil && err2 == nil && n >= 0 && n < 2 && region >= 0
}

// BuildTrainer is a three-word battle on everything learned so far, against
// a non-legendary Pokémon drawn for this trainer and this day.
func (c *Curriculum) BuildTrainer(st progress.State, region, n int, now time.Time) (Session, bool) {
	sess, ok := c.BuildTower(st, 1, now)
	if !ok {
		return Session{}, false
	}
	f := sess.Activities[0].Fight
	if len(f.Rounds) > 3 {
		f.Rounds = f.Rounds[:3]
	}
	f.HP = len(f.Rounds)

	h := fnv.New64a()
	fmt.Fprintf(h, "%s:%d:%d", now.Format("2006-01-02"), region, n)
	rng := rand.New(rand.NewPCG(h.Sum64(), 0xd7e5))
	var pool []Pokemon
	for _, p := range c.Roster() {
		if !p.Legendary {
			pool = append(pool, p)
		}
	}
	p := pool[rng.IntN(len(pool))]
	f.Pokemon, f.Name, f.Syllables = p.ID, p.Name, p.Syllables
	f.Legendary = false
	f.Taunt = TrainerTaunt
	f.TauntAudio = speech.URL(TrainerTaunt, speech.StyleNormal)

	sess.EpisodeID = fmt.Sprintf("%s%d-%d", TrainerPrefix, region, n)
	sess.Title = "Combat de Dresseur"
	sess.Legendary = false
	sess.Tower = nil
	sess.Trainer = &TrainerInfo{Region: region, N: n, Key: fmt.Sprintf("%d:%d", region, n)}
	sess.Activities[0].Title = "Combat de Dresseur"
	return sess, true
}
