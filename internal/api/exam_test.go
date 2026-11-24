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

func examServer(t *testing.T) (*httptest.Server, *progress.Store, *curriculum.Curriculum) {
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
		if ep.ID == "ep06b" {
			break
		}
	}
	srv := httptest.NewServer(New(cur, store, filepath.Join("../..", "web"), "/").Routes())
	t.Cleanup(srv.Close)
	return srv, store, cur
}

func postExam(t *testing.T, srv *httptest.Server, id string, sess curriculum.Session, right func(i int) bool) resultResponse {
	t.Helper()
	var items []map[string]any
	i := 0
	for _, a := range sess.Activities {
		for _, it := range a.Items {
			items = append(items, map[string]any{"id": it.ID, "correct": right(i)})
			i++
		}
	}
	b, _ := json.Marshal(map[string]any{"items": items})
	resp, err := http.Post(srv.URL+"/api/session/examen-"+id+"/result", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res resultResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestExamFailThenPass(t *testing.T) {
	srv, store, cur := examServer(t)
	ev, _ := cur.Event("petit")
	resp, err := http.Get(srv.URL + "/api/session/examen-petit")
	if err != nil {
		t.Fatal(err)
	}
	var sess curriculum.Session
	json.NewDecoder(resp.Body).Decode(&sess)
	resp.Body.Close()
	if sess.Exam == nil || sess.Exam.Total == 0 {
		t.Fatal("no exam payload")
	}

	res := postExam(t, srv, "petit", sess, func(i int) bool { return i%2 == 0 })
	if res.Exam == nil || res.Exam.Passed || len(res.NewCatches) != 0 || len(res.Exam.Missed) == 0 {
		t.Fatalf("a half-right exam: %+v", res.Exam)
	}
	if st := store.Snapshot(); st.Exams["petit"].Attempts != 1 || st.Exams["petit"].Passed {
		t.Fatalf("record after a fail: %+v", st.Exams["petit"])
	}

	res = postExam(t, srv, "petit", sess, func(int) bool { return true })
	if !res.Exam.Passed || len(res.NewCatches) != 1 || res.NewCatches[0] != ev.Pokemon {
		t.Fatalf("a full pass: %+v catches %v", res.Exam, res.NewCatches)
	}
	if res.State.Event != nil && res.State.Event.ID == "petit" {
		t.Fatal("a passed exam is still on the map")
	}
	// Passing again pays nothing more.
	if res = postExam(t, srv, "petit", sess, func(int) bool { return true }); len(res.NewCatches) != 0 || res.Balls != 0 {
		t.Fatalf("second pass paid again: %v %d", res.NewCatches, res.Balls)
	}
}

func TestExamTooShortFails(t *testing.T) {
	srv, _, _ := examServer(t)
	b, _ := json.Marshal(map[string]any{"items": []map[string]any{{"id": "son:a", "correct": true}}})
	resp, err := http.Post(srv.URL+"/api/session/examen-petit/result", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res resultResponse
	json.NewDecoder(resp.Body).Decode(&res)
	if res.Exam == nil || res.Exam.Passed {
		t.Fatalf("one answer passed the exam: %+v", res.Exam)
	}
}
