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

func serverAt(t *testing.T, upTo string) (*httptest.Server, *progress.Store) {
	t.Helper()
	cur, err := curriculum.Load(filepath.Join("../..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	store.ChoosePartner(4, 0)
	for _, ep := range cur.Episodes {
		store.CompleteEpisode(ep.ID, 3, time.Now())
		if ep.ID == upTo {
			break
		}
	}
	srv := httptest.NewServer(New(cur, store, filepath.Join("../..", "web"), "/").Routes())
	t.Cleanup(srv.Close)
	return srv, store
}

func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil {
		json.NewDecoder(resp.Body).Decode(v)
	}
	return resp.StatusCode
}

func TestAcademyDoorIsTheGrandExam(t *testing.T) {
	srv, store := serverAt(t, "ep35")
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	if st.NextID != "ep36" || st.Event == nil || st.Event.ID != "grand" {
		t.Fatalf("at the Académie door: next %q, event %+v", st.NextID, st.Event)
	}
	for _, e := range st.Episodes {
		if e.ID == "ep36" && (e.Unlocked || e.Gate != "grand") {
			t.Fatalf("ep36 should be a closed door: %+v", e)
		}
	}
	var sess curriculum.Session
	getJSON(t, srv.URL+"/api/session/ep36", &sess)
	if sess.Exam == nil || sess.Exam.ID != "grand" {
		t.Fatal("JOUER on the door should play the Grand Examen")
	}
	// The door's exam waits every day, even right after a failed try.
	store.RecordExam("grand", 40, false, nil, nil, time.Now().Add(-24*time.Hour))
	getJSON(t, srv.URL+"/api/state", &st)
	if st.Event == nil || st.Event.ID != "grand" {
		t.Fatal("the entry exam must come back the next day")
	}
	store.RecordExam("grand", 90, true, nil, nil, time.Now())
	var after stateView // fresh: a decode keeps the fields a reply omits
	getJSON(t, srv.URL+"/api/state", &after)
	for _, e := range after.Episodes {
		if e.ID == "ep36" && (!e.Unlocked || e.Gate != "") {
			t.Fatalf("passed: ep36 should open: %+v", e)
		}
	}
}

func TestAcademyDoorKeepsAnOldSaveGoing(t *testing.T) {
	srv, _ := serverAt(t, "ep37") // already in the Académie before the gate existed
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	for _, e := range st.Episodes {
		if e.ID == "ep38" && !e.Unlocked {
			t.Fatal("a save past the door must not be locked out")
		}
	}
}

func TestTicketsEarnedByReading(t *testing.T) {
	srv, store := serverAt(t, "ep02")
	spend := func() int {
		resp, err := http.Post(srv.URL+"/api/ticket", "application/json", bytes.NewReader([]byte(`{"game":"memory"}`)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	for i := 0; i < st.Bag["ticket"]; i++ {
		if code := spend(); code != http.StatusOK {
			t.Fatalf("ticket %d refused: %d", i, code)
		}
	}
	if code := spend(); code != http.StatusConflict {
		t.Fatalf("no ticket left: got %d, want 409", code)
	}
	b, _ := json.Marshal(map[string]any{"items": []any{}, "stars": 3, "completed": true})
	resp, err := http.Post(srv.URL+"/api/session/ep03/result", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var res resultResponse
	json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	if res.Tickets != 1 || store.Snapshot().Bag["ticket"] != 1 {
		t.Fatalf("a finished route pays one ticket: %d, bag %d", res.Tickets, store.Snapshot().Bag["ticket"])
	}
}

func TestInsertedEpisodeLocksNothingBeyond(t *testing.T) {
	cur, err := curriculum.Load(filepath.Join("../..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	store.ChoosePartner(4, 0)
	// A save that played every Faille but the new one (ep34b).
	for _, ep := range cur.Episodes {
		if ep.ID != "ep34b" {
			store.CompleteEpisode(ep.ID, 5, time.Now())
		}
		if ep.ID == "ep35" {
			break
		}
	}
	srv := httptest.NewServer(New(cur, store, filepath.Join("../..", "web"), "/").Routes())
	defer srv.Close()
	var st stateView
	getJSON(t, srv.URL+"/api/state", &st)
	if st.NextID != "ep34b" {
		t.Fatalf("the new Faille should be next, got %q", st.NextID)
	}
	for _, e := range st.Episodes {
		if (e.ID == "ep35" || e.ID == "ep34b") && !e.Unlocked {
			t.Fatalf("%s locked by the inserted episode", e.ID)
		}
	}
}
