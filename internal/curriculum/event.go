package curriculum

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// Surprise exams (data/events.json): an event shows up on the map on random
// days once its condition is met, and is a strict test of everything learned.
// README « Les examens surprises » owns the rules.

// EventPrefix starts an exam's session id: "examen-<event id>".
const EventPrefix = "examen-"

// Exam section ids, also the prefixes of the item ids they record.
const (
	SectionSons     = "son"
	SectionSyllabes = "syl"
	SectionMots     = "mot"
	SectionPhrases  = "phrase"
)

// Event is one surprise exam.
type Event struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Story string `json:"story"`
	// After is the episode that must be completed before the event can appear.
	After string `json:"after"`
	// Every: the event shows up on one random day in every Every days.
	Every int `json:"every"`
	// Pass is the score to reach, in percent of right answers.
	Pass    int `json:"pass"`
	Pokemon int `json:"pokemon"`
	// Questions per section: sons, syllabes, mots, phrases.
	Questions map[string]int `json:"questions"`
}

func (e Event) every() int {
	if e.Every < 1 {
		return 4
	}
	return e.Every
}

// PassMark is the percentage needed to pass.
func (e Event) PassMark() int {
	if e.Pass <= 0 {
		return 80
	}
	return e.Pass
}

// Total is the number of questions the exam asks.
func (e Event) Total() int {
	n := 0
	for _, v := range e.Questions {
		n += v
	}
	return n
}

// Event returns an event by id.
func (c *Curriculum) Event(id string) (*Event, bool) {
	for i := range c.Events {
		if c.Events[i].ID == id {
			return &c.Events[i], true
		}
	}
	return nil, false
}

// ParseEvent reads an exam session id.
func ParseEvent(id string) (string, bool) {
	return strings.CutPrefix(id, EventPrefix)
}

// Eligible: its episode, or any later one, is completed and it is not passed
// yet. "Later one" keeps it reachable after an admin replay-as-new.
func (c *Curriculum) Eligible(ev Event, st progress.State) bool {
	return c.ReachedEpisode(ev.After, st) && !st.Exams[ev.ID].Passed
}

// ReachedEpisode: id or an episode after it has been completed.
func (c *Curriculum) ReachedEpisode(id string, st progress.State) bool {
	i := c.Index(id)
	if i < 0 {
		return false
	}
	for _, ep := range c.Episodes[i:] {
		if st.Episodes[ep.ID].Completions > 0 {
			return true
		}
	}
	return false
}

// WordGraphemes cuts a word with the signs known where it was taught: "Aile"
// is a-i-l-e on its route, not the "ail" an Académie lesson teaches later.
func (c *Curriculum) WordGraphemes(text string) []string {
	for i, ep := range c.Episodes {
		for _, w := range ep.Words {
			if w.Text == text {
				gs, _ := Segment(text, c.KnownGraphemes(i))
				return gs
			}
		}
	}
	return nil
}

var eventEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)

// appearsOn: one day per window of Every days, picked by a hash of the event
// and the window — random-looking, the same for every reload of that day.
func (e Event) appearsOn(now time.Time) bool {
	day := int(now.Sub(eventEpoch).Hours() / 24)
	if day < 0 {
		day = 0
	}
	n := e.every()
	h := fnv.New64a()
	fmt.Fprintf(h, "%s:%d", e.ID, day/n)
	return day%n == int(h.Sum64()%uint64(n))
}

// Gated: the episode's entry exam is not passed yet, and he has not been
// past it already (a save from before the gate keeps its way).
func (c *Curriculum) Gated(ep Episode, st progress.State) bool {
	return ep.RequiresExam != "" && !st.Exams[ep.RequiresExam].Passed && !c.ReachedEpisode(ep.ID, st)
}

// GateToday is the entry exam of the door he stands at: the episode before
// the gated one is done. It waits on the map every day until passed.
func (c *Curriculum) GateToday(st progress.State) *Event {
	for i, ep := range c.Episodes {
		if i == 0 || !c.Gated(ep, st) || st.Episodes[c.Episodes[i-1].ID].Completions == 0 {
			continue
		}
		if ev, ok := c.Event(ep.RequiresExam); ok {
			return ev
		}
	}
	return nil
}

