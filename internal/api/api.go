// Package api exposes the HTTP surface consumed by the tablet.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
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
	cur     *curriculum.Curriculum
	players *progress.Registry
	store   *progress.Store // the save a request is served with (see play / adminPlay)
	web     http.Handler
	webDir  string
	base    string
	auth    *Auth // nil: the parent side is closed (see auth.go)
}

// New returns a Server serving the frontend from webDir, mounted at base
// ("/" standalone, or e.g. "/pokelecture/" behind a reverse proxy).
// New serves a single player's save (tests, tools); NewPlayers serves many.
func New(cur *curriculum.Curriculum, store *progress.Store, webDir, base string) *Server {
	return NewPlayers(cur, progress.Single(store), webDir, base)
}

// NewPlayers serves every player of a registry, one save each.
func NewPlayers(cur *curriculum.Curriculum, players *progress.Registry, webDir, base string) *Server {
	return &Server{
		cur:     cur,
		players: players,
		web:     http.FileServer(http.Dir(webDir)),
		webDir:  webDir,
		base:    base,
	}
}

// handlePage serves an HTML page (or the manifest) with the __BASE__ placeholder
// replaced by the real mount path. This is the whole mechanism that lets the same
// build run at / locally and under /pokelecture/ in production.
// parentPages maps each parent page's address to its file: the admin, and
// the voice studio under it.
var parentPages = map[string]string{"admin": "admin.html", "admin/studio": "parent.html"}

// oldParentPages are earlier addresses of the parent pages, now redirects.
var oldParentPages = map[string]string{"admin.html": "admin", "parent.html": "admin/studio", "studio": "admin/studio"}

// servePage serves one named page file, whatever the request path.
func (s *Server) servePage(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { s.page(w, r, file) }
}

// redirectTo sends an old address to its new one, inside the mount.
func (s *Server) redirectTo(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.base+route, http.StatusMovedPermanently)
	}
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path
	if name == "/" {
		name = "/index.html"
	}
	s.page(w, r, filepath.Base(name))
}

