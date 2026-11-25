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
	cur        *curriculum.Curriculum
	players    *progress.Registry
	store      *progress.Store // the save a request is served with (see play / adminPlay)
	web        http.Handler
	webDir     string
	base       string
	auth       *Auth  // nil: the parent side is closed (see auth.go)
	localAdmin string // -local-admin: this run's key; "" off (localadmin.go)
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
	mux.HandleFunc("POST /api/ticket", s.play((*Server).handleTicket))

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
	mux.HandleFunc("POST /api/admin/lock", s.guard(s.adminPlay((*Server).handleAdminLock)))

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
	mux.HandleFunc("POST /api/admin/events/{id}/today", s.guard(s.adminPlay((*Server).handleAdminEventToday)))
	mux.HandleFunc("GET /api/admin/fluency/words", s.guard(s.adminPlay((*Server).handleAdminFluencyWords)))
	mux.HandleFunc("POST /api/admin/fluency", s.guard(s.adminPlay((*Server).handleAdminFluency)))
	mux.HandleFunc("POST /api/admin/font", s.guard(s.adminPlay((*Server).handleAdminFont)))

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
	// Gate: the entry exam (event id) still to pass before this door opens.
	Gate string `json:"gate,omitempty"`
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
	// Event is today's surprise exam on the map; Games the open mini-games.
	Event *eventView `json:"event,omitempty"`
	Games []string   `json:"games"`
	Rod   bool       `json:"rod"` // the fishing rod is earned (README « La pêche »)
	// Daily: the day's objective when the parent turned the limit on.
	Daily *dailyView `json:"daily,omitempty"`
	Font  string     `json:"font,omitempty"` // reading font (admin setting)
	// Exams lists every surprise exam and where he stands (menu « Examens »).
	Exams []examCard `json:"exams"`
	// TowerNext: the Pokémon the next paying Tower floor awards (map's 🗼).
	TowerNext *towerNext `json:"tower_next,omitempty"`
	// TrainerMons: today's Pokémon of each map trainer, "region:n" → id.
	TrainerMons map[string]trainerMon `json:"trainer_mons"`
}

type towerNext struct {
	Pokemon int `json:"pokemon"`
	Floor   int `json:"floor"`
}

type trainerMon struct {
	Pokemon   int    `json:"pokemon"`
	Name      string `json:"name"`
	NameAudio string `json:"name_audio"`
}

type wildCount struct {
	Caught    int `json:"caught"`
	Shiny     int `json:"shiny"`
	Available int `json:"available"`
	Total     int `json:"total"`
}

