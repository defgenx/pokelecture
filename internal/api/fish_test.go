package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
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
