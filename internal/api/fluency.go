package api

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

// The parent's fluency check (admin, « Lecture » tab): the child reads words
// aloud for one minute, the parent taps each one right or wrong. Words read
// correctly per minute is the measure; every answer also feeds the review
// boxes. README « La lecture chronométrée ».

const fluencyWords = 80

// handleAdminFluencyWords hands out learned words in a random order.
func (s *Server) handleAdminFluencyWords(w http.ResponseWriter, r *http.Request) {
	words, _, ok := s.cur.LearnedPool(s.store.Snapshot())
	if !ok {
		http.Error(w, "aucun épisode terminé", http.StatusConflict)
		return
	}
	out := make([]string, 0, len(words))
	for _, wd := range words {
		out = append(out, wd.Text)
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	if len(out) > fluencyWords {
		out = out[:fluencyWords]
	}
	writeJSON(w, http.StatusOK, map[string][]string{"words": out})
}

func (s *Server) handleAdminFluency(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seconds int      `json:"seconds"`
		Right   []string `json:"right"`
		Wrong   []string `json:"wrong"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Seconds < 10 || req.Seconds > 600 {
		http.Error(w, "lecture invalide", http.StatusBadRequest)
		return
	}
	now := time.Now()
	answer := func(words []string, correct bool) {
		for _, word := range words {
			if strings.TrimSpace(word) != "" {
				s.store.Answer("mot:"+word, correct, now)
			}
		}
	}
	answer(req.Right, true)
	answer(req.Wrong, false)
	run := progress.FluencyRun{At: now, Seconds: req.Seconds, Right: len(req.Right), Wrong: len(req.Wrong),
		WCPM: len(req.Right) * 60 / req.Seconds}
	s.store.AddFluency(run)
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleAdminFont(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Font string `json:"font"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !s.store.SetFont(req.Font) {
		http.Error(w, "police inconnue", http.StatusBadRequest)
		return
	}
	s.adminDone(w)
}
