package progress

// Several children, one save each. The registry holds one Store per player,
// each in its own file var/players/<id>.json; a player is created and
// removed from the admin page, and each device remembers which player it is.
// A deleted player's file is moved to var/players/trash/, never destroyed.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry is every player's save.
type Registry struct {
	mu     sync.Mutex
	dir    string // var/players ("" for an in-memory registry: tests)
	stores map[string]*Store
	key    []byte // signs each device's right to play a player
}

// Key signs the device cookies: kept in var/players/.device-key, so a
// restart does not ask every device for its codes again.
func (r *Registry) Key() []byte { return r.key }

func loadKey(dir string) ([]byte, error) {
	if dir == "" {
		k := make([]byte, 32)
		_, err := rand.Read(k)
		return k, err
	}
	path := filepath.Join(dir, ".device-key")
	if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
		return b, nil
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	return k, os.WriteFile(path, k, 0o600)
}

// PlayerInfo is one row of the player list.
type PlayerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OpenRegistry loads var/players/*.json. A save from before players existed
// (var/progress.json) becomes the first player, and is kept beside as
// progress.json.before-players; legacyName is the startup -name, applied to
// that migrated player only.
func OpenRegistry(varDir, legacyName string) (*Registry, error) {
	r := &Registry{dir: filepath.Join(varDir, "players"), stores: map[string]*Store{}}
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(r.dir, 0o700) // owner-only, also for a folder made before
	key, err := loadKey(r.dir)
	if err != nil {
		return nil, err
	}
	r.key = key
	legacy := filepath.Join(varDir, "progress.json")
	if _, err := os.Stat(legacy); err == nil {
		files, _ := filepath.Glob(filepath.Join(r.dir, "*.json"))
		if len(files) == 0 {
			id := newID()
			b, err := os.ReadFile(legacy)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(r.dir, id+".json"), b, 0o600); err != nil {
				return nil, err
			}
			if err := os.Rename(legacy, legacy+".before-players"); err != nil {
				return nil, err
			}
		}
	}
	files, err := filepath.Glob(filepath.Join(r.dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		if !validID(id) {
			continue
		}
		name := ""
		if len(files) == 1 {
			name = legacyName // a lone player keeps honouring -name / POKELECTURE_NAME
		}
		st, err := Open(f, name)
		if err != nil {
			return nil, fmt.Errorf("joueur %s : %w", id, err)
		}
		r.stores[id] = st
	}
	if len(r.stores) == 0 {
		if _, err := r.Create(legacyName); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Single wraps one store as a one-player registry (tests, tools).
func Single(st *Store) *Registry {
	key, _ := loadKey("")
	return &Registry{stores: map[string]*Store{"p1": st}, key: key}
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// validID: what newID makes, so an id from a cookie or a URL can never
// name a path outside var/players.
func validID(id string) bool {
	if id == "" || len(id) > 16 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Get returns a player's store.
func (r *Registry) Get(id string) (*Store, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.stores[id]
	return st, ok
}

// Only returns the store when there is exactly one player.
func (r *Registry) Only() (string, *Store, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.stores) != 1 {
		return "", nil, false
	}
	for id, st := range r.stores {
		return id, st, true
	}
	return "", nil, false
}

// List returns the players, oldest first.
func (r *Registry) List() []PlayerInfo {
	r.mu.Lock()
	ids := make([]string, 0, len(r.stores))
	for id := range r.stores {
		ids = append(ids, id)
	}
	stores := make(map[string]*Store, len(ids))
	for id, st := range r.stores {
		stores[id] = st
	}
	r.mu.Unlock()
	out := make([]PlayerInfo, 0, len(ids))
	created := map[string]time.Time{}
	for _, id := range ids {
		snap := stores[id].Snapshot()
		out = append(out, PlayerInfo{ID: id, Name: snap.Name, UpdatedAt: snap.UpdatedAt})
		created[id] = snap.Created
	}
	// Oldest first: the player who was there before multi-player (no date)
	// leads, then each new one in the order the parents added them.
	sort.Slice(out, func(i, j int) bool {
		ci, cj := created[out[i].ID], created[out[j].ID]
		if !ci.Equal(cj) {
			return ci.Before(cj)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Create adds a player with an empty save and returns its id.
func (r *Registry) Create(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultName
	}
	if len([]rune(name)) > 30 {
		return "", errors.New("prénom trop long (30 caractères au plus)")
	}
	id := newID()
	var st *Store
	var err error
	if r.dir == "" {
		st, err = Open(filepath.Join(os.TempDir(), "pokelecture-"+id+".json"), name)
	} else {
		st, err = Open(filepath.Join(r.dir, id+".json"), name)
	}
	if err != nil {
		return "", err
	}
	r.mu.Lock()
	r.stores[id] = st
	r.mu.Unlock()
	return id, nil
}

// Delete removes a player: its file goes to var/players/trash/, recoverable
// by moving it back. The last player cannot be deleted.
func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.stores[id]; !ok {
		return errors.New("joueur inconnu")
	}
	if len(r.stores) == 1 {
		return errors.New("il faut au moins un joueur")
	}
	if r.dir != "" {
		trash := filepath.Join(r.dir, "trash")
		if err := os.MkdirAll(trash, 0o700); err != nil {
			return err
		}
		dst := filepath.Join(trash, fmt.Sprintf("%s-%s.json", id, time.Now().Format("20060102-150405")))
		if err := os.Rename(filepath.Join(r.dir, id+".json"), dst); err != nil {
			return err
		}
	}
	delete(r.stores, id)
	return nil
}
