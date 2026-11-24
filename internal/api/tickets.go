package api

import (
	"encoding/json"
	"net/http"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
)

// Game tickets: Memory and La chasse cost one; reading earns them (a route,
// a Tower floor, an exam passed, a book read). README « Les tickets ».

// ticketReward: tickets a reading achievement pays.
const ticketReward = 1

// handleTicket spends one ticket on a ticket game: 409 when there is none.
func (s *Server) handleTicket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Game string `json:"game"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !curriculum.TicketGames[req.Game] {
		http.Error(w, "jeu sans ticket", http.StatusBadRequest)
		return
	}
	if !s.store.AddItem("ticket", -1) {
		http.Error(w, "plus de ticket", http.StatusConflict)
		return
	}
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": s.state()})
}

// payTicket gives the ticket a reading achievement earns.
func (s *Server) payTicket() int {
	s.store.AddItem("ticket", ticketReward)
	return ticketReward
}
