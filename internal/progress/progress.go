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

// State is the whole savegame.
type State struct {
	Name      string                 `json:"name"`
	Stars     int                    `json:"stars"`
	Pokedex   []int                  `json:"pokedex"`
	Episodes  map[string]EpisodeStat `json:"episodes"`
	Items     map[string]Item        `json:"items"`
	Streak    int                    `json:"streak"`
	LastDay   string                 `json:"last_day"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Store is a mutex-guarded State persisted to a single file.
type Store struct {
	mu    sync.Mutex
	path  string
	state State
}

// Open loads the savegame, creating an empty one if the file does not exist.
func Open(path string, name string) (*Store, error) {
	s := &Store{path: path, state: State{
		Name:     name,
		Episodes: map[string]EpisodeStat{},
		Items:    map[string]Item{},
	}}

	b, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
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
	if s.state.Name == "" {
		s.state.Name = name
	}
	return s, nil
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
