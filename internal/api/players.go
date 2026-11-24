package api

// Players: one save each. A game request is served with the save of the
// player this device picked (cookie pk_player); an admin request with the
// player chosen on the admin page (?player=). A lone player needs no
// choosing. The voice track and the studio recordings are shared.

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

const (
	playerCookie = "pk_player"
	playerTTL    = 400 * 24 * time.Hour // the device keeps its player
)

// play runs a game handler on the save of this device's player. With
// several players and none picked yet, the answer is 409 + choose_player:
// the game then shows "Qui joue ?".
func (s *Server) play(h func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		st, ok := s.deviceStore(r)
		if !ok {
			writeJSON(w, http.StatusConflict, map[string]any{"choose_player": true})
			return
		}
		c := *s
		c.store = st
		h(&c, w, r)
	}
}

func (s *Server) deviceStore(r *http.Request) (*progress.Store, bool) {
	if c, err := r.Cookie(playerCookie); err == nil {
		if st, ok := s.players.Get(c.Value); ok {
			return st, true
		}
	}
	_, st, ok := s.players.Only()
	return st, ok
}

// adminPlay runs an admin handler on the player named by ?player=, or the
// only one, or the first.
func (s *Server) adminPlay(h func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("player")
		if id == "" {
			if list := s.players.List(); len(list) > 0 {
				id = list[0].ID
			}
		}
		st, ok := s.players.Get(id)
		if !ok {
			http.Error(w, "joueur inconnu", http.StatusNotFound)
			return
		}
		c := *s
		c.store = st
		h(&c, w, r)
	}
}

type playerView struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Avatar  int       `json:"avatar"` // the partner's species: what the child recognises
	Stars   int       `json:"stars"`
	Caught  int       `json:"caught"`
	Level   int       `json:"trainer_level"`
	Updated time.Time `json:"updated_at"`
}

func (s *Server) playerViews() []playerView {
	var out []playerView
	for _, p := range s.players.List() {
		st, ok := s.players.Get(p.ID)
		if !ok {
			continue
		}
		snap := st.Snapshot()
		v := playerView{ID: p.ID, Name: snap.Name, Avatar: 25, Stars: snap.Stars, Caught: len(snap.Pokedex),
			Level: trainerOf(snap).Level, Updated: snap.UpdatedAt}
		if pv := s.partnerView(snap); pv != nil {
			v.Avatar = pv.Species
		}
		out = append(out, v)
	}
	return out
}

// handlePlayers lists the players for the "Qui joue ?" screen, and which
// one this device is.
func (s *Server) handlePlayers(w http.ResponseWriter, r *http.Request) {
	current := ""
	if c, err := r.Cookie(playerCookie); err == nil {
		if _, ok := s.players.Get(c.Value); ok {
			current = c.Value
		}
	}
	if current == "" {
		if id, _, ok := s.players.Only(); ok {
			current = id
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews(), "current": current})
}

// handlePlayerSelect makes this device a player.
func (s *Server) handlePlayerSelect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	if _, ok := s.players.Get(req.ID); !ok {
		http.Error(w, "joueur inconnu", http.StatusNotFound)
		return
	}
	http.SetCookie(w, s.playerCookie(r, req.ID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) playerCookie(r *http.Request, id string) *http.Cookie {
	return &http.Cookie{
		Name: playerCookie, Value: id, Path: s.base, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: s.scheme(r) == "https", MaxAge: int(playerTTL.Seconds()),
	}
}

/* ---------------------------------------------------------------- admin --- */

func (s *Server) handleAdminPlayers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews()})
}

func (s *Server) handleAdminPlayerCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	id, err := s.players.Create(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "players": s.playerViews()})
}

func (s *Server) handleAdminPlayerDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.players.Delete(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews()})
}

/* ------------------------------------------------------- from the game --- */

// maxPlayers caps the players the game itself may create: it has no
// sign-in, so this is what keeps an open page from filling the disk.
const maxPlayers = 12

// handlePlayerCreate: "➕ Nouveau joueur" on the "Qui joue ?" screen. The
// new player becomes this device's player.
func (s *Server) handlePlayerCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "prénom manquant", http.StatusBadRequest)
		return
	}
	if len(s.players.List()) >= maxPlayers {
		http.Error(w, "trop de joueurs : demande à un parent d'en retirer dans l'administration", http.StatusConflict)
		return
	}
	id, err := s.players.Create(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.SetCookie(w, s.playerCookie(r, id))
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// handleName names this device's player — only one that has no name yet
// (the "Dresseur" a fresh install starts with). Renaming a named child stays
// in the admin.
func (s *Server) handleName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	name := ""
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		name = strings.TrimSpace(req.Name)
	}
	if name == "" || len([]rune(name)) > 30 {
		http.Error(w, "prénom manquant ou trop long", http.StatusBadRequest)
		return
	}
	if s.store.Snapshot().Name != progress.DefaultName {
		http.Error(w, "ce joueur a déjà un prénom", http.StatusConflict)
		return
	}
	s.store.Rename(name)
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"state": s.state()})
}