func (s *Server) page(w http.ResponseWriter, r *http.Request, file string) {
	name := file
	b, err := os.ReadFile(filepath.Join(s.webDir, file))
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
	mux.HandleFunc("GET /api/state", s.play((*Server).handleState))
	mux.HandleFunc("GET /api/session/{id}", s.play((*Server).handleSession))
	mux.HandleFunc("POST /api/session/{id}/result", s.play((*Server).handleResult))

	// Mini-games: learned material to play with, and best scores to beat.
	mux.HandleFunc("GET /api/arcade", s.play((*Server).handleArcade))
	mux.HandleFunc("POST /api/records", s.play((*Server).handleRecord))

	// The partner, the daily gift and the tall grass.
	mux.HandleFunc("POST /api/partner", s.play((*Server).handlePartner))
	mux.HandleFunc("POST /api/gift", s.play((*Server).handleGift))
	mux.HandleFunc("GET /api/wild", s.play((*Server).handleWild))
	mux.HandleFunc("POST /api/wild/catch", s.play((*Server).handleWildCatch))
	mux.HandleFunc("POST /api/find", s.play((*Server).handleFind))
	mux.HandleFunc("GET /api/team", s.play((*Server).handleTeam))
	mux.HandleFunc("POST /api/team", s.play((*Server).handleTeamChoose))
	mux.HandleFunc("POST /api/feed", s.play((*Server).handleFeed))

	// Players: who plays on this device ("Qui joue ?").
	mux.HandleFunc("GET /api/players", s.handlePlayers)
	mux.HandleFunc("POST /api/players/select", s.handlePlayerSelect)
	mux.HandleFunc("POST /api/players", s.handlePlayerCreate)
	mux.HandleFunc("POST /api/name", s.play((*Server).handleName))
	mux.HandleFunc("GET /api/admin/players", s.guard(s.handleAdminPlayers))
	mux.HandleFunc("POST /api/admin/players", s.guard(s.handleAdminPlayerCreate))
	mux.HandleFunc("POST /api/admin/players/{id}/delete", s.guard(s.handleAdminPlayerDelete))

	// Parent-side voice recording — behind the Google sign-in when it is on.
	mux.HandleFunc("GET /api/texts", s.guard(s.handleTexts))
	mux.HandleFunc("PUT /api/record/{key}", s.guard(s.handleRecordPut))
	mux.HandleFunc("DELETE /api/record/{key}", s.guard(s.handleRecordDelete))

	// Parent-side administration: inspect and repair the savegame.
	mux.HandleFunc("GET /api/admin/summary", s.guard(s.adminPlay((*Server).handleAdminSummary)))
	mux.HandleFunc("POST /api/admin/name", s.guard(s.adminPlay((*Server).handleAdminName)))
	mux.HandleFunc("POST /api/admin/reset", s.guard(s.adminPlay((*Server).handleAdminReset)))
	mux.HandleFunc("POST /api/admin/episodes/{id}/reset", s.guard(s.adminPlay((*Server).handleAdminEpisodeReset)))
	mux.HandleFunc("POST /api/admin/episodes/{id}/complete", s.guard(s.adminPlay((*Server).handleAdminEpisodeComplete)))
	mux.HandleFunc("POST /api/admin/pokedex/{id}", s.guard(s.adminPlay((*Server).handleAdminPokedex)))
	mux.HandleFunc("POST /api/admin/settings", s.guard(s.adminPlay((*Server).handleAdminSettings)))
	mux.HandleFunc("POST /api/admin/episodes/{id}/stars", s.guard(s.adminPlay((*Server).handleAdminEpisodeStars)))
	mux.HandleFunc("POST /api/admin/records/reset", s.guard(s.adminPlay((*Server).handleAdminRecordsReset)))

	// Google sign-in for the parent pages.
	mux.HandleFunc("GET /auth/login", s.handleLogin)
	mux.HandleFunc("GET /auth/callback", s.handleCallback)
	mux.HandleFunc("GET /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/me", s.handleMe)

	// Must sit in front of the file server so a recording can shadow the
	// generated file behind the same URL.
	mux.HandleFunc("GET /audio/{file}", s.handleAudio)

	// These must sit in front of the file server so their <base href> gets rewritten.
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /index.html", s.handlePage)
	// The parent pages live at clean addresses, registered for every method
	// so a POST or PUT cannot slip past the guard to the file server. The old
	// .html addresses only redirect (bookmarks keep working).
	for route, file := range parentPages {
		mux.HandleFunc("/"+route, s.guard(getOnly(s.servePage(file))))
	}
	for old, route := range oldParentPages {
		mux.HandleFunc("/"+old, getOnly(s.redirectTo(route)))
	}
	// Recordings are served only through handleAudio, with an audio type.
	mux.HandleFunc("/audio/recorded/", http.NotFound)
	mux.HandleFunc("GET /manifest.webmanifest", s.handlePage)

	mux.Handle("/", noStore(static(s.web)))
	return logging(mux)
}

type episodeView struct {
	ID        string   `json:"id"`
	Route     int      `json:"route"`
	Title     string   `json:"title"`
	Region    string   `json:"region"`
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
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Syllables []string   `json:"syllables"`
	SylAudio  []string   `json:"syllable_audio"`
	Types     []string   `json:"types"`
	Dex       string     `json:"dex"`
	DexAudio  string     `json:"dex_audio"`
	NameAudio string     `json:"name_audio"`
	Sprite    string     `json:"sprite"`
	Art       string     `json:"art"`
	Caught    bool       `json:"caught"`
	Shiny     bool       `json:"shiny,omitempty"`
	Where     *voiceLine `json:"where,omitempty"` // where an uncaught one waits
}

type stateView struct {
	Name       string            `json:"name"`
	Stars      int               `json:"stars"`
	Streak     int               `json:"streak"`
	NextID     string            `json:"next_episode"`
	AllDone    bool              `json:"all_done"`
	Tower      bool              `json:"tower"` // Tour de Combat unlocked (≥1 episode done)
	TowerFloor int               `json:"tower_floor"`
	Records    map[string]int    `json:"records"`
	Episodes   []episodeView     `json:"episodes"`
	Pokedex    []pokemonView     `json:"pokedex"`
	Badges     []badgeView       `json:"badges"`
	UI         map[string]string `json:"ui"`

	Partner  *partnerView  `json:"partner"`
	Starters []starterView `json:"starters,omitempty"` // only while none is chosen
	Prompt   voiceLine     `json:"starter_prompt"`
	Balls    int           `json:"balls"`
	Gift     bool          `json:"gift"` // today's gift is waiting
	Wild     wildCount     `json:"wild"`

	Trainer trainerView    `json:"trainer"`
	Players int            `json:"players"` // how many players share the game
	Named   bool           `json:"named"`   // false: still the default "Dresseur", ask the name
	TeamN   int            `json:"team_size"`
	Bag     map[string]int `json:"bag"`
	Found   []string       `json:"found"`  // hidden objects picked today
	Beaten  []string       `json:"beaten"` // trainers beaten today
	Hungry  bool           `json:"hungry"` // the partner wants its berry
	Quests  []questView    `json:"quests"`
}

