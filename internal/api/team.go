package api

// The team (any caught Pokémon can be the partner, each keeps its level),
// the trainer's own level, and the Pokédex milestones.

import (
	"encoding/json"

	"net/http"
	"sort"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

const (
	catchTrainerXP   = 10
	missionTrainerXP = 15
	levelBalls       = 2 // every trainer level
	fifthLevelBonus  = 5 // every fifth: a Super Ball and a berry too
)

type trainerView struct {
	Level int    `json:"level"`
	XP    int    `json:"xp"`
	Need  int    `json:"need"`
	Title string `json:"title"`
}

func trainerOf(st progress.State) trainerView {
	l, into, need := curriculum.TrainerLevel(st.TrainerXP)
	return trainerView{Level: l, XP: into, Need: need, Title: curriculum.TrainerTitle(l)}
}

// trainerEvent is a trainer level-up: the reward is already in the bag.
type trainerEvent struct {
	Level   int        `json:"level"`
	Title   string     `json:"title"`
	NewRank *voiceLine `json:"new_rank,omitempty"` // a new title was earned
	Line    voiceLine  `json:"line"`
	Balls   int        `json:"balls"`
	Bonus   bool       `json:"bonus"` // + a Super Ball and a berry
}

// settle pays whatever the last actions earned: trainer levels and the
// Pokédex milestone. Every handler that hands out XP or catches calls it.
func (s *Server) settle() (*trainerEvent, int) {
	st := s.store.Snapshot()
	level, _, _ := curriculum.TrainerLevel(st.TrainerXP)
	var ev *trainerEvent
	for _, l := range s.store.PayTrainerLevel(level) {
		s.store.AddBalls(levelBalls)
		bonus := l%fifthLevelBonus == 0
		if bonus {
			s.store.AddItem("superball", 1)
			s.store.AddItem("berry", 1)
		}
		prevTitle := curriculum.TrainerTitle(l - 1)
		ev = &trainerEvent{Level: l, Title: curriculum.TrainerTitle(l), Balls: levelBalls, Bonus: bonus,
			Line: s.line(curriculum.TrainerLevelUp, curriculum.TrainerLevelUp)}
		if t := curriculum.TrainerTitle(l); t != prevTitle {
			nr := s.line(curriculum.TrainerTitlePhrase(t), curriculum.TrainerTitlePhrase(t))
			ev.NewRank = &nr
		}
	}
	caught := len(s.store.Snapshot().Pokedex)
	m := caught / 10 * 10
	milestone := 0
	if m >= 10 && s.store.ReachMilestone(m) {
		s.store.AddItem("superball", 1)
		s.store.AddBalls(2)
		milestone = m
	}
	return ev, milestone
}

type memberView struct {
	Species   int    `json:"species"`
	Base      int    `json:"base"`
	Name      string `json:"name"`
	NameAudio string `json:"name_audio"`
	Level     int    `json:"level"`
	Active    bool   `json:"active"`
	NextName  string `json:"next_name,omitempty"`
	NextLevel int    `json:"next_level,omitempty"`
}

// team lists every Pokémon he owns, one per evolution line: its current
// form and level if it has been his partner, else its highest caught stage.
func (s *Server) team(st progress.State) []memberView {
	owned := map[int]bool{}
	for _, id := range st.Pokedex {
		owned[id] = true
	}
	for _, id := range st.Wild {
		owned[id] = true
	}
	activeBase, _ := activeXP(st)
	byBase := map[int]int{} // base → highest owned species
	for id := range owned {
		b := s.cur.Base(id)
		if cur, ok := byBase[b]; !ok || s.cur.StageLevel(id) > s.cur.StageLevel(cur) {
			byBase[b] = id
		}
	}
	if st.Partner != nil {
		byBase[st.Partner.Line] = st.Partner.Line
	}
	var out []memberView
	for b, top := range byBase {
		var xp int
		switch {
		case st.Partner != nil && b == st.Partner.Line:
			xp = st.Partner.XP
		case st.Team[b] > 0:
			xp = st.Team[b]
		default:
			xp = curriculum.XPForLevel(s.cur.StageLevel(top))
		}
		v := s.memberView(b, xp)
		out = append(out, memberView{
			Species: v.Species, Base: b, Name: v.Name, NameAudio: v.NameAudio, Level: v.Level,
			Active: b == activeBase, NextName: v.NextName, NextLevel: v.NextLevel,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		if out[i].Level != out[j].Level {
			return out[i].Level > out[j].Level
		}
		return out[i].Species < out[j].Species
	})
	return out
}

func (s *Server) handleTeam(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"members": s.team(s.store.Snapshot()),
		"prompt":  s.line(curriculum.TeamPrompt, curriculum.TeamPrompt),
		"chosen":  s.line(curriculum.TeamChosen, curriculum.TeamChosen),
	})
}

func (s *Server) handleTeamChoose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Species int `json:"species"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	st := s.store.Snapshot()
	base := s.cur.Base(req.Species)
	var found *memberView
	for _, m := range s.team(st) {
		if m.Base == base {
			m := m
			found = &m
		}
	}
	if found == nil {
		http.Error(w, "ce Pokémon n'est pas encore à toi", http.StatusBadRequest)
		return
	}
	s.store.SetActive(base, curriculum.XPForLevel(found.Level))
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"state": s.state()})
}

func (s *Server) whereLine(id int) voiceLine {
	t := s.cur.Where(id)
	return voiceLine{Text: t, Audio: speech.URL(t, speech.StyleNormal)}
}

func milestoneLine(s *Server, m int) *voiceLine {
	if m == 0 {
		return nil
	}
	l := s.line(curriculum.MilestonePhrase(m), curriculum.MilestonePhrase(m))
	return &l
}
