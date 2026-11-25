package curriculum

import (
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

func loadCur(t *testing.T) *Curriculum {
	t.Helper()
	c, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSameSoundAndMinimalPairs(t *testing.T) {
	loadCur(t)
	for _, p := range [][2]string{{"san", "sen"}, {"ca", "ka"}, {"son", "çon"}, {"fo", "pho"}, {"ta", "tta"}} {
		if !sameSound(p[0], p[1]) {
			t.Errorf("%s/%s should sound the same", p[0], p[1])
		}
	}
	if sameSound("ba", "da") {
		t.Error("ba/da do not sound the same")
	}
	if !MinimalPair("ba", "da") || !MinimalPair("fa", "va") || !MinimalPair("cha", "ja") {
		t.Error("ba/da, fa/va, cha/ja are minimal pairs")
	}
	if MinimalPair("ba", "mo") || MinimalPair("ba", "ba") || MinimalPair("ba", "ma") {
		t.Error("ba/mo, ba/ba, ba/ma are not minimal pairs")
	}
}

func TestDistractorsNeverSoundTheSame(t *testing.T) {
	loadCur(t)
	all := []string{"san", "sen", "son", "çon", "ca", "ka", "ma", "mi", "lu"}
	for _, target := range all {
		for i := 0; i < 20; i++ {
			for _, d := range pickDistractors(target, all, 3) {
				if sameSound(d, target) {
					t.Fatalf("%s offered with %s", target, d)
				}
			}
		}
	}
}

func TestDictationOnlyOnAFairTray(t *testing.T) {
	loadCur(t)
	rounds := []Item{{Prompt: "Abo", Tray: []string{"A", "bo", "mi"}}, {Prompt: "Rattata", Tray: []string{"Ra", "tta", "ta"}}}
	dictateLast(rounds)
	if last := rounds[len(rounds)-1]; !last.Dictee || last.Prompt != "Abo" {
		t.Fatalf("the fair word should be dictated last: %+v", rounds)
	}
	for _, r := range rounds[:1] {
		if r.Dictee {
			t.Fatal("Rattata (ta/tta) dictated")
		}
	}
}

func TestOneExamAWeek(t *testing.T) {
	c := loadCur(t)
	now := time.Now()
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	for _, ep := range c.Episodes {
		st.Episodes[ep.ID] = progress.EpisodeStat{Completions: 1}
	}
	st.Exams = map[string]progress.ExamRecord{"petit": {Attempts: 1, LastAt: now.Add(-48 * time.Hour)}}
	for d := 0; d < 3; d++ {
		if ev := c.EventToday(st, now.Add(time.Duration(d)*24*time.Hour)); ev != nil {
			t.Fatalf("day %d: %s drawn two days after an exam", d, ev.ID)
		}
	}
	st.Exams["petit"] = progress.ExamRecord{Attempts: 1, LastAt: now}
	if ev := c.EventToday(st, now); ev == nil || ev.ID != "petit" {
		t.Fatalf("a failed exam must stay for a retry the same day, got %v", ev)
	}
}

func TestWordQuizPrefersLookAlikes(t *testing.T) {
	loadCur(t)
	target := Word{Text: "Pikachu", Emoji: "a", Syllables: []string{"Pi", "ka", "chu"}}
	pool := []Word{target,
		{Text: "Pipo", Emoji: "b", Syllables: []string{"Pi", "po"}},
		{Text: "Zéro", Emoji: "c", Syllables: []string{"Zé", "ro"}},
		{Text: "Lune", Emoji: "d", Syllables: []string{"Lu", "ne"}},
		{Text: "Roc", Emoji: "e", Syllables: []string{"Roc"}}}
	for i := 0; i < 20; i++ {
		it := lectureItem(target, pool, 2)
		for _, ch := range it.Choices {
			if !ch.Correct && ch.Label != "Pipo" {
				t.Fatalf("distractor %s instead of the look-alike Pipo", ch.Label)
			}
		}
	}
}

// The right picture is the subject's: a wrong one must start like it, or
// the first letter of the sentence answers the question.
func TestSentenceQuizPrefersLookAlikes(t *testing.T) {
	loadCur(t)
	sentences := []Sentence{{Text: "Abo utilise Morsure.", Emoji: "a"}, {Text: "Zéro dort.", Emoji: "z"}}
	words := []Word{{Text: "Abra", Emoji: "b"}, {Text: "Lune", Emoji: "d"}, {Text: "Morsure", Emoji: "m"}}
	for i := 0; i < 20; i++ {
		items := phraseItems(sentences, words, 2)
		for _, ch := range items[0].Choices {
			if !ch.Correct && ch.Emoji != "b" {
				t.Fatalf("distractor %q instead of the look-alike Abra", ch.Emoji)
			}
		}
	}
}

func TestFirstExamWaitsEveryDay(t *testing.T) {
	c := loadCur(t)
	st := progress.State{Episodes: map[string]progress.EpisodeStat{}}
	for _, ep := range c.Episodes {
		st.Episodes[ep.ID] = progress.EpisodeStat{Completions: 1}
		if ep.ID == "ep03b" { // the first badge
			break
		}
	}
	for d := 0; d < 10; d++ {
		if ev := c.EventToday(st, time.Now().Add(time.Duration(d)*24*time.Hour)); ev == nil || ev.ID != "petit" {
			t.Fatalf("day %d: the first exam, never tried, should wait on the map (got %v)", d, ev)
		}
	}
}

func TestStoryShowsCameoAndPeek(t *testing.T) {
	c := loadCur(t)
	ep, _ := c.Episode("ep37")
	s := c.BuildSession(ep, progress.State{Episodes: map[string]progress.EpisodeStat{}}, time.Now())
	story := s.Activities[0]
	if story.Kind != "story" || len(story.Cameo) == 0 || story.Peek == nil || story.Peek.Line == "" {
		t.Fatalf("story screen: %+v", story)
	}
}