type wildCount struct {
	Caught    int `json:"caught"`
	Shiny     int `json:"shiny"`
	Available int `json:"available"`
	Total     int `json:"total"`
}

func (s *Server) state() stateView {
	st := s.store.Snapshot()

	out := stateView{
		Name: st.Name, Stars: st.Stars, Streak: st.Streak,
		TowerFloor: st.TowerFloor, Records: st.Records, UI: curriculum.UIAudio(),
		Partner: s.partnerView(st), Balls: st.Balls,
		Prompt: s.line(curriculum.StarterPrompt, curriculum.StarterPrompt),
		Gift:   st.Partner != nil && st.GiftDay != time.Now().Format("2006-01-02"),
	}
	if st.Partner == nil {
		out.Starters = s.starters()
	}
	out.Quests = s.questViews()
	today := s.store.Snapshot().Today
	out.Bag, out.Found, out.Beaten = st.Bag, today.Found, today.Beaten
	if out.Bag == nil {
		out.Bag = map[string]int{}
	}
	out.Hungry = st.Partner != nil && !today.Fed
	out.Trainer = trainerOf(st)
	out.Players = len(s.players.List())
	out.Named = st.Name != progress.DefaultName
	out.TeamN = len(s.team(st))
	for _, v := range s.wildViews(st) {
		out.Wild.Total++
		if v.Available {
			out.Wild.Available++
		}
		if v.Caught {
			out.Wild.Caught++
		}
		if v.Shiny {
			out.Wild.Shiny++
		}
	}
	unlocked := true
	for _, ep := range s.cur.Episodes {
		stat := st.Episodes[ep.ID]
		v := episodeView{
			ID:        ep.ID,
			Route:     ep.Route,
			Title:     ep.Title,
			Region:    ep.Region,
			Unlocked:  unlocked || st.Settings.FreePlay,
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
		// The parent decides what "done enough to move on" means: completed
		// is the default, a star threshold (global, or set per episode) makes
		// replays part of the course.
		unlocked = unlocked && stat.Completions > 0 && stat.BestStars >= st.Settings.MinStarsFor(ep.ID)
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
			SylAudio:  curriculum.SyllableAudio(p.Syllables),
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
		if !caught[p.ID] {
			w := s.whereLine(p.ID)
			out.Pokedex[len(out.Pokedex)-1].Where = &w
		}
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
	if region, n, ok := curriculum.ParseTrainer(id); ok {
		sess, built := s.cur.BuildTrainer(s.store.Snapshot(), region, n, time.Now())
		if !built {
			http.Error(w, "les Dresseurs attendent ta première route terminée", http.StatusConflict)
			return
		}
		writeJSON(w, http.StatusOK, sess)
		return
	}
	if id == curriculum.TowerID {
		st := s.store.Snapshot()
		// Always the first uncleared floor: no skipping ahead, no need to choose.
		sess, ok := s.cur.BuildTower(st, st.TowerFloor+1, time.Now())
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
	// Tower results: the floor attempted, the rounds played and the points
	// scored. The pass threshold is recomputed server-side from the rounds.
	Floor  int `json:"floor"`
	Rounds int `json:"rounds"`
	Points int `json:"points"`
}

type resultResponse struct {
	// NewCatches lists every Pokémon this completion added to the Pokédex: the
	// reward, the boss and everything read in the episode — reading a name is
	// catching it, which is what makes the Pokédex completable.
	NewCatches []int `json:"new_catches"`
	// Tower verdict: whether the floor was cleared, and the recomputed target.
	TowerPassed bool `json:"tower_passed"`
	TowerNeed   int  `json:"tower_need"`
	NewShiny    int  `json:"new_shiny,omitempty"` // a guardian floor's shiny legendary
	// What the session paid: Poké Balls, and the partner's experience.
	Balls        int           `json:"balls_won"`
	PartnerEvent *partnerEvent `json:"partner_event"`
	TrainerWon   bool          `json:"trainer_won,omitempty"`
	QuestsDone   []questView   `json:"quests_done"`
	TrainerEvent *trainerEvent `json:"trainer_event,omitempty"`
	Milestone    *voiceLine    `json:"milestone,omitempty"`
	State        stateView     `json:"state"`
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// The tower is practice, not progression: answers feed the spaced review
	// boxes and nothing else — no completion, no stars, no catch.
	isTower := id == curriculum.TowerID
	tRegion, tN, isTrainer := curriculum.ParseTrainer(id)
	var ep *curriculum.Episode
	if !isTower && !isTrainer {
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

	right := 0
	for _, it := range req.Items {
		if it.Correct {
			right++
		}
	}

	words := 0
	for _, it := range req.Items {
		if it.Correct && strings.HasPrefix(it.ID, "mot:") {
			words++
		}
	}

	res := resultResponse{NewCatches: []int{}}
	if isTrainer {
		// A won battle comes back completed; a lost one does not.
		if req.Completed && s.store.Beat(fmt.Sprintf("%d:%d", tRegion, tN), now, questsFor) {
			res.TrainerWon = true
			res.Balls = trainerBalls
			res.PartnerEvent = s.award(trainerXP + right)
			res.QuestsDone = append(res.QuestsDone, s.progress("trainer", 1)...)
		}
	} else if isTower {
		// Clearing a floor means at least half the words were built without a
		// single wrong tap; only the very next floor can be cleared.
		res.TowerNeed = curriculum.TowerNeed(req.Rounds)
		st0 := s.store.Snapshot()
		if req.Rounds > 0 && req.Points >= res.TowerNeed && req.Floor == s.store.Snapshot().TowerFloor+1 {
			res.TowerPassed = true
			s.store.ClearFloor(req.Floor)
			if id := s.cur.TowerRewardFor(req.Floor); id != 0 && s.store.Catch(id) {
				res.NewCatches = append(res.NewCatches, id)
			}
			// A beaten guardian he already has joins in its shiny form.
			if guard, ok := s.cur.TowerOpponent(req.Floor, st0); ok && req.Floor%curriculum.GuardianEvery == 0 {
				if contains(st0.Pokedex, guard.ID) && !contains(st0.Shiny, guard.ID) {
					s.store.MarkShiny(guard.ID)
					res.NewShiny = guard.ID
				}
			}
			res.Balls = towerBalls
		}
		res.PartnerEvent = s.award(min(maxSessionXP, right+req.Points))
	} else if req.Completed {
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
		res.Balls = episodeBalls + stars
		res.PartnerEvent = s.award(min(maxSessionXP, right+sessionBonusXP+starXP*stars))
	}
	if res.Balls > 0 {
		s.store.AddBalls(res.Balls)
	}
	res.QuestsDone = append(res.QuestsDone, s.progress("words", words)...)
	if req.Completed && !isTower && !isTrainer {
		res.QuestsDone = append(res.QuestsDone, s.progress("route", 1)...)
	}
	tev, m := s.settle()
	res.TrainerEvent, res.Milestone = tev, milestoneLine(s, m)
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

func getOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "méthode non permise", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

// static is the file server behind the app's own routes: GET and HEAD only,
// never the parent pages under another spelling (a case-insensitive disk
// would serve /ADMIN.HTML), and never a type guessed from the content.
func static(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "méthode non permise", http.StatusMethodNotAllowed)
			return
		}
		switch strings.ToLower(path.Base(r.URL.Path)) {
		case "admin.html", "parent.html":
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(strings.ToLower(r.URL.Path), "/audio/recorded/") {
			http.NotFound(w, r)
			return
		}
		// No directory listings (the page itself is served by GET /{$}).
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never let a browser guess a type from the bytes (an HTML page
		// uploaded as a "recording" would run as one).
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// No page of this app is meant to be framed by another site.
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		start := time.Now()
		next.ServeHTTP(w, r)
		if hasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
