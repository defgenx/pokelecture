package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

// A water Pokémon is fished, never met in the grass: without the rod the
// server refuses the catch; a route at RodStars from RodFrom on earns it.
func TestFishingNeedsTheRod(t *testing.T) {
	srv, store := serverAt(t, "ep20") // every route at 3 stars: no rod yet
	store.AddBalls(20)
	throw := func() int {
		b, _ := json.Marshal(map[string]any{"id": 120, "attempt": 1}) // Stari, a water Pokémon
		resp, err := http.Post(srv.URL+"/api/wild/catch", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	if st.Rod {
		t.Fatal("rod without a 4-star route")
	}
	if code := throw(); code != http.StatusBadRequest {
		t.Fatalf("catching a water Pokémon without the rod: %d", code)
	}

	store.CompleteEpisode("ep12", 5, time.Now()) // before the Côte Saphir: no rod
	getJSON(t, srv.URL+"/api/state", &st)
	if st.Rod {
		t.Fatal("rod from a route before the Côte Saphir")
	}
	store.CompleteEpisode("ep14", 4, time.Now())
	getJSON(t, srv.URL+"/api/state", &st)
	if !st.Rod {
		t.Fatal("no rod after a 4-star Côte Saphir route")
	}
	if code := throw(); code != http.StatusOK {
		t.Fatalf("catching a water Pokémon with the rod: %d", code)
	}
}

// The wild Pokédex lists the water Pokémon too, marked as such.
func TestWildPoolMarksWaterPokemon(t *testing.T) {
	srv, _ := serverAt(t, "ep20")
	var pool wildPoolView
	getJSON(t, srv.URL+"/api/wild", &pool)
	water, grass := 0, 0
	for _, p := range pool.Pokemon {
		if p.Water {
			water++
		} else {
			grass++
		}
	}
	if water < 10 || grass < 50 {
		t.Fatalf("%d water, %d grass Pokémon in the wild pool", water, grass)
	}
}

// The route that earns the rod says so once, the next one does not.
func TestRouteAnnouncesTheRod(t *testing.T) {
	srv, _ := serverAt(t, "ep20")
	post := func(ep string, stars int) resultResponse {
		b, _ := json.Marshal(map[string]any{"items": []any{}, "stars": stars, "completed": true})
		resp, err := http.Post(srv.URL+"/api/session/"+ep+"/result", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var res resultResponse
		json.NewDecoder(resp.Body).Decode(&res)
		return res
	}
	if res := post("ep14", 3); res.RodWon {
		t.Fatal("rod won at 3 stars")
	}
	if res := post("ep14", 4); !res.RodWon || !res.State.Rod {
		t.Fatalf("4 stars on ep14: rod_won %v, rod %v", res.RodWon, res.State.Rod)
	}
	if res := post("ep15", 5); res.RodWon {
		t.Fatal("rod announced twice")
	}
}

// Free play opens every place on the map, as if the whole course were done.
func TestFreePlayOpensEveryPlace(t *testing.T) {
	cur, err := curriculum.Load(filepath.Join("../..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	store.ChoosePartner(4, 0)
	store.UpdateSettings(0, true, 3) // no route done, free play on
	srv := httptest.NewServer(New(cur, store, filepath.Join("../..", "web"), "/").Routes())
	t.Cleanup(srv.Close)
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	if !st.Tower || !st.Rod {
		t.Fatalf("free play: tower %v, rod %v", st.Tower, st.Rod)
	}
	var pool wildPoolView
	if code := getJSON(t, srv.URL+"/api/wild", &pool); code != http.StatusOK {
		t.Fatalf("wild pool under free play: %d", code)
	}
}

// The daily limit: once the objective is reached, nothing more starts today.
func TestDailyLimitStopsTheDay(t *testing.T) {
	srv, store := serverAt(t, "ep05")
	store.SetDailyLimit(true, 1)
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	if st.Daily == nil || st.Daily.Over || st.Daily.Goal != 1 {
		t.Fatalf("before any lesson: %+v", st.Daily)
	}
	if code := getJSON(t, srv.URL+"/api/session/ep06", nil); code != http.StatusOK {
		t.Fatalf("a lesson before the objective: %d", code)
	}
	b, _ := json.Marshal(map[string]any{"items": []any{}, "stars": 3, "completed": true})
	resp, err := http.Post(srv.URL+"/api/session/ep06/result", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var res resultResponse
	json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	paid := false
	for _, q := range res.QuestsDone {
		paid = paid || q.ID == lessonQuest
	}
	if !paid {
		t.Fatalf("the lessons mission was not paid: %+v", res.QuestsDone)
	}
	getJSON(t, srv.URL+"/api/state", &st)
	if st.Daily == nil || !st.Daily.LessonsOver || st.Daily.Over || st.Daily.Done != 1 {
		t.Fatalf("after the lessons: %+v", st.Daily)
	}
	lessonMission := false
	for _, q := range st.Quests {
		if q.ID == lessonQuest && q.Done && q.Goal == 1 {
			lessonMission = true
		}
	}
	if !lessonMission {
		t.Fatalf("the lessons are not a done mission: %+v", st.Quests)
	}
	// Lessons done: no more routes, but the other missions stay open.
	if code := getJSON(t, srv.URL+"/api/session/ep07", nil); code != http.StatusForbidden {
		t.Errorf("a route after the lessons: %d", code)
	}
	if code := getJSON(t, srv.URL+"/api/session/tour", nil); code != http.StatusOK {
		t.Errorf("the Tower before the missions are done: %d", code)
	}
	// Every mission done: nothing more at all.
	for _, q := range store.Snapshot().Today.Quests {
		store.Claim(q)
	}
	getJSON(t, srv.URL+"/api/state", &st)
	if !st.Daily.Over {
		t.Fatalf("every mission done: %+v", st.Daily)
	}
	for _, path := range []string{"/api/session/ep07", "/api/session/tour", "/api/session/dresseur-0-0"} {
		if code := getJSON(t, srv.URL+path, nil); code != http.StatusForbidden {
			t.Errorf("%s after every mission: %d", path, code)
		}
	}
	store.SetDailyLimit(false, 1)
	if code := getJSON(t, srv.URL+"/api/session/ep07", nil); code != http.StatusOK {
		t.Fatalf("limit off: %d", code)
	}
}
