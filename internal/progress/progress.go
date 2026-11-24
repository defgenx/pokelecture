// Package progress stores what the child has learned. One JSON file, atomically
// rewritten: a single reader on the home network does not need a database, and a
// file you can open and read yourself is worth a lot when you are debugging why
// your son is being served the same syllable four times.
package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Box intervals for the spaced review. Box 0 is "seen and wrong", box 4 is
// "solid, come back in a week".
var boxDelay = []time.Duration{
	0,
	10 * time.Minute,
	24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
}

// Item is the mastery record for one reviewable thing: a grapheme sound, a
// syllable or a word. IDs look like "son:ou", "syl:ma", "mot:tomate".
type Item struct {
	Box      int       `json:"box"`
	Seen     int       `json:"seen"`
	Correct  int       `json:"correct"`
	LastSeen time.Time `json:"last_seen"`
}

// Due reports whether the item should come back in this session.
func (it Item) Due(now time.Time) bool {
	box := min(it.Box, len(boxDelay)-1)
	return now.Sub(it.LastSeen) >= boxDelay[box]
}

// EpisodeStat tracks how an episode went.
type EpisodeStat struct {
	Completions int       `json:"completions"`
	BestStars   int       `json:"best_stars"`
	LastPlayed  time.Time `json:"last_played"`
}

// Settings is the parent-tunable progression policy, edited from the admin
// page. Zero values are the defaults: completing an episode unlocks the next,
// nothing is force-unlocked.
type Settings struct {
	// MinStars an episode must have earned before the next one unlocks (0–3).
	MinStars int `json:"min_stars"`
	// FreePlay unlocks every episode regardless of completion.
	FreePlay bool `json:"free_play"`
	// EpisodeStars overrides MinStars for specific episodes: the stars THAT
	// episode must have earned before the following one unlocks.
	EpisodeStars map[string]int `json:"episode_stars,omitempty"`
	// BattleLives is how many wrong taps a battle forgives before it is lost
	// and the episode restarts: -1 = unlimited, otherwise 2–5. The zero value
	// means "never configured" and is migrated to the default at Open().
	BattleLives int `json:"battle_lives,omitempty"`
}

// DefaultBattleLives is the battle heart count on a fresh (or pre-lives) save.
const DefaultBattleLives = 3

// MinStarsFor is the threshold in force for one episode: its own override if
// set, the global policy otherwise.
func (s Settings) MinStarsFor(id string) int {
	if v, ok := s.EpisodeStars[id]; ok {
		return v
	}
	return s.MinStars
}

