// Package api exposes the HTTP surface consumed by the tablet.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// Server wires the curriculum, the savegame and the static frontend together.
type Server struct {
	cur    *curriculum.Curriculum
	store  *progress.Store
	web    http.Handler
	webDir string
	base   string
}

// New returns a Server serving the frontend from webDir, mounted at base
// ("/" standalone, or e.g. "/pokelecture/" behind a reverse proxy).
func New(cur *curriculum.Curriculum, store *progress.Store, webDir, base string) *Server {
	return &Server{
		cur:    cur,
		store:  store,
		web:    http.FileServer(http.Dir(webDir)),
		webDir: webDir,
		base:   base,
	}
}

// handlePage serves an HTML page (or the manifest) with the __BASE__ placeholder
// replaced by the real mount path. This is the whole mechanism that lets the same
// build run at / locally and under /pokelecture/ in production.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	if name == "/" {
		name = "/index.html"
	}
	b, err := os.ReadFile(filepath.Join(s.webDir, filepath.Base(name)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body := strings.ReplaceAll(string(b), "__BASE__", s.base)

	ct := "text/html; charset=utf-8"
	if strings.HasSuffix(name, ".webmanifest") {
		ct = "application/manifest+json"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, body)
}

// Routes builds the mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/session/{id}", s.handleSession)
	mux.HandleFunc("POST /api/session/{id}/result", s.handleResult)

	// Parent-side voice recording.
	mux.HandleFunc("GET /api/texts", s.handleTexts)
	mux.HandleFunc("PUT /api/record/{key}", s.handleRecordPut)
	mux.HandleFunc("DELETE /api/record/{key}", s.handleRecordDelete)

	// Parent-side administration: inspect and repair the savegame.
	mux.HandleFunc("GET /api/admin/summary", s.handleAdminSummary)
	mux.HandleFunc("POST /api/admin/name", s.handleAdminName)
	mux.HandleFunc("POST /api/admin/reset", s.handleAdminReset)
	mux.HandleFunc("POST /api/admin/episodes/{id}/reset", s.handleAdminEpisodeReset)
	mux.HandleFunc("POST /api/admin/episodes/{id}/complete", s.handleAdminEpisodeComplete)
	mux.HandleFunc("POST /api/admin/pokedex/{id}", s.handleAdminPokedex)

	// Must sit in front of the file server so a recording can shadow the
	// generated file behind the same URL.
	mux.HandleFunc("GET /audio/{file}", s.handleAudio)

	// These must sit in front of the file server so their <base href> gets rewritten.
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /index.html", s.handlePage)
	mux.HandleFunc("GET /parent.html", s.handlePage)
	mux.HandleFunc("GET /admin.html", s.handlePage)
	mux.HandleFunc("GET /manifest.webmanifest", s.handlePage)

	mux.Handle("/", noStore(s.web))
	return logging(mux)
}

type episodeView struct {
	ID        string   `json:"id"`
	Route     int      `json:"route"`
	Title     string   `json:"title"`
	Unlocked  bool     `json:"unlocked"`
	Stars     int      `json:"stars"`
	Done      int      `json:"done"`
	Reward    int      `json:"reward_pokemon"`
	Sounds    []string `json:"sounds"`
	Legendary bool     `json:"legendary,omitempty"`
	Badge     string   `json:"badge,omitempty"`
}

type badgeView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Done bool   `json:"done"`
}

type pokemonView struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Syllables []string `json:"syllables"`
	Types     []string `json:"types"`
	Dex       string   `json:"dex"`
	DexAudio  string   `json:"dex_audio"`
	NameAudio string   `json:"name_audio"`
	Sprite    string   `json:"sprite"`
	Art       string   `json:"art"`
	Caught    bool     `json:"caught"`
	Shiny     bool     `json:"shiny,omitempty"`
}

type stateView struct {
	Name     string            `json:"name"`
	Stars    int               `json:"stars"`
	Streak   int               `json:"streak"`
	NextID   string            `json:"next_episode"`
	AllDone  bool              `json:"all_done"`
	Tower    bool              `json:"tower"` // Tour de Combat unlocked (≥1 episode done)
	Episodes []episodeView     `json:"episodes"`
	Pokedex  []pokemonView     `json:"pokedex"`
	Badges   []badgeView       `json:"badges"`
	UI       map[string]string `json:"ui"`
}

