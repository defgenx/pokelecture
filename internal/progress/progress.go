// Package progress stores what the child has learned. One JSON file, atomically
// rewritten: a single reader on the home network does not need a database, and a
// file you can open and read yourself is worth a lot when you are debugging why
// your son is being served the same syllable four times.
package progress

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	// Font is the reading font: "" (the game's), "andika" (print for early
	// readers) or "cursive" (French school cursive).
	Font string `json:"font,omitempty"`
}

// Fonts the reading text can use (web/styles.css, .font-*).
var Fonts = map[string]bool{"": true, "andika": true, "cursive": true}

// SpeedStat is his usual answer time for one kind of question: a moving
// average, so "fast" means faster than he usually is.
type SpeedStat struct {
	MS int `json:"ms"`
	N  int `json:"n"`
}

// FluencyRun is one timed reading a parent scored in the admin.
type FluencyRun struct {
	At      time.Time `json:"at"`
	Seconds int       `json:"seconds"`
	Right   int       `json:"right"`
	Wrong   int       `json:"wrong"`
	WCPM    int       `json:"wcpm"` // words read correctly per minute
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
	// Exams holds the surprise exam results, by event id.
	Exams map[string]ExamRecord `json:"exams,omitempty"`
	// Speed: his usual answer time per item kind ("syl", "mot", "son").
	Speed map[string]SpeedStat `json:"speed,omitempty"`
	// Fluency: the timed readings scored by a parent, oldest first.
	Fluency []FluencyRun `json:"fluency,omitempty"`

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

	// Team holds the XP of every other Pokémon that has been the partner,
	// keyed by the first stage of its line; Active is the one walking with
	// him now (0: the starter, whose XP stays in Partner.XP).
	Team   map[int]int `json:"team,omitempty"`
	Active int         `json:"active,omitempty"`
	// TrainerXP is the child's own experience: everything he does pays it.
	TrainerXP int `json:"trainer_xp,omitempty"`
	// Milestone is the last Pokédex size (a multiple of 10) already rewarded.
	Milestone int `json:"milestone,omitempty"`
	// TrainerPaid is the last trainer level whose reward was handed out.
	TrainerPaid int `json:"trainer_paid,omitempty"`

	// Bag holds the items found on the map: "superball", "berry".
	Bag map[string]int `json:"bag,omitempty"`
	// Today is the day-scoped part of the save: hidden objects picked up,
	// trainers beaten, the partner fed, and the daily missions. It resets
	// itself on the first event of a new day.
	Today Today `json:"today"`

	// Created dates the save (players are listed oldest first).
	Created time.Time `json:"created,omitempty"`
	// Lock is the player's secret picture code (three Pokémon in order),
	// hashed: never sent to any page. Nil: no code, anyone may play this save.
	Lock *Lock `json:"lock,omitempty"`

	Settings  Settings  `json:"settings"`
	Streak    int       `json:"streak"`
	LastDay   string    `json:"last_day"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Today is what the map and the missions remember for one day only.
type Today struct {
	Day      string         `json:"day"`
	Found    []string       `json:"found,omitempty"`    // hidden objects picked: "region:x,y"
	Beaten   []string       `json:"beaten,omitempty"`   // trainers beaten: "region:n"
	Fed      bool           `json:"fed,omitempty"`      // partner fed a berry
	Quests   []string       `json:"quests,omitempty"`   // today's mission ids
	Progress map[string]int `json:"progress,omitempty"` // per mission kind
	Claimed  []string       `json:"claimed,omitempty"`  // missions already rewarded
	Event    string         `json:"event,omitempty"`    // exam a parent called up for today
}

// ExamRecord is the history of one surprise exam: what the parent reads in
// the admin to know which sounds hold and which do not.
type ExamRecord struct {
	Attempts int       `json:"attempts"`
	Passed   bool      `json:"passed,omitempty"`
	Best     int       `json:"best"` // percent
	Last     int       `json:"last"` // percent
	LastAt   time.Time `json:"last_at"`
	// Sections: right and asked per section of the last attempt.
	Sections map[string][2]int `json:"sections,omitempty"`
	// Missed: item ids answered wrong on the last attempt.
	Missed []string `json:"missed,omitempty"`
}

// Lock is a hashed picture code, with its count of wrong tries.
type Lock struct {
	Salt  string    `json:"salt"`
	Hash  string    `json:"hash"`
	Fails int       `json:"fails,omitempty"` // wrong codes since the last right one
	Until time.Time `json:"until,omitzero"`  // no try before this
}

// Wrong codes: every lockStep of them, a wait that grows; at LockClosedAt
// the player stays closed until a parent reopens it (at most 15 tries of
// 6561: an attacker's odds stay under 0.3 %). Devices already in keep playing.
const (
	lockStep     = 5
	LockClosedAt = 15
)

var lockWaits = map[int]time.Duration{5: 5 * time.Minute, 10: time.Hour}

// LockResult is the outcome of a code try.
type LockResult int

const (
	LockOpen   LockResult = iota // the right code, or no code needed
	LockWrong                    // a wrong (or missing) code
	LockWait                     // too many tries: wait
	LockClosed                   // too many tries: a parent must reopen it
)

// now is the clock, swapped by tests.
var now = time.Now

func lockHash(salt string, code []int) string {
	h := sha256.New()
	h.Write([]byte(salt))
	for _, c := range code {
		fmt.Fprintf(h, "|%d", c)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SetLock sets the picture code (fresh count); an empty code removes it.
func (s *Store) SetLock(code []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(code) == 0 {
		s.state.Lock = nil
		return
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	salt := hex.EncodeToString(b)
	s.state.Lock = &Lock{Salt: salt, Hash: lockHash(salt, code)}
}

// ReopenLock clears the wrong tries, keeping the code (and every device in).
func (s *Store) ReopenLock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Lock != nil {
		s.state.Lock.Fails, s.state.Lock.Until = 0, time.Time{}
	}
}

// TryLock checks a code and counts it, in one step: concurrent tries cannot
// slip past the count. A nil code (asking whether one is needed) is not
// counted. The count is saved, so a restart does not reset it.
func (s *Store) TryLock(code []int) (LockResult, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.state.Lock
	if l == nil {
		return LockOpen, 0
	}
	if l.Fails >= LockClosedAt {
		return LockClosed, 0
	}
	if d := l.Until.Sub(now()); d > 0 {
		return LockWait, d
	}
	if code == nil {
		return LockWrong, 0
	}
	if subtle.ConstantTimeCompare([]byte(lockHash(l.Salt, code)), []byte(l.Hash)) == 1 {
		if l.Fails != 0 || !l.Until.IsZero() {
			l.Fails, l.Until = 0, time.Time{}
			_ = s.write()
		}
		return LockOpen, 0
	}
	l.Fails++
	if d, ok := lockWaits[l.Fails]; ok {
		l.Until = now().Add(d)
	}
	_ = s.write()
	if l.Fails >= LockClosedAt {
		return LockClosed, 0
	}
	return LockWrong, 0
}

// LockTag identifies the current code ("" unlocked): a device's right to
// play is bound to it, so changing the code asks every device again.
func (s *Store) LockTag() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Lock == nil {
		return ""
	}
	return s.state.Lock.Hash
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
		s.state.Created = time.Now()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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
	out.Bag = make(map[string]int, len(s.state.Bag))
	for k, v := range s.state.Bag {
		out.Bag[k] = v
	}
	out.Today.Found = append([]string(nil), s.state.Today.Found...)
	out.Today.Beaten = append([]string(nil), s.state.Today.Beaten...)
	out.Today.Quests = append([]string(nil), s.state.Today.Quests...)
	out.Today.Claimed = append([]string(nil), s.state.Today.Claimed...)
	out.Today.Progress = make(map[string]int, len(s.state.Today.Progress))
	for k, v := range s.state.Today.Progress {
		out.Today.Progress[k] = v
	}
	out.WildShiny = append([]int(nil), s.state.WildShiny...)
	out.Fluency = append([]FluencyRun(nil), s.state.Fluency...)
	out.Speed = make(map[string]SpeedStat, len(s.state.Speed))
	for k, v := range s.state.Speed {
		out.Speed[k] = v
	}
	out.Exams = make(map[string]ExamRecord, len(s.state.Exams))
	for k, v := range s.state.Exams {
		v.Missed = append([]string(nil), v.Missed...)
		out.Exams[k] = v
	}
	if s.state.Partner != nil {
		pt := *s.state.Partner
		out.Partner = &pt
	}
	if s.state.Lock != nil {
		l := *s.state.Lock
		out.Lock = &l
	}
	out.Team = make(map[int]int, len(s.state.Team))
	for k, v := range s.state.Team {
		out.Team[k] = v
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

// AddTimes folds answer times (ms, by kind) into his usual speed. Absurd
// times (a tap under 300 ms, a minute away from the tablet) are ignored.
func (s *Store) AddTimes(times map[string][]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Speed == nil {
		s.state.Speed = map[string]SpeedStat{}
	}
	for kind, list := range times {
		st := s.state.Speed[kind]
		for _, ms := range list {
			if ms < 300 || ms > 60000 {
				continue
			}
			if st.N == 0 {
				st.MS = ms
			} else {
				st.MS += (ms - st.MS) / 8
			}
			st.N++
		}
		s.state.Speed[kind] = st
	}
}

// AddFluency stores a timed reading.
func (s *Store) AddFluency(r FluencyRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Fluency = append(s.state.Fluency, r)
}

// SetFont chooses the reading font; false if unknown.
func (s *Store) SetFont(font string) bool {
	if !Fonts[font] {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings.Font = font
	return true
}

// RecordExam stores an exam attempt; true when it is the first pass.
func (s *Store) RecordExam(id string, percent int, passed bool, sections map[string][2]int, missed []string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Exams == nil {
		s.state.Exams = map[string]ExamRecord{}
	}
	r := s.state.Exams[id]
	first := passed && !r.Passed
	r.Attempts++
	r.Passed = r.Passed || passed
	r.Best = max(r.Best, percent)
	r.Last, r.LastAt, r.Sections, r.Missed = percent, now, sections, missed
	s.state.Exams[id] = r
	return first
}

// CallEvent puts an exam on today's map (from the admin); "" clears it.
func (s *Store) CallEvent(id string, now time.Time, quests func(day string) []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
	s.state.Today.Event = id
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

// GainXP adds experience to the active partner — and the same to the
// trainer — and returns the partner's base and XP before and after (equal
// when there is no partner yet).
func (s *Store) GainXP(xp int) (base, before, after int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Partner == nil {
		return 0, 0, 0
	}
	if xp > 0 {
		s.state.TrainerXP += xp
	}
	if s.state.Active == 0 || s.state.Active == s.state.Partner.Line {
		before = s.state.Partner.XP
		s.state.Partner.XP += max(0, xp)
		return s.state.Partner.Line, before, s.state.Partner.XP
	}
	if s.state.Team == nil {
		s.state.Team = map[int]int{}
	}
	before = s.state.Team[s.state.Active]
	s.state.Team[s.state.Active] += max(0, xp)
	return s.state.Active, before, s.state.Team[s.state.Active]
}

// TrainerGain adds experience to the trainer only (catches, missions).
func (s *Store) TrainerGain(xp int) (before, after int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before = s.state.TrainerXP
	s.state.TrainerXP += max(0, xp)
	return before, s.state.TrainerXP
}

// SetActive makes another Pokémon (the base of its line) the partner. A
// first-time member starts with startXP (its stage's level).
func (s *Store) SetActive(base, startXP int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Partner != nil && base == s.state.Partner.Line {
		s.state.Active = 0
		return
	}
	if s.state.Team == nil {
		s.state.Team = map[int]int{}
	}
	if _, ok := s.state.Team[base]; !ok || s.state.Team[base] < startXP {
		s.state.Team[base] = startXP
	}
	s.state.Active = base
}

// PayTrainerLevel records the trainer levels paid up to level; it returns
// the levels newly reached (none if already paid).
func (s *Store) PayTrainerLevel(level int) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.TrainerPaid == 0 {
		s.state.TrainerPaid = 1 // level 1 is where everyone starts: nothing to pay
	}
	var out []int
	for l := s.state.TrainerPaid + 1; l <= level; l++ {
		out = append(out, l)
	}
	if level > s.state.TrainerPaid {
		s.state.TrainerPaid = level
	}
	return out
}

// ReachMilestone records a Pokédex milestone; false if it was already paid.
func (s *Store) ReachMilestone(m int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m <= s.state.Milestone {
		return false
	}
	s.state.Milestone = m
	return true
}

// AddWild puts a Pokémon in the tall-grass collection without spending a
// ball (an evolution reached it). False if it was already there.
func (s *Store) AddWild(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if contains(s.state.Wild, id) {
		return false
	}
	s.state.Wild = append(s.state.Wild, id)
	sort.Ints(s.state.Wild)
	return true
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

// day rolls Today over when the date changed; quests picks the new day's
// missions. Callers hold the lock.
func (s *Store) dayLocked(now time.Time, quests func(day string) []string) {
	day := now.Format("2006-01-02")
	if s.state.Today.Day == day {
		return
	}
	s.state.Today = Today{Day: day, Progress: map[string]int{}}
	if quests != nil {
		s.state.Today.Quests = quests(day)
	}
}

// Roll makes sure Today is today's, with its missions.
func (s *Store) Roll(now time.Time, quests func(day string) []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
}

// Count adds n to a mission kind's progress for today.
func (s *Store) Count(kind string, n int, now time.Time, quests func(day string) []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
	if s.state.Today.Progress == nil {
		s.state.Today.Progress = map[string]int{}
	}
	s.state.Today.Progress[kind] += n
}

// Claim marks a mission rewarded; false if it already was.
func (s *Store) Claim(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.state.Today.Claimed {
		if c == id {
			return false
		}
	}
	s.state.Today.Claimed = append(s.state.Today.Claimed, id)
	return true
}

// Pick records a hidden object picked up today; false if it already was.
func (s *Store) Pick(spot string, now time.Time, quests func(day string) []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
	for _, f := range s.state.Today.Found {
		if f == spot {
			return false
		}
	}
	s.state.Today.Found = append(s.state.Today.Found, spot)
	return true
}

// Beat records a trainer beaten today; false if already beaten.
func (s *Store) Beat(trainer string, now time.Time, quests func(day string) []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
	for _, b := range s.state.Today.Beaten {
		if b == trainer {
			return false
		}
	}
	s.state.Today.Beaten = append(s.state.Today.Beaten, trainer)
	return true
}

// AddItem gives (or, negative, takes) a bag item; false if there was none to take.
func (s *Store) AddItem(item string, n int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Bag == nil {
		s.state.Bag = map[string]int{}
	}
	if s.state.Bag[item]+n < 0 {
		return false
	}
	s.state.Bag[item] += n
	return true
}

// Feed spends a berry on the partner, once a day.
func (s *Store) Feed(now time.Time, quests func(day string) []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dayLocked(now, quests)
	if s.state.Today.Fed || s.state.Bag["berry"] <= 0 || s.state.Partner == nil {
		return false
	}
	s.state.Bag["berry"]--
	s.state.Today.Fed = true
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

	name, settings, created, lock := s.state.Name, s.state.Settings, s.state.Created, s.state.Lock
	s.state = State{
		Name:     name,
		Settings: settings,
		Created:  created,
		Lock:     lock,
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
	return s.write()
}

// write saves without touching UpdatedAt ("last played"): a wrong code is
// not a game. Owner-only: a save holds the hashed code, which 6561 tries undo.
func (s *Store) write() error {
	b, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	_ = os.Remove(tmp)
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
