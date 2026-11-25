package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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

func TestPartnerBallsAndGift(t *testing.T) {
	s, err := Open(t.TempDir()+"/p.json", "")
	if err != nil {
		t.Fatal(err)
	}
	if !s.ChoosePartner(4, 40) || s.ChoosePartner(7, 0) {
		t.Fatal("the partner is chosen exactly once")
	}
	st := s.Snapshot()
	if st.Partner.Line != 4 || st.Partner.XP != 40 || st.Balls != StarterBalls {
		t.Fatalf("after choosing: %+v balls=%d", st.Partner, st.Balls)
	}
	if _, b, a := s.GainXP(12); a-b != 12 {
		t.Fatalf("GainXP: %d → %d", b, a)
	}

	for i := 0; i < StarterBalls; i++ {
		if spent, _ := s.CatchWild(20, false); !spent {
			t.Fatalf("ball %d should be available", i+1)
		}
	}
	if spent, _ := s.CatchWild(21, false); spent {
		t.Fatal("no ball left: the catch must be refused")
	}
	if got := s.Snapshot().Wild; len(got) != 1 || got[0] != 20 {
		t.Fatalf("catching the same Pokémon twice keeps one entry, got %v", got)
	}

	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	if !s.OpenGift(now, 3) || s.OpenGift(now.Add(time.Hour), 3) || !s.OpenGift(now.AddDate(0, 0, 1), 3) {
		t.Fatal("the gift opens once per day")
	}
	if s.Snapshot().Balls != 6 {
		t.Fatalf("two gifts of 3 balls, got %d", s.Snapshot().Balls)
	}
}

func TestLockTriesGrowThenClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	st, err := Open(path, "Nino")
	if err != nil {
		t.Fatal(err)
	}
	st.SetLock([]int{25, 4, 7})
	clock := time.Date(2026, 11, 24, 18, 0, 0, 0, time.UTC)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()
	wrong := []int{52, 52, 52}

	if r, _ := st.TryLock(nil); r != LockWrong {
		t.Fatalf("sans code : %v", r)
	}
	for i := 0; i < 4; i++ {
		st.TryLock(wrong)
	}
	if r, _ := st.TryLock(nil); r != LockWrong {
		t.Fatalf("un code absent ne compte pas : %v", r)
	}
	st.TryLock(wrong) // 5th
	if r, d := st.TryLock([]int{25, 4, 7}); r != LockWait || d != 5*time.Minute {
		t.Fatalf("après 5 : attendre 5 min, %v %v", r, d)
	}
	// The count survives a restart.
	again, _ := Open(path, "")
	if r, _ := again.TryLock([]int{25, 4, 7}); r != LockWait {
		t.Fatalf("après redémarrage, toujours en attente : %v", r)
	}
	clock = clock.Add(6 * time.Minute)
	for i := 0; i < 5; i++ {
		st.TryLock(wrong)
	}
	if _, d := st.TryLock(wrong); d != time.Hour {
		t.Fatalf("après 10 : attendre 1 h, %v", d)
	}
	clock = clock.Add(2 * time.Hour)
	for i := 0; i < 5; i++ {
		st.TryLock(wrong)
	}
	if r, _ := st.TryLock([]int{25, 4, 7}); r != LockClosed {
		t.Fatalf("après 15 : fermé, même au bon code : %v", r)
	}
	clock = clock.Add(30 * 24 * time.Hour)
	if r, _ := st.TryLock([]int{25, 4, 7}); r != LockClosed {
		t.Fatalf("fermé ne s'ouvre pas avec le temps : %v", r)
	}
	tag := st.LockTag()
	st.ReopenLock()
	if r, _ := st.TryLock([]int{25, 4, 7}); r != LockOpen || st.LockTag() != tag {
		t.Fatalf("rouvert, même code, mêmes appareils : %v", r)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("la sauvegarde doit être 0600 : %v", info.Mode().Perm())
	}
}

// Concurrent tries are counted one by one: a burst cannot test more codes
// than the limit allows.
func TestLockTriesAreAtomic(t *testing.T) {
	st, _ := Open(filepath.Join(t.TempDir(), "p.json"), "Nino")
	st.SetLock([]int{0, 0, 0}) // never in the burst: no success resets the count
	var wg sync.WaitGroup
	var mu sync.Mutex
	tested := 0
	for _, a := range []int{25, 4, 7, 1, 133, 39, 52, 143, 54} {
		for _, b := range []int{25, 4, 7, 1, 133, 39, 52, 143, 54} {
			for _, c := range []int{25, 4, 7, 1, 133, 39, 52, 143, 54} {
				wg.Add(1)
				go func(code []int) {
					defer wg.Done()
					if r, _ := st.TryLock(code); r == LockWrong {
						mu.Lock()
						tested++
						mu.Unlock()
					}
				}([]int{a, b, c})
			}
		}
	}
	wg.Wait()
	if tested > 5 {
		t.Fatalf("%d codes évalués d'un coup, 5 au plus", tested)
	}
}

