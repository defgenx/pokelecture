package curriculum

import (
	"strings"
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

func TestTowerLadderIndexRoundTrips(t *testing.T) {
	for floor, want := range map[int]int{1: -1, 2: 0, 3: -1, 40: 19, 41: 20, 42: 21, 70: 49} {
		if got := towerLadderIndex(floor); got != want {
			t.Errorf("floor %d: index %d, want %d", floor, got, want)
		}
		if want >= 0 && TowerFloorOf(want) != floor {
			t.Errorf("index %d: floor %d, want %d", want, TowerFloorOf(want), floor)
		}
	}
}

func TestEventAppearsOncePerWindow(t *testing.T) {
	ev := Event{ID: "x", Every: 4}
	for w := 0; w < 30; w++ {
		n := 0
		for d := 0; d < 4; d++ {
			if ev.appearsOn(eventEpoch.Add(time.Duration(w*4+d)*24*time.Hour + time.Hour)) {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("window %d: %d days, want exactly 1", w, n)
		}
	}
}

func TestExamScore(t *testing.T) {
	pct, secs := ExamScore(map[string]bool{"son:ch": true, "son:ou": false, "mot:Abo": true, "syl:ma": true})
	if pct != 75 || secs["son"] != [2]int{1, 2} || secs["mot"] != [2]int{1, 1} {
		t.Fatalf("got %d %v", pct, secs)
	}
}

func TestBuildExamIsStrictAndFair(t *testing.T) {
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	for _, ep := range c.Episodes {
		st.Episodes[ep.ID] = progress.EpisodeStat{Completions: 1}
		if ep.ID == "ep24b" {
			break
		}
	}
	ev, ok := c.Event("grand")
	if !ok {
		t.Fatal("no grand exam")
	}
	sess, ok := c.BuildExam(ev, st, time.Now())
	if !ok || sess.Exam == nil || sess.Exam.Total != ev.Total() {
		t.Fatalf("built %v, total %v, want %d", ok, sess.Exam, ev.Total())
	}
	for _, a := range sess.Activities {
		if a.Kind != "examen" || a.Text != SectionSons {
			continue
		}
		for _, it := range a.Items {
			right := strings.TrimPrefix(it.ID, "son:")
			for _, ch := range it.Choices {
				if !ch.Correct && soundsAlike(ch.Label, right) {
					t.Errorf("%s offered with its homophone %s", right, ch.Label)
				}
			}
		}
	}
	// Passed: never drawn again; not yet eligible: never drawn.
	st.Exams = map[string]progress.ExamRecord{"grand": {Passed: true}}
	for d := 0; d < 20; d++ {
		if e := c.EventToday(st, time.Now().Add(time.Duration(d)*24*time.Hour)); e != nil && (e.ID == "grand" || e.ID == "maitre") {
			t.Fatalf("day %d: %s drawn", d, e.ID)
		}
	}
}

func TestGamesOpenAlongTheStory(t *testing.T) {
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	if got := c.GamesOpen(st); len(got) != 0 {
		t.Fatalf("fresh save: %v open", got)
	}
	st.Episodes["ep01"] = progress.EpisodeStat{Completions: 1}
	if got := c.GamesOpen(st); len(got) != 1 || got[0] != "memory" {
		t.Fatalf("after ep01: %v", got)
	}
	// An arena done but its route replayed-as-new: still open.
	st.Episodes = map[string]progress.EpisodeStat{"ep03b": {Completions: 1}}
	if got := c.GamesOpen(st); len(got) != 5 {
		t.Fatalf("after ep03b only: %v", got)
	}
	st.Settings.FreePlay = true
	if got := c.GamesOpen(st); len(got) != len(GameUnlocks) {
		t.Fatalf("free play: %v", got)
	}
}
