package api

// The mini-games corner ("l'Arcade"): timed games built client-side from the
// material the child has actually learned. The server hands out the pool and
// keeps the best scores; the comprehension games also report their answers so
// the spaced repetition learns from play.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

type arcadeWord struct {
	Text      string   `json:"text"`
	Audio     string   `json:"audio"`
	Emoji     string   `json:"emoji,omitempty"`
	Sprite    string   `json:"sprite,omitempty"`
	Icon      string   `json:"icon,omitempty"`
	Syllables []string `json:"syllables"`
}

type arcadeSyllable struct {
	Text  string `json:"text"`
	Audio string `json:"audio"`
}

// arcadePokemon is one face of the mini-games' roster: the course's Pokémon,
// caught or not, plus the arcade-only extras.
type arcadePokemon struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	NameAudio string `json:"name_audio"`
	Sprite    string `json:"sprite"`
}

type arcadeView struct {
	Words     []arcadeWord     `json:"words"`
	Syllables []arcadeSyllable `json:"syllables"`
	Pokemon   []arcadePokemon  `json:"pokemon"`
}

// handleArcade returns everything learned so far, for the games to draw from.
func (s *Server) handleArcade(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	words, syllables, ok := s.cur.LearnedPool(st)
	if !ok {
		http.Error(w, "les mini-jeux ouvrent après le premier épisode terminé", http.StatusConflict)
		return
	}

	out := arcadeView{Words: []arcadeWord{}, Syllables: []arcadeSyllable{}, Pokemon: []arcadePokemon{}}
	for _, p := range s.cur.Roster() {
		out.Pokemon = append(out.Pokemon, arcadePokemon{
			ID:        p.ID,
			Name:      p.Name,
			NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
			Sprite:    fmt.Sprintf("sprites/art/%d.png", p.ID),
		})
	}
	for _, wd := range words {
		pic := wd.Pic()
		out.Words = append(out.Words, arcadeWord{
			Text:      wd.Text,
			Audio:     speech.URL(wd.Spoken(), speech.StyleWord),
			Emoji:     pic.Emoji,
			Sprite:    pic.Sprite,
			Icon:      pic.Icon,
			Syllables: wd.Syllables,
		})
	}
	for _, syl := range syllables {
		out.Syllables = append(out.Syllables, arcadeSyllable{
			Text:  syl,
			Audio: speech.URL(curriculum.SpokenSyllable(syl), speech.StyleSyllable),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type recordRequest struct {
	Game  string `json:"game"`
	Score int    `json:"score"`
	// Lower flips the comparison for time-based games (memory: fastest wins).
	Lower bool `json:"lower"`
	// Items lets the comprehension games feed the spaced repetition.
	Items []struct {
		ID      string `json:"id"`
		Correct bool   `json:"correct"`
	} `json:"items"`
}

var arcadeGames = map[string]bool{
	"memory": true, "chasse": true, "lecture": true, "oreille": true,
	"atelier": true, "mystere": true, "paires": true,
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request) {
	var req recordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !arcadeGames[req.Game] {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	now := time.Now()
	for _, it := range req.Items {
		if it.ID != "" {
			s.store.Answer(it.ID, it.Correct, now)
		}
	}
	beaten := s.store.Record(req.Game, req.Score, req.Lower)
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"beaten": beaten,
		"state":  s.state(),
	})
}
