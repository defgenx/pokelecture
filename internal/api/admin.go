package api

// The parent backoffice: inspect the savegame, rename the child, replay or
// force-complete an episode, fix the Pokédex, or wipe everything.
//
// No authentication, like the voice studio — fine on the home network, put an
// auth_basic in front on a public URL (see deploy/delvecch.io.md).

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type adminEpisodeView struct {
	ID          string    `json:"id"`
	Route       int       `json:"route"`
	Title       string    `json:"title"`
	Legendary   bool      `json:"legendary,omitempty"`
	Badge       string    `json:"badge,omitempty"`
	Unlocked    bool      `json:"unlocked"`
	Completions int       `json:"completions"`
	BestStars   int       `json:"best_stars"`
	LastPlayed  time.Time `json:"last_played"`
	Catches     []int     `json:"catches"`
}

type adminItemView struct {
	ID       string    `json:"id"`
	Box      int       `json:"box"`
	Seen     int       `json:"seen"`
	Correct  int       `json:"correct"`
	LastSeen time.Time `json:"last_seen"`
	Due      bool      `json:"due"`
}

type adminPokemonView struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Caught bool   `json:"caught"`
	Shiny  bool   `json:"shiny"`
}

type adminSummary struct {
	Name      string             `json:"name"`
	Stars     int                `json:"stars"`
	Streak    int                `json:"streak"`
	LastDay   string             `json:"last_day"`
	UpdatedAt time.Time          `json:"updated_at"`
	Episodes  []adminEpisodeView `json:"episodes"`
	Items     []adminItemView    `json:"items"`
	Pokedex   []adminPokemonView `json:"pokedex"`
}

func (s *Server) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	now := time.Now()

	out := adminSummary{
		Name:      st.Name,
		Stars:     st.Stars,
		Streak:    st.Streak,
		LastDay:   st.LastDay,
		UpdatedAt: st.UpdatedAt,
	}

	unlocked := true
	for _, ep := range s.cur.Episodes {
		stat := st.Episodes[ep.ID]
		v := adminEpisodeView{
			ID:          ep.ID,
			Route:       ep.Route,
			Title:       ep.Title,
			Legendary:   ep.Legendary,
			Unlocked:    unlocked,
			Completions: stat.Completions,
			BestStars:   stat.BestStars,
			LastPlayed:  stat.LastPlayed,
			Catches:     ep.CatchablePokemon(),
		}
		if ep.Badge != nil {
			v.Badge = ep.Badge.Name
		}
		out.Episodes = append(out.Episodes, v)
		unlocked = unlocked && stat.Completions > 0
	}

	for id, it := range st.Items {
		out.Items = append(out.Items, adminItemView{
			ID:       id,
			Box:      it.Box,
			Seen:     it.Seen,
			Correct:  it.Correct,
			LastSeen: it.LastSeen,
			Due:      it.Due(now),
		})
	}
	// Weakest first, so the top of the list is what to practise tonight.
	sort.Slice(out.Items, func(i, j int) bool {
		if out.Items[i].Box != out.Items[j].Box {
			return out.Items[i].Box < out.Items[j].Box
		}
		return out.Items[i].ID < out.Items[j].ID
	})

	caught := map[int]bool{}
	for _, id := range st.Pokedex {
		caught[id] = true
	}
	shiny := map[int]bool{}
	for _, id := range st.Shiny {
		shiny[id] = true
	}
	for _, id := range s.cur.PokemonOrder {
		out.Pokedex = append(out.Pokedex, adminPokemonView{
			ID:     id,
			Name:   s.cur.Pokemon[id].Name,
			Caught: caught[id],
			Shiny:  shiny[id],
		})
	}
	sort.Slice(out.Pokedex, func(i, j int) bool { return out.Pokedex[i].ID < out.Pokedex[j].ID })

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "prénom manquant", http.StatusBadRequest)
		return
	}
	s.store.Rename(strings.TrimSpace(req.Name))
	s.adminDone(w)
}

func (s *Server) handleAdminReset(w http.ResponseWriter, r *http.Request) {
	s.store.Reset()
	s.adminDone(w)
}

func (s *Server) handleAdminEpisodeReset(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.cur.Episode(r.PathValue("id")); !ok {
		http.Error(w, "épisode inconnu", http.StatusNotFound)
		return
	}
	s.store.ResetEpisode(r.PathValue("id"))
	s.adminDone(w)
}

// handleAdminEpisodeComplete marks an episode as done without playing it — the
// skip-ahead tool. It awards no stars but does fill the Pokédex, exactly like a
// real completion, so the following routes stay coherent.
func (s *Server) handleAdminEpisodeComplete(w http.ResponseWriter, r *http.Request) {
	ep, ok := s.cur.Episode(r.PathValue("id"))
	if !ok {
		http.Error(w, "épisode inconnu", http.StatusNotFound)
		return
	}
	s.store.CompleteEpisode(ep.ID, 0, time.Now())
	for _, id := range ep.CatchablePokemon() {
		s.store.Catch(id)
	}
	s.adminDone(w)
}

func (s *Server) handleAdminPokedex(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "identifiant invalide", http.StatusBadRequest)
		return
	}
	if _, ok := s.cur.Pokemon[id]; !ok {
		http.Error(w, "pokémon inconnu", http.StatusNotFound)
		return
	}
	var req struct {
		Caught bool `json:"caught"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	if req.Caught {
		s.store.Catch(id)
	} else {
		s.store.Uncatch(id)
	}
	s.adminDone(w)
}

// adminDone persists and replies with the fresh summary, so the page never has
// to guess at the post-action state.
func (s *Server) adminDone(w http.ResponseWriter) {
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