func TestAddTimesIgnoresAbsurdTimes(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	s.AddTimes(map[string][]int{"mot": {4000, 100, 120000, 2000}})
	s.AddTimes(map[string][]int{"syl": {50}})
	if _, kept := s.Snapshot().Speed["syl"]; kept {
		t.Fatal("a kind with only absurd times was stored")
	}
	got := s.Snapshot().Speed["mot"]
	if got.N != 2 || got.MS != 4000+(2000-4000)/8 {
		t.Fatalf("got %+v", got)
	}
}

func TestThreeStarSaveBecomesFiveStars(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	old := `{"name":"Nino","stars":6,"episodes":{"ep01":{"completions":1,"best_stars":3},"ep02":{"completions":2,"best_stars":2},"ep03":{"completions":1,"best_stars":1}},"items":{},"settings":{"min_stars":2,"episode_stars":{"ep03b":3},"battle_lives":3}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	st := s.Snapshot()
	if st.Episodes["ep01"].BestStars != 5 || st.Episodes["ep02"].BestStars != 3 || st.Episodes["ep03"].BestStars != 1 {
		t.Fatalf("best stars: %+v", st.Episodes)
	}
	if st.Stars != 9 || st.Settings.MinStars != 3 || st.Settings.EpisodeStars["ep03b"] != 5 || st.StarScale != MaxStars {
		t.Fatalf("total %d, min %d, override %d, scale %d", st.Stars, st.Settings.MinStars, st.Settings.EpisodeStars["ep03b"], st.StarScale)
	}
	// Converted once: reopening changes nothing.
	s2, _ := Open(path, "")
	if s2.Snapshot().Stars != 9 {
		t.Fatal("converted twice")
	}
}

// Single-board memory/paires times cannot be compared with the 4-board games:
// dropped once, the other records kept, and a new record survives a reopen.
func TestBoardGameRecordsDroppedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	old := `{"name":"Nino","star_scale":5,"episodes":{},"items":{},"records":{"memory":30,"paires":25,"lecture":12}}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	r := s.Snapshot().Records
	if _, ok := r["memory"]; ok || r["lecture"] != 12 {
		t.Fatalf("records after migration: %v", r)
	}
	if _, ok := r["paires"]; ok {
		t.Fatal("paires record kept")
	}
	s.Record("memory", 80, true)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, _ := Open(path, "")
	if s2.Snapshot().Records["memory"] != 80 {
		t.Fatal("dropped again on reopen")
	}
}

// Yesterday's lessons do not count against today's objective.
func TestDayOverIsToday(t *testing.T) {
	now := time.Date(2026, 11, 25, 18, 0, 0, 0, time.Local)
	st := State{Settings: Settings{DailyLimit: true, DailyLessons: 2}, Today: Today{Day: "2026-11-24", Lessons: 5}}
	if st.DayOver(now) {
		t.Fatal("yesterday's lessons closed today")
	}
	st.Today = Today{Day: "2026-11-25", Lessons: 2, Quests: []string{"catch", "feed"}, Claimed: []string{"catch"}}
	if !st.LessonsOver(now) || st.DayOver(now) {
		t.Fatal("lessons done, a mission left: routes close, the day stays open")
	}
	st.Today.Claimed = append(st.Today.Claimed, "feed")
	if !st.DayOver(now) {
		t.Fatal("every mission done but the day is open")
	}
	st.Settings.DailyLimit = false
	if st.DayOver(now) {
		t.Fatal("limit off but the day is closed")
	}
}

// The number of objectives decides the day: the lessons plus N−1 missions,
// never the route mission under the limit.
func TestDailyObjectives(t *testing.T) {
	now := time.Date(2026, 11, 25, 18, 0, 0, 0, time.Local)
	st := State{
		Settings: Settings{DailyLimit: true, DailyLessons: 1, DailyMissions: 3},
		Today:    Today{Day: "2026-11-25", Lessons: 1, Quests: []string{"route", "catch", "feed", "find"}},
	}
	if got := st.ActiveQuests(); len(got) != 2 || got[0] != "catch" || got[1] != "feed" {
		t.Fatalf("3 objectives: %v, want the lessons + catch, feed", got)
	}
	st.Settings.DailyMissions = 1
	if len(st.ActiveQuests()) != 0 || !st.DayOver(now) {
		t.Fatal("1 objective = the lessons alone: the day must end with them")
	}
	st.Settings.DailyLimit = false
	if got := st.ActiveQuests(); len(got) != 3 || got[0] != "route" {
		t.Fatalf("no limit: the usual three missions, got %v", got)
	}
}