// EventToday is the exam waiting on the map today, if any: an entry exam at
// his door, the one a parent called up from the admin, else the first
// eligible event whose day it is.
func (c *Curriculum) EventToday(st progress.State, now time.Time) *Event {
	if ev := c.GateToday(st); ev != nil {
		return ev
	}
	if st.Today.Day == now.Format("2006-01-02") && st.Today.Event != "" {
		if ev, ok := c.Event(st.Today.Event); ok && !st.Exams[ev.ID].Passed {
			return ev
		}
	}
	// At most one exam a week: an exam tried in the last days keeps the others
	// away (a retry the same day stays possible).
	today := now.Format("2006-01-02")
	recent := false
	for _, ev := range c.Events {
		r := st.Exams[ev.ID]
		if r.LastAt.IsZero() || now.Sub(r.LastAt) >= ExamGap {
			continue
		}
		if r.LastAt.Format("2006-01-02") == today && !r.Passed {
			return c.eventPtr(ev.ID)
		}
		recent = true
	}
	if recent {
		return nil
	}
	for i := range c.Events {
		ev := c.Events[i]
		if c.Eligible(ev, st) && ev.appearsOn(now) {
			return &c.Events[i]
		}
	}
	return nil
}

func (c *Curriculum) eventPtr(id string) *Event {
	ev, _ := c.Event(id)
	return ev
}

// ExamGap: after an exam attempt, no other exam for this long.
const ExamGap = 6 * 24 * time.Hour

// ExamInfo rides on an exam session: what to reach, and what passing gives.
type ExamInfo struct {
	ID     string  `json:"id"`
	Pass   int     `json:"pass"`
	Total  int     `json:"total"`
	Reward *Reward `json:"reward,omitempty"`
}

// Homophones groups graphemes that sound alike: never offered side by side,
// or he would be right and be told no. The one list: le son manquant gets it
// from /api/arcade.
var Homophones = [][]string{
	{"o", "au", "eau"}, {"é", "er"}, {"è", "ê", "ai", "ei"},
	{"an", "am", "en"}, {"in", "ain"}, {"on", "om"}, {"ou", "oo"},
	{"s", "ss", "ce", "ci", "ç", "tion"}, {"z", "s"}, {"c", "k", "qu"}, {"f", "ph", "ff"},
	{"i", "y"}, {"j", "ge", "gi"}, {"eu", "oeu", "œu"},
	{"eil", "eille"}, {"ail", "aille"}, {"ill", "y"},
	{"ette", "esse", "elle", "enne", "è"},
	{"m", "mm"}, {"p", "pp"}, {"t", "tt"}, {"l", "ll"}, {"n", "nn"}, {"r", "rr"},
}

func soundsAlike(a, b string) bool {
	if a == b {
		return true
	}
	for _, g := range Homophones {
		ina, inb := false, false
		for _, x := range g {
			ina = ina || x == a
			inb = inb || x == b
		}
		if ina && inb {
			return true
		}
	}
	return false
}

// weakestFirst orders ids by their spaced-review box, shuffled inside a box.
func weakestFirst(ids []string, st progress.State) {
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	sort.SliceStable(ids, func(i, j int) bool { return st.Items[ids[i]].Box < st.Items[ids[j]].Box })
}

// soundItems: hear a sound, touch the letters that write it.
func (c *Curriculum) soundItems(known []string, n int, st progress.State) []Item {
	var ids []string
	for _, id := range known {
		g, ok := c.Graphemes[id]
		if ok && g.Kind != "muet" && g.Kind != "double" {
			ids = append(ids, SectionSons+":"+id)
		}
	}
	weakestFirst(ids, st)
	var out []Item
	for _, key := range ids {
		if len(out) == n {
			break
		}
		if it, ok := c.soundItem(strings.TrimPrefix(key, SectionSons+":"), known); ok {
			out = append(out, it)
		}
	}
	return out
}

// soundItem is one "hear the sound, touch its letters" question.
func (c *Curriculum) soundItem(id string, known []string) (Item, bool) {
	g, ok := c.Graphemes[id]
	if !ok || g.Kind == "muet" || g.Kind == "double" {
		return Item{}, false
	}
	it := Item{ID: SectionSons + ":" + id, Prompt: g.Display, Audio: speech.URL(g.SayAs, speech.StyleSound)}
	it.Choices = []Choice{{Label: g.Display, Audio: it.Audio, Correct: true}}
	others := append([]string(nil), known...)
	rand.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	for _, o := range others {
		og, ok := c.Graphemes[o]
		if !ok || og.Kind == "muet" || og.Kind == "double" || soundsAlike(o, g.ID) {
			continue
		}
		it.Choices = append(it.Choices, Choice{Label: og.Display, Audio: speech.URL(og.SayAs, speech.StyleSound)})
		if len(it.Choices) == 4 {
			break
		}
	}
	rand.Shuffle(len(it.Choices), func(i, j int) { it.Choices[i], it.Choices[j] = it.Choices[j], it.Choices[i] })
	return it, len(it.Choices) > 1
}