func (s *Server) state() stateView {
	// A Pokémon moved from the arcade roster into the course (the Tower
	// ladder) may already be in his wild Pokédex, reached by evolution: it
	// counts as caught in the course Pokédex too.
	for _, id := range s.store.Snapshot().Wild {
		if _, inCourse := s.cur.Pokemon[id]; inCourse {
			s.store.Catch(id)
		}
	}
	s.store.GrantTickets()
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
	out.Event = s.eventToday(st)
	out.Font = st.Settings.Font
	out.Exams = s.examCards(st)
	if id, f := s.cur.TowerNext(st.TowerFloor); id != 0 {
		out.TowerNext = &towerNext{Pokemon: id, Floor: f}
	}
	out.TrainerMons = map[string]trainerMon{}
	for r := 0; r < s.regionCount(); r++ {
		for n := 0; n < 2; n++ {
			p := s.cur.TrainerPokemon(r, n, time.Now())
			out.TrainerMons[fmt.Sprintf("%d:%d", r, n)] = trainerMon{Pokemon: p.ID, Name: p.Name, NameAudio: speech.URL(p.Spoken(), speech.StyleWord)}
		}
	}
	out.Rod = s.cur.HasRod(st)
	if st.Settings.DailyLimit {
		now := time.Now()
		out.Daily = &dailyView{Goal: st.Settings.DailyGoal(), Done: st.LessonsToday(now),
			LessonsOver: st.LessonsOver(now), Over: st.DayOver(now)}
	}
	out.Games = s.cur.GamesOpen(st)
	if out.Games == nil {
		out.Games = []string{}
	}
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
		// An entry exam closes the door; the door stays the next stop, and
		// JOUER on it starts the exam (handleSession).
		if s.cur.Gated(ep, st) && !st.Settings.FreePlay {
			if unlocked && out.NextID == "" {
				out.NextID = ep.ID
			}
			out.Episodes = append(out.Episodes, episodeView{ID: ep.ID, Route: ep.Route, Title: ep.Title, Region: ep.Region,
				Reward: ep.RewardPokemon, Sounds: ep.NewGraphemes, Legendary: ep.Legendary, Gate: ep.RequiresExam})
			unlocked = false
			continue
		}
		// Reached: done, or a later one done. An episode inserted in the course
		// (a new Faille) becomes the next stop without closing what he has
		// already played beyond it.
		reached := s.cur.ReachedEpisode(ep.ID, st)
		v := episodeView{
			ID:        ep.ID,
			Route:     ep.Route,
			Title:     ep.Title,
			Region:    ep.Region,
			Unlocked:  unlocked || reached || st.Settings.FreePlay,
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
		if stat.Completions > 0 || st.Settings.FreePlay {
			out.Tower = true
		}
		if unlocked && out.NextID == "" && stat.Completions == 0 {
			out.NextID = ep.ID
		}
		// The parent decides what "done enough to move on" means: completed
		// is the default, a star threshold (global, or set per episode) makes
		// replays part of the course.
		unlocked = (unlocked || reached) && stat.Completions > 0 && stat.BestStars >= st.Settings.MinStarsFor(ep.ID)
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
	if s.dayOver(w) {
		return
	}
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
		writeJSON(w, http.StatusOK, s.speedy(sess))
		return
	}
	if evID, ok := isExam(id); ok {
		if s.lessonsOver(w) {
			return
		}
		ev, known := s.cur.Event(evID)
		if !known {
			http.Error(w, "examen inconnu", http.StatusNotFound)
			return
		}
		sess, built := s.cur.BuildExam(ev, s.store.Snapshot(), time.Now())
		if !built {
			http.Error(w, "l'examen attend ta première route terminée", http.StatusConflict)
			return
		}
		writeJSON(w, http.StatusOK, s.speedy(sess))
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
		writeJSON(w, http.StatusOK, s.speedy(sess))
		return
	}
	ep, ok := s.cur.Episode(id)
	if !ok {
		http.Error(w, "épisode inconnu", http.StatusNotFound)
		return
	}
	if s.lessonsOver(w) {
		return
	}
	// A closed door plays its entry exam.
	if st := s.store.Snapshot(); s.cur.Gated(*ep, st) && !st.Settings.FreePlay {
		ev, _ := s.cur.Event(ep.RequiresExam)
		if sess, built := s.cur.BuildExam(ev, st, time.Now()); built {
			writeJSON(w, http.StatusOK, s.speedy(sess))
			return
		}
	}
	sess := s.cur.BuildSession(ep, s.store.Snapshot(), time.Now())
	writeJSON(w, http.StatusOK, s.speedy(sess))
}