// State is the whole savegame.
type State struct {
	Name    string `json:"name"`
	Stars   int    `json:"stars"`
	Pokedex []int  `json:"pokedex"`
	// Shiny lists Pokémon caught in their rare recolored form on a replay —
	// a second, slower collection layered over the Pokédex.
	Shiny    []int                  `json:"shiny,omitempty"`
	Episodes map[string]EpisodeStat `json:"episodes"`
	Items    map[string]Item        `json:"items"`
	// TowerFloor is the highest Tour de Combat floor cleared.
	TowerFloor int `json:"tower_floor,omitempty"`
	// Records holds the best score per mini-game.
	Records map[string]int `json:"records,omitempty"`

	// Partner is the starter chosen on first launch: one species line, levelled
	// by experience earned reading. Nil until the child has picked one.
	Partner *Partner `json:"partner,omitempty"`
	// Balls are the Poké Balls earned in the course and spent in the tall grass.
	Balls int `json:"balls"`
	// Wild lists the Pokémon caught in the tall grass — the arcade roster,
	// a collection beside the course Pokédex. WildShiny the rare recolored ones.
	Wild      []int `json:"wild,omitempty"`
	WildShiny []int `json:"wild_shiny,omitempty"`
	// GiftDay is the last day the daily gift was opened (YYYY-MM-DD).
	GiftDay string `json:"gift_day,omitempty"`
	// BallsGranted marks the one-time starter bag of Poké Balls as handed out.
	BallsGranted bool `json:"balls_granted,omitempty"`

	Settings  Settings  `json:"settings"`
	Streak    int       `json:"streak"`
	LastDay   string    `json:"last_day"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Partner is the child's own Pokémon. Its level and form derive from XP.
type Partner struct {
	Line int `json:"line"` // the starter's dex number (first stage of its line)
	XP   int `json:"xp"`
}

// StarterBalls is the bag handed out with the first partner.
const StarterBalls = 5

// Store is a mutex-guarded State persisted to a single file.
type Store struct {
	mu    sync.Mutex
	path  string
	state State
}

// DefaultName is the fallback when nothing is configured at startup and the
// savegame does not carry a name yet.
const DefaultName = "Dresseur"

// Open loads the savegame, creating an empty one if the file does not exist.
//
// name is what was configured at startup (-name / POKELECTURE_NAME); the empty
// string means "not configured". A configured name always wins over the one
// already in the savegame, so renaming the child is a restart away. It used to
// apply only when the stored name was empty, which meant the very first run
// pinned the name forever and the flag looked like it did nothing.
func Open(path string, name string) (*Store, error) {
	s := &Store{path: path, state: State{
		Name:     name,
		Episodes: map[string]EpisodeStat{},
		Items:    map[string]Item{},
		Settings: Settings{BattleLives: DefaultBattleLives},
	}}

	b, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if s.state.Name == "" {
			s.state.Name = DefaultName
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		return s, s.flush()
	case err != nil:
		return nil, err
	}
	if err := json.Unmarshal(b, &s.state); err != nil {
		return nil, err
	}
	if s.state.Episodes == nil {
		s.state.Episodes = map[string]EpisodeStat{}
	}
	if s.state.Items == nil {
		s.state.Items = map[string]Item{}
	}

	// A save from before the lives system (or a fresh one) gets the default.
	if s.state.Settings.BattleLives == 0 {
		s.state.Settings.BattleLives = DefaultBattleLives
	}

	// Persist right away rather than waiting for the next Save(): the name is
	// the one field a parent changes on purpose, and seeing it stick in
	// progress.json is how you confirm the restart took.
	switch {
	case name != "" && name != s.state.Name:
		s.state.Name = name
		if err := s.flush(); err != nil {
			return nil, err
		}
	case s.state.Name == "":
		s.state.Name = DefaultName
		if err := s.flush(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Name returns the child's name currently in effect.
func (s *Store) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Name
}

// Snapshot returns a copy safe to serialise.
func (s *Store) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.copyLocked()
}

func (s *Store) copyLocked() State {
	out := s.state
	out.Pokedex = append([]int(nil), s.state.Pokedex...)
	out.Shiny = append([]int(nil), s.state.Shiny...)
	out.Wild = append([]int(nil), s.state.Wild...)
	out.WildShiny = append([]int(nil), s.state.WildShiny...)
	if s.state.Partner != nil {
		pt := *s.state.Partner
		out.Partner = &pt
	}
	out.Records = make(map[string]int, len(s.state.Records))
	for k, v := range s.state.Records {
		out.Records[k] = v
	}
	out.Settings.EpisodeStars = make(map[string]int, len(s.state.Settings.EpisodeStars))
	for k, v := range s.state.Settings.EpisodeStars {
		out.Settings.EpisodeStars[k] = v
	}
	out.Episodes = make(map[string]EpisodeStat, len(s.state.Episodes))
	for k, v := range s.state.Episodes {
		out.Episodes[k] = v
	}
	out.Items = make(map[string]Item, len(s.state.Items))
	for k, v := range s.state.Items {
		out.Items[k] = v
	}
	return out
}

// Answer records one attempt and moves the item up or down a box.
func (s *Store) Answer(id string, correct bool, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	it := s.state.Items[id]
	it.Seen++
	it.LastSeen = now
	if correct {
		it.Correct++
		if it.Box < len(boxDelay)-1 {
			it.Box++
		}
	} else if it.Box > 0 {
		// One mistake drops a single box, not all the way to zero: an
		// accidental mis-tap on a tablet should not erase real progress.
		it.Box--
	}
	s.state.Items[id] = it
}

// CompleteEpisode records a finished session and awards stars.
func (s *Store) CompleteEpisode(id string, stars int, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state.Episodes[id]
	st.Completions++
	st.LastPlayed = now
	if stars > st.BestStars {
		s.state.Stars += stars - st.BestStars
		st.BestStars = stars
	}
	s.state.Episodes[id] = st

	day := now.Format("2006-01-02")
	if s.state.LastDay != day {
		if s.state.LastDay == now.AddDate(0, 0, -1).Format("2006-01-02") {
			s.state.Streak++
		} else {
			s.state.Streak = 1
		}
		s.state.LastDay = day
	}
}

// Catch adds a Pokémon to the Pokédex. Returns false if already caught.
func (s *Store) Catch(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, got := range s.state.Pokedex {
		if got == id {
			return false
		}
	}
	s.state.Pokedex = append(s.state.Pokedex, id)
	sort.Ints(s.state.Pokedex)
	return true
}

// Uncatch removes a Pokémon from the Pokédex (admin repair tool).
func (s *Store) Uncatch(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.state.Pokedex[:0]
	for _, got := range s.state.Pokedex {
		if got != id {
			out = append(out, got)
		}
	}
	s.state.Pokedex = out

	sh := s.state.Shiny[:0]
	for _, got := range s.state.Shiny {
		if got != id {
			sh = append(sh, got)
		}
	}
	s.state.Shiny = sh
}

// MarkShiny records that a Pokémon was caught in its shiny form.
func (s *Store) MarkShiny(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, got := range s.state.Shiny {
		if got == id {
			return
		}
	}
	s.state.Shiny = append(s.state.Shiny, id)
	sort.Ints(s.state.Shiny)
}

// ClearFloor records a Tour de Combat floor as cleared. Floors only ever go up.
func (s *Store) ClearFloor(floor int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if floor > s.state.TowerFloor {
		s.state.TowerFloor = floor
	}
}

// Record stores a mini-game score and reports whether it beats the old best.
// lowerBetter flips the comparison for time-based games.
func (s *Store) Record(game string, score int, lowerBetter bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state.Records == nil {
		s.state.Records = map[string]int{}
	}
	old, has := s.state.Records[game]
	beaten := !has || (lowerBetter && score < old) || (!lowerBetter && score > old)
	if beaten {
		s.state.Records[game] = score
	}
	return beaten
}

// ChoosePartner sets the first partner. Only once: a second call is a no-op
// and returns false. The head start (XP) lets a child who played long before
// partners existed see a first evolution soon rather than restart from zero.
func (s *Store) ChoosePartner(line, headStart int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Partner != nil {
		return false
	}
	s.state.Partner = &Partner{Line: line, XP: max(0, headStart)}
	if !s.state.BallsGranted {
		s.state.Balls += StarterBalls
		s.state.BallsGranted = true
	}
	return true
}

// GainXP adds experience to the partner and returns the XP before and after
// (equal when there is no partner yet).
func (s *Store) GainXP(xp int) (before, after int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Partner == nil || xp <= 0 {
		if s.state.Partner != nil {
			return s.state.Partner.XP, s.state.Partner.XP
		}
		return 0, 0
	}
	before = s.state.Partner.XP
	s.state.Partner.XP += xp
	return before, s.state.Partner.XP
}

// AddBalls gives (or, negative, takes) Poké Balls; never below zero.
func (s *Store) AddBalls(n int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Balls = max(0, s.state.Balls+n)
	return s.state.Balls
}

// CatchWild spends one Poké Ball on a tall-grass Pokémon. It reports whether
// a ball was available and whether this Pokémon is new to the collection.
func (s *Store) CatchWild(id int, shiny bool) (spent, isNew bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Balls <= 0 {
		return false, false
	}
	s.state.Balls--
	isNew = !contains(s.state.Wild, id)
	if isNew {
		s.state.Wild = append(s.state.Wild, id)
		sort.Ints(s.state.Wild)
	}
	if shiny && !contains(s.state.WildShiny, id) {
		s.state.WildShiny = append(s.state.WildShiny, id)
		sort.Ints(s.state.WildShiny)
		isNew = true
	}
	return true, isNew
}

// OpenGift claims today's gift; false if it was already opened today.
func (s *Store) OpenGift(now time.Time, balls int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	day := now.Format("2006-01-02")
	if s.state.GiftDay == day {
		return false
	}
	s.state.GiftDay = day
	s.state.Balls += balls
	return true
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// UpdateSettings replaces the global progression policy. The per-episode
// overrides are managed by SetEpisodeStars and survive this call.
func (s *Store) UpdateSettings(minStars int, freePlay bool, battleLives int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings.MinStars = max(0, min(3, minStars))
	s.state.Settings.FreePlay = freePlay
	if battleLives == -1 || (battleLives >= 2 && battleLives <= 5) {
		s.state.Settings.BattleLives = battleLives
	}
}

// ResetRecords wipes one mini-game's best score, or all of them ("").
func (s *Store) ResetRecords(game string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if game == "" {
		s.state.Records = map[string]int{}
		return
	}
	delete(s.state.Records, game)
}

// SetEpisodeStars sets (0–3) or clears (negative) one episode's own star
// threshold, overriding the global policy for that episode only.
func (s *Store) SetEpisodeStars(id string, stars int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stars < 0 {
		delete(s.state.Settings.EpisodeStars, id)
		return
	}
	if s.state.Settings.EpisodeStars == nil {
		s.state.Settings.EpisodeStars = map[string]int{}
	}
	s.state.Settings.EpisodeStars[id] = min(3, stars)
}

// Rename changes the child's name without touching anything else.
func (s *Store) Rename(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name != "" {
		s.state.Name = name
	}
}

// Reset wipes the whole savegame, keeping only the name and the parental
// settings — those describe the parent's choices, not the child's progress.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	name, settings := s.state.Name, s.state.Settings
	s.state = State{
		Name:     name,
		Settings: settings,
		Episodes: map[string]EpisodeStat{},
		Items:    map[string]Item{},
	}
}

// ResetEpisode forgets one episode's stats, so it plays as new again. Its
// banked stars leave the total with it: the counter's invariant is
// Stars = sum of BestStars, and keeping them would double-count on the replay
// (CompleteEpisode awards the delta against a BestStars that is now gone).
// The spaced-review items are shared across episodes and stay.
func (s *Store) ResetEpisode(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.state.Episodes[id]; ok {
		s.state.Stars = max(0, s.state.Stars-st.BestStars)
	}
	delete(s.state.Episodes, id)
}

// Save persists the current state atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

func (s *Store) flush() error {
	s.state.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