// ExamSections lists the sections in the order they are asked, with their
// title and the instruction read aloud.
var ExamSections = []struct{ ID, Title, Instruction string }{
	{SectionSons, "Les sons", ExamSoundsHint},
	{SectionSyllabes, "Les syllabes", "Écoute la syllabe, puis touche-la."},
	{SectionMots, "Les mots", "Lis le mot tout seul, puis touche la bonne image."},
	{SectionPhrases, "Les phrases", ExamPhraseHint},
}

// Voice lines of the exams.
const (
	ExamSoundsHint = "Écoute le son, puis touche les lettres qui l'écrivent."
	ExamPhraseHint = "Lis la phrase tout seul, puis touche la bonne image."
	ExamRule       = "C'est un examen : une seule réponse par question. Concentre-toi !"
	ExamPassed     = "Bravo ! Tu as réussi l'examen !"
	ExamFailed     = "Pas encore ! Révise ces sons, et l'examen reviendra bientôt."
	ExamArrived    = "Un examen surprise est arrivé ! Trouve le professeur sur la carte !"
	ExamDialog     = "C'est l'examen surprise ! Tu es prêt ?"
	ExamWhere      = "Réussis un examen surprise pour l'attraper !"
	ExamDone       = "Tu as déjà réussi cet examen. Bravo !"
	ExamGate       = "Pour entrer ici, réussis l'examen du professeur ! Il t'attend sur la carte."
)

// BuildExam generates an exam: four strict sections drawn from everything
// learned so far, weakest material first. False while nothing is learned.
func (c *Curriculum) BuildExam(ev *Event, st progress.State, now time.Time) (Session, bool) {
	lastDone := -1
	for i, ep := range c.Episodes {
		if st.Episodes[ep.ID].Completions > 0 {
			lastDone = i
		}
	}
	if lastDone < 0 {
		return Session{}, false
	}
	known := c.KnownGraphemes(lastDone)
	p := c.poolUpTo(lastDone)

	syls := append([]string(nil), p.syllables...)
	keys := make([]string, len(syls))
	for i, s := range syls {
		keys[i] = "syl:" + s
	}
	weakestFirst(keys, st)
	var sylItems []Item
	for _, k := range keys {
		if len(sylItems) == ev.Questions[SectionSyllabes] {
			break
		}
		it := ecouteItem(strings.TrimPrefix(k, "syl:"), p.syllables, 4)
		it.ID = k
		sylItems = append(sylItems, it)
	}

	words := append([]Word(nil), p.words...)
	wkeys := map[string]Word{}
	var wids []string
	for _, w := range words {
		wids = append(wids, "mot:"+w.Text)
		wkeys["mot:"+w.Text] = w
	}
	weakestFirst(wids, st)
	var wordItems []Item
	for _, k := range wids {
		if len(wordItems) == ev.Questions[SectionMots] {
			break
		}
		it := lectureItem(wkeys[k], p.words, 4)
		it.ID = k
		wordItems = append(wordItems, it)
	}

	var sentences []Sentence
	for i := 0; i <= lastDone; i++ {
		sentences = append(sentences, c.Episodes[i].Sentences...)
	}
	rand.Shuffle(len(sentences), func(i, j int) { sentences[i], sentences[j] = sentences[j], sentences[i] })
	phrases := phraseItems(sentences, p.words, 3)
	if n := ev.Questions[SectionPhrases]; len(phrases) > n {
		phrases = phrases[:n]
	}

	items := map[string][]Item{
		SectionSons:     c.soundItems(known, ev.Questions[SectionSons], st),
		SectionSyllabes: sylItems,
		SectionMots:     wordItems,
		SectionPhrases:  phrases,
	}

	s := Session{
		EpisodeID: EventPrefix + ev.ID,
		Title:     ev.Title,
		Legendary: true,
		Lives:     -1,
		UI:        UIAudio(),
		Exam:      &ExamInfo{ID: ev.ID, Pass: ev.PassMark(), Reward: c.reward(ev.Pokemon)},
	}
	s.Activities = append(s.Activities, Activity{
		Kind: "story", Title: ev.Title, Text: ev.Story,
		Audio: speech.URL(ev.Story, speech.StyleNormal),
	})
	s.Activities = append(s.Activities, Activity{
		Kind: "story", Title: "📝 L'examen", Text: ExamRule,
		Audio: speech.URL(ExamRule, speech.StyleNormal),
	})
	for _, sec := range ExamSections {
		its := items[sec.ID]
		if len(its) == 0 {
			continue
		}
		s.Exam.Total += len(its)
		s.Activities = append(s.Activities, Activity{
			Kind: "examen", Title: sec.Title, Instruction: sec.Instruction,
			Text: sec.ID, Items: its,
		})
	}
	return s, s.Exam.Total > 0
}

