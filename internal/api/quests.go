package api

// Daily missions, hidden objects on the map, trainers and feeding the
// partner: the reasons to open the game again tomorrow. Everything day-scoped
// lives in progress.State.Today and rolls over on the first event of a day.

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// A mission kind: what counts, how many, what it pays. Each text is a voice
// line (curriculum.QuestPhrases) — the child cannot read it.
type questDef struct {
	ID     string
	Text   string
	Goal   int
	Reward string // "balls", "berry", "superball"
}

var questDefs = []questDef{
	{"catch", "Attrape deux Pokémon dans les hautes herbes.", 2, "superball"},
	{"words", "Lis dix mots sans te tromper.", 10, "berry"},
	{"route", "Termine une route.", 1, "balls"},
	{"trainer", "Bats un Dresseur sur la carte.", 1, "berry"},
	{"find", "Trouve un objet caché sur la carte.", 1, "balls"},
	{"feed", "Donne une baie à ton partenaire.", 1, "superball"},
	{"arcade", "Joue à un mini-jeu.", 1, "berry"},
}

// questsFor picks the day's three missions, the same for the whole day.
func questsFor(day string) []string {
	h := fnv.New64a()
	h.Write([]byte(day))
	rng := rand.New(rand.NewPCG(h.Sum64(), 0x9e57))
	ids := make([]string, len(questDefs))
	for i, q := range questDefs {
		ids[i] = q.ID
	}
	rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	return ids[:3]
}

func questDefOf(id string) (questDef, bool) {
	for _, q := range questDefs {
		if q.ID == id {
			return q, true
		}
	}
	return questDef{}, false
}

type questView struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Audio    string `json:"audio"`
	Goal     int    `json:"goal"`
	Progress int    `json:"progress"`
	Done     bool   `json:"done"`
	Reward   string `json:"reward"`
}

func (s *Server) questViews() []questView {
	s.store.Roll(time.Now(), questsFor)
	st := s.store.Snapshot()
	claimed := map[string]bool{}
	for _, c := range st.Today.Claimed {
		claimed[c] = true
	}
	var out []questView
	for _, id := range st.Today.Quests {
		q, ok := questDefOf(id)
		if !ok {
			continue
		}
		out = append(out, questView{
			ID: q.ID, Text: q.Text, Audio: speech.URL(q.Text, speech.StyleNormal),
			Goal: q.Goal, Progress: min(q.Goal, st.Today.Progress[q.ID]), Done: claimed[q.ID], Reward: q.Reward,
		})
	}
	return out
}

// progress counts an event towards today's missions and pays out every
// mission it completes; the result is what to celebrate on screen.
func (s *Server) progress(kind string, n int) []questView {
	if n <= 0 || s.store.Snapshot().Partner == nil {
		return nil
	}
	s.store.Count(kind, n, time.Now(), questsFor)
	var done []questView
	for _, q := range s.questViews() {
		if q.ID != kind || q.Done || q.Progress < q.Goal {
			continue
		}
		if !s.store.Claim(q.ID) {
			continue
		}
		s.give(q.Reward)
		s.store.TrainerGain(missionTrainerXP)
		q.Done = true
		done = append(done, q)
	}
	return done
}

// give pays one reward: two Poké Balls, a berry or a Super Ball.
func (s *Server) give(reward string) {
	switch reward {
	case "balls":
		s.store.AddBalls(2)
	default:
		s.store.AddItem(reward, 1)
	}
}

/* ------------------------------------------------------- hidden objects --- */

// objectsPerRegion is how many sparkles each region's map shows a day.
const objectsPerRegion = 2

type findReward struct {
	Kind  string    `json:"kind"` // balls | superball | candy | berry
	Count int       `json:"count"`
	Line  voiceLine `json:"line"`
}

func (s *Server) handleFind(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Region int    `json:"region"`
		Spot   string `json:"spot"` // "x,y" on the landscape map
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Spot == "" {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	now := time.Now()
	s.store.Roll(now, questsFor)
	st := s.store.Snapshot()
	prefix := fmt.Sprintf("%d:", req.Region)
	n := 0
	for _, f := range st.Today.Found {
		if strings.HasPrefix(f, prefix) {
			n++
		}
	}
	if n >= objectsPerRegion || !s.store.Pick(prefix+req.Spot, now, questsFor) {
		http.Error(w, "déjà ramassé", http.StatusConflict)
		return
	}
	var rw findReward
	var ev *partnerEvent
	switch roll := rand.Float64(); {
	case roll < 0.40:
		rw = findReward{Kind: "balls", Count: 1 + rand.IntN(2)}
		s.store.AddBalls(rw.Count)
		rw.Line = s.line(curriculum.FoundBalls, curriculum.FoundBalls)
	case roll < 0.65:
		rw = findReward{Kind: "superball", Count: 1}
		s.store.AddItem("superball", 1)
		rw.Line = s.line(curriculum.FoundSuperBall, curriculum.FoundSuperBall)
	case roll < 0.85:
		rw = findReward{Kind: "candy", Count: 1}
		ev = s.award(candyXP)
		rw.Line = s.line(curriculum.FoundCandy, curriculum.FoundCandy)
	default:
		rw = findReward{Kind: "berry", Count: 1}
		s.store.AddItem("berry", 1)
		rw.Line = s.line(curriculum.FoundBerry, curriculum.FoundBerry)
	}
	done := s.progress("find", 1)
	tev, m := s.settle()
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"reward": rw, "partner_event": ev, "quests_done": done,
		"trainer_event": tev, "milestone": milestoneLine(s, m), "state": s.state()})
}

const (
	candyXP   = 30
	berryXP   = 15
	trainerXP = 20
)

/* ---------------------------------------------------------------- feed --- */

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	if !s.store.Feed(time.Now(), questsFor) {
		http.Error(w, "pas de baie, ou déjà nourri aujourd'hui", http.StatusConflict)
		return
	}
	ev := s.award(berryXP)
	done := s.progress("feed", 1)
	tev, m := s.settle()
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"partner_event": ev, "quests_done": done,
		"trainer_event": tev, "milestone": milestoneLine(s, m), "state": s.state()})
}