type resultRequest struct {
	Items []struct {
		ID      string `json:"id"`
		Correct bool   `json:"correct"`
		MS      int    `json:"ms,omitempty"` // answer time, drill items only
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
	// Fight facts for the day's missions: the Téracristal was used, every
	// word was built without a wrong tap.
	Tera     bool `json:"tera"`
	Flawless bool `json:"flawless"`
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
	// What the session paid: Poké Balls, game tickets, the partner's experience.
	Balls        int           `json:"balls_won"`
	Tickets      int           `json:"tickets_won,omitempty"`
	PartnerEvent *partnerEvent `json:"partner_event"`
	TrainerWon   bool          `json:"trainer_won,omitempty"`
	QuestsDone   []questView   `json:"quests_done"`
	TrainerEvent *trainerEvent `json:"trainer_event,omitempty"`
	Milestone    *voiceLine    `json:"milestone,omitempty"`
	Exam         *examResult   `json:"exam,omitempty"`
	// NewGames lists the mini-games this result opened.
	NewGames []string `json:"new_games"`
	// RodWon: this route earned the fishing rod.
	RodWon bool      `json:"rod_won,omitempty"`
	State  stateView `json:"state"`
}

func (s *Server) handleResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// The tower is practice, not progression: answers feed the spaced review
	// boxes and nothing else — no completion, no stars, no catch.
	isTower := id == curriculum.TowerID
	tRegion, tN, isTrainer := curriculum.ParseTrainer(id)
	examID, isExamID := isExam(id)
	var ep *curriculum.Episode
	if !isTower && !isTrainer && !isExamID {
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
	times := map[string][]int{}
	for _, it := range req.Items {
		if it.ID != "" {
			s.store.Answer(it.ID, it.Correct, now)
		}
		if kind, _, ok := strings.Cut(it.ID, ":"); ok && it.MS > 0 && it.Correct {
			times[kind] = append(times[kind], it.MS)
		}
	}
	s.store.AddTimes(times)

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

	res := resultResponse{NewCatches: []int{}, NewGames: []string{}}
	gamesBefore := s.cur.GamesOpen(s.store.Snapshot())
	rodBefore := s.cur.HasRod(s.store.Snapshot())
	if isExamID {
		if !s.gradeExam(examID, req, &res, now) {
			http.Error(w, "examen inconnu", http.StatusNotFound)
			return
		}
		res.PartnerEvent = s.award(min(maxSessionXP, right+sessionBonusXP))
	} else if isTrainer {
		// A won battle comes back completed; a lost one does not.
		if req.Completed && s.store.Beat(fmt.Sprintf("%d:%d", tRegion, tN), now, s.questsFor) {
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
			res.QuestsDone = append(res.QuestsDone, s.progress("tower", 1)...)
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
			res.Tickets = s.payTicket()
		}
		res.PartnerEvent = s.award(min(maxSessionXP, right+req.Points))
	} else if req.Completed {
		stars := max(0, min(progress.MaxStars, req.Stars))
		// Balls and XP keep their 3-star amounts: 5★ pays what 3★ paid.
		paid := (stars*3 + 2) / progress.MaxStars
		s.store.CompleteEpisode(ep.ID, stars, now)
		for _, id := range ep.CatchablePokemon() {
			if s.store.Catch(id) {
				res.NewCatches = append(res.NewCatches, id)
			}
		}
		if req.Shiny && ep.RewardPokemon != 0 {
			s.store.MarkShiny(ep.RewardPokemon)
		}
		res.Balls = episodeBalls + paid
		res.Tickets = s.payTicket()
		res.PartnerEvent = s.award(min(maxSessionXP, right+sessionBonusXP+starXP*paid))
	}
	if res.Balls > 0 {
		s.store.AddBalls(res.Balls)
	}
	res.QuestsDone = append(res.QuestsDone, s.progress("words", words)...)
	if req.Completed && (ep != nil || isExamID) {
		s.store.AddLesson(now, s.questsFor)
		res.QuestsDone = append(res.QuestsDone, s.progress(lessonQuest, 1)...)
	}
	if req.Tera {
		res.QuestsDone = append(res.QuestsDone, s.progress("tera", 1)...)
	}
	if req.Flawless && (req.Completed || res.TowerPassed) {
		res.QuestsDone = append(res.QuestsDone, s.progress("flawless", 1)...)
	}
	if req.Completed && ep != nil && req.Stars >= progress.MaxStars {
		res.QuestsDone = append(res.QuestsDone, s.progress("stars5", 1)...)
	}
	if req.Completed && ep != nil {
		res.QuestsDone = append(res.QuestsDone, s.progress("route", 1)...)
	}
	tev, m := s.settle()
	res.TrainerEvent, res.Milestone = tev, milestoneLine(s, m)
	if err := s.store.Save(); err != nil {
		log.Printf("sauvegarde: %v", err)
	}

	res.State = s.state()
	res.RodWon = !rodBefore && res.State.Rod
	open := map[string]bool{}
	for _, g := range gamesBefore {
		open[g] = true
	}
	for _, g := range res.State.Games {
		if !open[g] {
			res.NewGames = append(res.NewGames, g)
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// regionCount counts the map's regions as the client does (web/overworld.js
// regions()): runs of consecutive episodes sharing a region.
func (s *Server) regionCount() int {
	n, last := 0, ""
	for i, ep := range s.cur.Episodes {
		if i == 0 || ep.Region != last {
			n++
		}
		last = ep.Region
	}
	return n
}

// speedMinAnswers: below this many timed answers, his usual speed is unknown
// and the client keeps its fixed limits.
const speedMinAnswers = 10

// speedy adds his usual answer times to a session.
func (s *Server) speedy(sess curriculum.Session) curriculum.Session {
	for kind, v := range s.store.Snapshot().Speed {
		if v.N >= speedMinAnswers {
			if sess.Speed == nil {
				sess.Speed = map[string]int{}
			}
			sess.Speed[kind] = v.MS
		}
	}
	return sess
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

// dailyView is the day under the daily limit: Done of Goal lessons;
// LessonsOver once they are done (no more routes or exams), Over once every
// mission is done too (nothing more at all).
type dailyView struct {
	Goal        int  `json:"goal"`
	Done        int  `json:"done"`
	LessonsOver bool `json:"lessons_over"`
	Over        bool `json:"over"`
}

// dayOver refuses with 403 once every mission of the day is done under the
// daily limit (README « La limite du jour »); the client says the same.
func (s *Server) dayOver(w http.ResponseWriter) bool {
	if !s.store.Snapshot().DayOver(time.Now()) {
		return false
	}
	http.Error(w, "les missions du jour sont faites : à demain", http.StatusForbidden)
	return true
}

// lessonsOver refuses a route or an exam once today's lessons are done.
func (s *Server) lessonsOver(w http.ResponseWriter) bool {
	if !s.store.Snapshot().LessonsOver(time.Now()) {
		return false
	}
	http.Error(w, "les leçons du jour sont faites : termine tes missions", http.StatusForbidden)
	return true
}