// ExamScore grades answers: right answers per section and overall percent.
func ExamScore(items map[string]bool) (percent int, sections map[string][2]int) {
	sections = map[string][2]int{}
	right, total := 0, 0
	for id, ok := range items {
		sec, _, _ := strings.Cut(id, ":")
		v := sections[sec]
		v[1]++
		if ok {
			v[0]++
			right++
		}
		sections[sec] = v
		total++
	}
	if total == 0 {
		return 0, sections
	}
	return right * 100 / total, sections
}

// validateEvents checks every event points at real content.
func (c *Curriculum) validateEvents() []error {
	var errs []error
	seen := map[string]bool{}
	for i, ep := range c.Episodes {
		if ep.RequiresExam == "" {
			continue
		}
		ev, ok := c.Event(ep.RequiresExam)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s: requires_exam %q absent de events.json", ep.ID, ep.RequiresExam))
		case c.Index(ev.After) >= i:
			errs = append(errs, fmt.Errorf("%s: l'examen %q n'apparaît qu'après %s — il ne peut pas en ouvrir la porte", ep.ID, ev.ID, ev.After))
		}
	}
	for _, ev := range c.Events {
		switch {
		case ev.ID == "" || seen[ev.ID]:
			errs = append(errs, fmt.Errorf("events.json: id %q vide ou en double", ev.ID))
		case c.Index(ev.After) < 0:
			errs = append(errs, fmt.Errorf("events.json: %s: épisode %q inconnu", ev.ID, ev.After))
		case ev.Total() == 0:
			errs = append(errs, fmt.Errorf("events.json: %s: aucune question", ev.ID))
		case ev.Story == "" || ev.Title == "":
			errs = append(errs, fmt.Errorf("events.json: %s: titre ou histoire manquant", ev.ID))
		}
		if _, ok := c.Pokemon[ev.Pokemon]; !ok {
			errs = append(errs, fmt.Errorf("events.json: %s: pokémon #%d absent de pokemon.json", ev.ID, ev.Pokemon))
		}
		for k := range ev.Questions {
			ok := false
			for _, sec := range ExamSections {
				ok = ok || sec.ID == k
			}
			if !ok {
				errs = append(errs, fmt.Errorf("events.json: %s: section %q inconnue", ev.ID, k))
			}
		}
		seen[ev.ID] = true
	}
	return errs
}

// ItemLine is what a review item shows and says: "son:ch" → « ch » and its
// sound, "mot:Pikachu" → the word and its clip.
func (c *Curriculum) ItemLine(id string) (label, audio string) {
	kind, text, _ := strings.Cut(id, ":")
	switch kind {
	case SectionSons:
		if g, ok := c.Graphemes[text]; ok {
			return g.Display, speech.URL(g.SayAs, speech.StyleSound)
		}
	case SectionSyllabes:
		return text, speech.URL(SpokenSyllable(text), speech.StyleSyllable)
	case SectionMots:
		for _, ep := range c.Episodes {
			for _, w := range ep.Words {
				if w.Text == text {
					return w.Text, speech.URL(w.Spoken(), speech.StyleWord)
				}
			}
		}
	case SectionPhrases:
		for _, ep := range c.Episodes {
			for _, s := range ep.Sentences {
				if s.Text == text {
					return s.Text, speech.URL(s.Spoken(), speech.StyleWord)
				}
			}
		}
	}
	return text, ""
}
