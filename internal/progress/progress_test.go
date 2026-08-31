package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Fixed so streak arithmetic never depends on when the suite runs.
var refTime = time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)

// The name is the one setting a parent changes on purpose after the savegame
// already exists, so the startup value has to win over the stored one.
func TestOpenNameResolution(t *testing.T) {
	t.Run("fresh savegame falls back to the default", func(t *testing.T) {
		s := open(t, newPath(t), "")
		if got := s.Name(); got != DefaultName {
			t.Fatalf("Name() = %q, want %q", got, DefaultName)
		}
	})

	t.Run("fresh savegame takes the configured name", func(t *testing.T) {
		s := open(t, newPath(t), "Naïs")
		if got := s.Name(); got != "Naïs" {
			t.Fatalf("Name() = %q, want %q", got, "Naïs")
		}
	})

	t.Run("configured name overrides the stored one", func(t *testing.T) {
		path := newPath(t)
		open(t, path, "Dresseur")

		s := open(t, path, "Sacha")
		if got := s.Name(); got != "Sacha" {
			t.Fatalf("Name() = %q, want %q — startup value must win", got, "Sacha")
		}
		if got := storedName(t, path); got != "Sacha" {
			t.Fatalf("persisted name = %q, want %q", got, "Sacha")
		}
	})

	t.Run("no configured name keeps the stored one", func(t *testing.T) {
		path := newPath(t)
		open(t, path, "Sacha")

		s := open(t, path, "")
		if got := s.Name(); got != "Sacha" {
			t.Fatalf("Name() = %q, want %q — an unset name must not rename", got, "Sacha")
		}
	})
}

// Renaming must not cost the child their progress.
func TestOpenRenameKeepsProgress(t *testing.T) {
	path := newPath(t)

	s := open(t, path, "Dresseur")
	if !s.Catch(25) {
		t.Fatal("Catch(25) = false on a fresh savegame")
	}
	s.CompleteEpisode("ep01", 3, refTime)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	s = open(t, path, "Sacha")
	st := s.Snapshot()
	if st.Name != "Sacha" {
		t.Errorf("Name = %q, want %q", st.Name, "Sacha")
	}
	if st.Stars != 3 {
		t.Errorf("Stars = %d, want 3", st.Stars)
	}
	if len(st.Pokedex) != 1 || st.Pokedex[0] != 25 {
		t.Errorf("Pokedex = %v, want [25]", st.Pokedex)
	}
	if st.Episodes["ep01"].BestStars != 3 {
		t.Errorf("ep01 best stars = %d, want 3", st.Episodes["ep01"].BestStars)
	}
}

// Resetting an episode must reclaim its stars, or replaying it double-counts:
// CompleteEpisode awards the delta against a BestStars the reset just erased.
func TestResetEpisodeReclaimsStars(t *testing.T) {
	s := open(t, newPath(t), "")

	s.CompleteEpisode("ep01", 3, refTime)
	s.CompleteEpisode("ep02", 2, refTime)
	if got := s.Snapshot().Stars; got != 5 {
		t.Fatalf("Stars = %d, want 5", got)
	}

	s.ResetEpisode("ep01")
	if got := s.Snapshot().Stars; got != 2 {
		t.Fatalf("Stars after reset = %d, want 2", got)
	}

	s.CompleteEpisode("ep01", 3, refTime)
	if got := s.Snapshot().Stars; got != 5 {
		t.Fatalf("Stars after replay = %d, want 5 (no double count)", got)
	}

	// Resetting an episode that was never played must not touch the total.
	s.ResetEpisode("ep99")
	if got := s.Snapshot().Stars; got != 5 {
		t.Fatalf("Stars after no-op reset = %d, want 5", got)
	}
}

func newPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "var", "progress.json")
}

func open(t *testing.T, path, name string) *Store {
	t.Helper()
	s, err := Open(path, name)
	if err != nil {
		t.Fatalf("Open(%q, %q): %v", path, name, err)
	}
	return s
}

func storedName(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return st.Name
}
