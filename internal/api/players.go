package api

// Players: one save each. A game request is served with the save of the
// player this device picked (cookie pk_player); an admin request with the
// player chosen on the admin page (?player=). A lone player needs no
// choosing. The voice track and the studio recordings are shared.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/adelvecchio/pokelecture/internal/curriculum"
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

// deviceStore is the save this device may play: the player its signed
// cookie names — the signature binds the player's current code, so a forged
// cookie or a changed code asks again — or a lone player without a code.
func (s *Server) deviceStore(r *http.Request) (*progress.Store, bool) {
	if id, ok := s.devicePlayer(r); ok {
		st, _ := s.players.Get(id)
		return st, true
	}
	if _, st, ok := s.players.Only(); ok && st.LockTag() == "" {
		return st, true
	}
	return nil, false
}

func (s *Server) devicePlayer(r *http.Request) (string, bool) {
	c, err := r.Cookie(playerCookie)
	if err != nil {
		return "", false
	}
	id, mac, _ := strings.Cut(c.Value, ".")
	st, ok := s.players.Get(id)
	if !ok {
		return "", false
	}
	if mac == "" { // a cookie from before codes existed: fine while unlocked
		return id, st.LockTag() == ""
	}
	return id, hmac.Equal([]byte(mac), []byte(s.deviceMAC(id, st.LockTag())))
}

func (s *Server) deviceMAC(id, lockTag string) string {
	m := hmac.New(sha256.New, s.players.Key())
	m.Write([]byte("device|" + id + "|" + lockTag))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// A code is three Pokémon taken from this palette (the game draws the same).
var lockPalette = map[int]bool{25: true, 4: true, 7: true, 1: true, 133: true, 39: true, 52: true, 143: true, 54: true}

const lockLength = 4

func validCode(code []int) bool {
	if len(code) != lockLength {
		return false
	}
	for _, c := range code {
		if !lockPalette[c] {
			return false
		}
	}
	return true
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
	ID     string `json:"id"`
	Name   string `json:"name"`
	Locked bool   `json:"locked"` // a picture code guards this save
	Avatar int    `json:"avatar"` // the partner's species: what the child recognises
	// The rest is left out for a private player on the game's list: anyone
	// can open "Qui joue ?", and a name and a Pokémon are all it needs.
	Stars   int       `json:"stars,omitzero"`
	Caught  int       `json:"caught,omitzero"`
	Level   int       `json:"trainer_level,omitzero"`
	Updated time.Time `json:"updated_at,omitzero"`
}

// playerViews lists the players; full adds a private player's progress
// (the admin's list, behind the sign-in).
func (s *Server) playerViews(full bool) []playerView {
	var out []playerView
	for _, p := range s.players.List() {
		st, ok := s.players.Get(p.ID)
		if !ok {
			continue
		}
		snap := st.Snapshot()
		v := playerView{ID: p.ID, Name: snap.Name, Locked: snap.Lock != nil, Avatar: 25}
		if full || snap.Lock == nil {
			v.Stars, v.Caught, v.Level, v.Updated = snap.Stars, len(snap.Pokedex), trainerOf(snap).Level, snap.UpdatedAt
		}
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
	if id, ok := s.devicePlayer(r); ok {
		current = id
	} else if id, st, ok := s.players.Only(); ok && st.LockTag() == "" {
		current = id
	}
	// ui: "Qui joue ?" and the code screen speak before any save is open.
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews(false), "current": current, "ui": curriculum.UIAudio()})
}

// handlePlayerSelect makes this device a player.
func (s *Server) handlePlayerSelect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Code []int  `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	st, ok := s.players.Get(req.ID)
	if !ok {
		http.Error(w, "joueur inconnu", http.StatusNotFound)
		return
	}
	switch res, wait := st.TryLock(req.Code); res {
	case progress.LockWait:
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"locked": true, "wait": int(wait.Seconds()) + 1})
		return
	case progress.LockClosed:
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"locked": true, "closed": true})
		return
	case progress.LockWrong:
		writeJSON(w, http.StatusForbidden, map[string]any{"locked": true})
		return
	}
	http.SetCookie(w, s.playerCookie(r, req.ID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) playerCookie(r *http.Request, id string) *http.Cookie {
	tag := ""
	if st, ok := s.players.Get(id); ok {
		tag = st.LockTag()
	}
	return &http.Cookie{
		Name: playerCookie, Value: id + "." + s.deviceMAC(id, tag), Path: s.base, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: s.scheme(r) == "https", MaxAge: int(playerTTL.Seconds()),
	}
}

/* ---------------------------------------------------------------- admin --- */

func (s *Server) handleAdminPlayers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews(true)})
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
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "players": s.playerViews(true)})
}

// handleAdminLock sets (three palette Pokémon) or removes (empty) the
// picture code of the player named by ?player=. Every device must then
// enter the new code.
func (s *Server) handleAdminLock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code   []int `json:"code"`
		Reopen bool  `json:"reopen"` // keep the code, forget the wrong tries
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (len(req.Code) > 0 && !validCode(req.Code)) {
		http.Error(w, "un code, c'est quatre Pokémon de la liste", http.StatusBadRequest)
		return
	}
	if req.Reopen {
		s.store.ReopenLock()
	} else {
		s.store.SetLock(req.Code)
	}
	s.adminDone(w)
}

func (s *Server) handleAdminPlayerDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.players.Delete(r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"players": s.playerViews(true)})
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