func (s *Server) state() stateView {
	st := s.store.Snapshot()

	out := stateView{Name: st.Name, Stars: st.Stars, Streak: st.Streak, UI: curriculum.UIAudio()}
	unlocked := true
	for _, ep := range s.cur.Episodes {
		stat := st.Episodes[ep.ID]
		v := episodeView{
			ID:        ep.ID,
			Route:     ep.Route,
			Title:     ep.Title,
			Unlocked:  unlocked,
			Stars:     stat.BestStars,
			Done:      stat.Completions,
			Reward:    ep.RewardPokemon,
			Sounds:    ep.NewGraphemes,
			Legendary: ep.Legendary,
		}
		if ep.Badge != nil {
			v.Badge = ep.Badge.ID
			out.Badges = append(out.Badges, badgeView{
				ID:   ep.Badge.ID,
				Name: ep.Badge.Name,
				Done: stat.Completions > 0,
			})
		}
		out.Episodes = append(out.Episodes, v)
		if stat.Completions > 0 {
			out.Tower = true
		}
		if unlocked && out.NextID == "" && stat.Completions == 0 {
			out.NextID = ep.ID
		}
		unlocked = unlocked && stat.Completions > 0
	}
	if out.NextID == "" && len(s.cur.Episodes) > 0 {
		// Everything done: replay the least recently played episode, so the
		// long tail of replays rotates through the whole course instead of
		// hammering the finale.
		out.AllDone = true
		oldest := s.cur.Episodes[0].ID
		oldestAt := st.Episodes[oldest].LastPlayed
		for _, ep := range s.cur.Episodes[1:] {
			if at := st.Episodes[ep.ID].LastPlayed; at.Before(oldestAt) {
				oldest, oldestAt = ep.ID, at
			}
		}
		out.NextID = oldest
	}

	caught := map[int]bool{}
	for _, id := range st.Pokedex {
		caught[id] = true
	}
	shiny := map[int]bool{}
	for _, id := range st.Shiny {
		shiny[id] = true
	}
	ids := make([]int, 0, len(s.cur.Pokemon))
	for id := range s.cur.Pokemon {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		p := s.cur.Pokemon[id]
		out.Pokedex = append(out.Pokedex, pokemonView{
			ID:        p.ID,
			Name:      p.Name,
			Syllables: p.Syllables,
			Types:     p.Types,
			Dex:       p.Dex,
			DexAudio:  speech.URL(p.Dex, speech.StyleNormal),
			// Spoken(), not Name: the audio file was generated from the phonetic
			// respelling ("Pikatchou"), and the plain-name URL does not exist.
			NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
			Sprite:    fmt.Sprintf("sprites/art/%d.png", p.ID),
			Art:       fmt.Sprintf("sprites/art/%d.png", p.ID),
			Caught:    caught[p.ID],
			Shiny:     shiny[p.ID],
		})
	}
	return out
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.state())
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "next" {
		id = s.state().NextID
	}
	if id == curriculum.TowerID {
		sess, ok := s.cur.BuildTower(s.store.Snapshot(), time.Now())
		if !ok {
			http.Error(w, "la Tour ouvre après le premier épisode terminé", http.StatusConflict)
			return
		}
		writeJSON(w, http.StatusOK, sess)
		return
	}
	ep, ok := s.cur.Episode(id)
	if !ok {
		http.Error(w, "épisode inconnu", http.StatusNotFound)
		return
	}
	sess := s.cur.BuildSession(ep, s.store.Snapshot(), time.Now())
	writeJSON(w, http.StatusOK, sess)
}

type resultRequest struct {
	Items []struct {
		ID      string `json:"id"`
		Correct bool   `json:"correct"`
	} `json:"items"`
	Stars     int  `json:"stars"`
	Completed bool `json:"completed"`
	// Shiny reports that the session's reward was rolled shiny (the roll lives
	// in the session payload; a family app can trust its own client).
	Shiny bool `json:"shiny"`
}

type resultResponse struct {
	// NewCatches lists every Pokémon this completion added to the Pokédex: the
	// reward, the boss and everything read in the episode — reading a name is
	// catching it, which is what makes the Pokédex completable.
	NewCatches []int     `json:"new_catches"`
	State      stateView `json:"state"`
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// The tower is practice, not progression: answers feed the spaced review
	// boxes and nothing else — no completion, no stars, no catch.
	isTower := id == curriculum.TowerID
	var ep *curriculum.Episode
	if !isTower {
		var ok bool
		ep, ok = s.cur.Episode(id)
		if !ok {
			http.Error(w, "épisode inconnu", http.StatusNotFound)
			return
		}
	}

	var req resultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}

	now := time.Now()
	for _, it := range req.Items {
		if it.ID != "" {
			s.store.Answer(it.ID, it.Correct, now)
		}
	}

	res := resultResponse{NewCatches: []int{}}
	if !isTower && req.Completed {
		stars := max(0, min(3, req.Stars))
		s.store.CompleteEpisode(ep.ID, stars, now)
		for _, id := range ep.CatchablePokemon() {
			if s.store.Catch(id) {
				res.NewCatches = append(res.NewCatches, id)
			}
		}
		if req.Shiny && ep.RewardPokemon != 0 {
			s.store.MarkShiny(ep.RewardPokemon)
		}
	}
	if err := s.store.Save(); err != nil {
		log.Printf("sauvegarde: %v", err)
	}

	res.State = s.state()
	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encodage json: %v", err)
	}
}

// noStore keeps the tablet from caching index.html/app.js while we iterate
// nightly. Audio and sprites are content-addressed or stable, so we let those
// through with a long cache.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case hasPrefix(r.URL.Path, "/audio/"), hasPrefix(r.URL.Path, "/sprites/"):
			w.Header().Set("Cache-Control", "public, max-age=604800")
		default:
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if hasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}
