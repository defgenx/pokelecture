package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

// Surprise exams: the event waiting on today's map, its grading, and the
// parent's view of the results. README « Les examens surprises ».

// eventView is today's exam, as the map shows it.
type eventView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Pokemon  int    `json:"pokemon"`
	Pass     int    `json:"pass"`
	Attempts int    `json:"attempts"`
	Best     int    `json:"best"`
}

func (s *Server) eventToday(st progress.State) *eventView {
	ev := s.cur.EventToday(st, time.Now())
	if ev == nil {
		return nil
	}
	r := st.Exams[ev.ID]
	return &eventView{ID: ev.ID, Title: ev.Title, Pokemon: ev.Pokemon, Pass: ev.PassMark(), Attempts: r.Attempts, Best: r.Best}
}

// missedView is one wrong answer, shown and playable on the result screen.
type missedView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Audio string `json:"audio,omitempty"`
}

type examResult struct {
	Passed   bool              `json:"passed"`
	Score    int               `json:"score"`
	Need     int               `json:"need"`
	Sections map[string][2]int `json:"sections"`
	Missed   []missedView      `json:"missed"`
}

// examBalls: what a first pass pays, on top of the Pokémon.
const examBalls = 5

// gradeExam scores an exam result; false when the id is not a known event.
// The session must have been asked in full: an attempt that answered fewer
// than half the questions is graded as failed.
func (s *Server) gradeExam(evID string, req resultRequest, res *resultResponse, now time.Time) bool {
	ev, ok := s.cur.Event(evID)
	if !ok {
		return false
	}
	answers := map[string]bool{}
	for _, it := range req.Items {
		if it.ID == "" {
			continue
		}
		if _, seen := answers[it.ID]; !seen {
			answers[it.ID] = it.Correct
		}
	}
	score, sections := curriculum.ExamScore(answers)
	passed := score >= ev.PassMark() && 2*len(answers) >= ev.Total()
	var missed []string
	for id, ok := range answers {
		if !ok {
			missed = append(missed, id)
		}
	}
	sort.Strings(missed)
	r := &examResult{Passed: passed, Score: score, Need: ev.PassMark(), Sections: sections, Missed: []missedView{}}
	for _, id := range missed {
		label, audio := s.cur.ItemLine(id)
		r.Missed = append(r.Missed, missedView{ID: id, Label: label, Audio: audio})
	}
	if s.store.RecordExam(ev.ID, score, passed, sections, missed, now) {
		if s.store.Catch(ev.Pokemon) {
			res.NewCatches = append(res.NewCatches, ev.Pokemon)
		}
		res.Balls = examBalls
		res.Tickets = s.payTicket()
	}
	res.Exam = r
	return true
}

// adminExamView is one exam in the admin: the parent's report card.
type adminExamView struct {
	ID       string              `json:"id"`
	Title    string              `json:"title"`
	After    string              `json:"after"`
	Pokemon  int                 `json:"pokemon"`
	Pass     int                 `json:"pass"`
	Eligible bool                `json:"eligible"`
	Today    bool                `json:"today"`
	Record   progress.ExamRecord `json:"record"`
	Missed   []missedView        `json:"missed"`
}

func (s *Server) adminExams(st progress.State) []adminExamView {
	today := s.cur.EventToday(st, time.Now())
	var out []adminExamView
	for _, ev := range s.cur.Events {
		r := st.Exams[ev.ID]
		v := adminExamView{ID: ev.ID, Title: ev.Title, After: ev.After, Pokemon: ev.Pokemon, Pass: ev.PassMark(),
			Eligible: s.cur.Eligible(ev, st), Today: today != nil && today.ID == ev.ID, Record: r, Missed: []missedView{}}
		for _, id := range r.Missed {
			label, audio := s.cur.ItemLine(id)
			v.Missed = append(v.Missed, missedView{ID: id, Label: label, Audio: audio})
		}
		out = append(out, v)
	}
	return out
}

// handleAdminEventToday puts an exam on today's map, whatever the dice said.
func (s *Server) handleAdminEventToday(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != "none" {
		if _, ok := s.cur.Event(id); !ok {
			http.Error(w, "examen inconnu", http.StatusNotFound)
			return
		}
	} else {
		id = ""
	}
	s.store.CallEvent(id, time.Now(), questsFor)
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func isExam(id string) (string, bool) {
	return strings.CutPrefix(id, curriculum.EventPrefix)
}
