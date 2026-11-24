package curriculum

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// Choice is one tappable answer.
type Choice struct {
	Label   string `json:"label,omitempty"`
	Emoji   string `json:"emoji,omitempty"`
	Sprite  string `json:"sprite,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Audio   string `json:"audio,omitempty"`
	Correct bool   `json:"correct"`
}

// Item is one question. ID is the spaced-review key reported back by the client.
type Item struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt,omitempty"`
	Audio         string   `json:"audio,omitempty"`
	Emoji         string   `json:"emoji,omitempty"`
	Sprite        string   `json:"sprite,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Syllables     []string `json:"syllables,omitempty"`
	SyllableAudio []string `json:"syllable_audio,omitempty"`
	Choices       []Choice `json:"choices,omitempty"`
	Tray          []string `json:"tray,omitempty"`
	// Dictee hides the battle word: he hears it and builds it from syllables.
	Dictee bool `json:"dictee,omitempty"`
}

// GraphemeCard is the "new sound of the day" screen.
type GraphemeCard struct {
	ID            string `json:"id"`
	Display       string `json:"display"`
	Kind          string `json:"kind"`
	SoundAudio    string `json:"sound_audio"`
	Mnemo         string `json:"mnemo,omitempty"`
	MnemoAudio    string `json:"mnemo_audio,omitempty"`
	Example       string `json:"example,omitempty"`
	ExampleEmoji  string `json:"example_emoji,omitempty"`
	ExampleSprite string `json:"example_sprite,omitempty"`
	ExampleIcon   string `json:"example_icon,omitempty"`
	ExampleAudio  string `json:"example_audio,omitempty"`
}

// Pair is a consonant + vowel to slam together in the blending game.
type Pair struct {
	Left       string `json:"left"`
	Right      string `json:"right"`
	Syllable   string `json:"syllable"`
	LeftAudio  string `json:"left_audio"`
	RightAudio string `json:"right_audio"`
	Audio      string `json:"audio"`
}

// Fight is the end-of-episode boss.
type Fight struct {
	Pokemon    int      `json:"pokemon"`
	Name       string   `json:"name"`
	Syllables  []string `json:"syllables"`
	HP         int      `json:"hp"`
	Legendary  bool     `json:"legendary,omitempty"`
	Taunt      string   `json:"taunt"`
	TauntAudio string   `json:"taunt_audio"`
	Rounds     []Item   `json:"rounds"`
}

// BadgeCard is the badge ceremony after an arena victory.
type BadgeCard struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Audio string `json:"audio"`
}

// Reward is the caught Pokémon. Shiny marks the rare recolored variant rolled
// on a replay — the games' own trick for making old ground worth walking again.
type Reward struct {
	Pokemon   int      `json:"pokemon"`
	Name      string   `json:"name"`
	Syllables []string `json:"syllables"`
	// SyllableAudio makes each chunk of the name tappable in the voice of the
	// track; without it a tap fell back to the browser's own voice.
	SyllableAudio []string `json:"syllable_audio"`
	NameAudio     string   `json:"name_audio"`
	Types         []string `json:"types"`
	Dex           string   `json:"dex"`
	DexAudio      string   `json:"dex_audio"`
	Shiny         bool     `json:"shiny,omitempty"`
}

// Activity is one screen of a session.
type Activity struct {
	Kind        string         `json:"kind"`
	Title       string         `json:"title,omitempty"`
	Instruction string         `json:"instruction,omitempty"`
	Text        string         `json:"text,omitempty"`
	Audio       string         `json:"audio,omitempty"`
	Grapheme    *GraphemeCard  `json:"grapheme,omitempty"`
	Graphemes   []GraphemeCard `json:"graphemes,omitempty"`
	Pairs       []Pair         `json:"pairs,omitempty"`
	Items       []Item         `json:"items,omitempty"`
	Fight       *Fight         `json:"fight,omitempty"`
	Reward      *Reward        `json:"reward,omitempty"`
	Badge       *BadgeCard     `json:"badge,omitempty"`
	// Story screens: the Pokémon the story names, and a Tower Pokémon
	// peeking in (one he has not caught yet, with where it waits).
	Cameo []Face `json:"cameo,omitempty"`
	Peek  *Face  `json:"peek,omitempty"`
}

// Face is a Pokémon shown on a story screen: its picture, its name, and,
// for a peek, the spoken line saying where it waits.
type Face struct {
	Pokemon   int    `json:"pokemon"`
	Name      string `json:"name"`
	NameAudio string `json:"name_audio"`
	Line      string `json:"line,omitempty"`
	LineAudio string `json:"line_audio,omitempty"`
}

func (c *Curriculum) face(id int) Face {
	p := c.Pokemon[id]
	return Face{Pokemon: id, Name: p.Name, NameAudio: speech.URL(p.Spoken(), speech.StyleWord)}
}

// TowerPeek is a Tower Pokémon he has not caught yet, drawn for the day:
// the new Pokémon show up in every lesson long before their floor.
func (c *Curriculum) TowerPeek(st progress.State, now time.Time) *Face {
	caught := map[int]bool{}
	for _, id := range st.Pokedex {
		caught[id] = true
	}
	var left []int
	for _, id := range c.TowerLadder {
		if !caught[id] {
			left = append(left, id)
		}
	}
	if len(left) == 0 {
		return nil
	}
	day := int(now.Sub(eventEpoch).Hours() / 24)
	f := c.face(left[(day*7+len(st.Pokedex))%len(left)])
	f.Line = c.Where(f.Pokemon)
	f.LineAudio = speech.URL(f.Line, speech.StyleNormal)
	return &f
}

// TowerNext is the Pokémon the next Tower floor (or the first paying one
// above it) awards, 0 when the ladder is done: shown on the map's 🗼.
func (c *Curriculum) TowerNext(cleared int) (pokemon, floor int) {
	for f := cleared + 1; f <= cleared+2; f++ {
		if id := c.TowerRewardFor(f); id != 0 {
			return id, f
		}
	}
	return 0, 0
}

// Session is the generated playable episode.
type Session struct {
	EpisodeID     string     `json:"episode_id"`
	Route         int        `json:"route"`
	Title         string     `json:"title"`
	RewardPokemon int        `json:"reward_pokemon"`
	Legendary     bool       `json:"legendary,omitempty"`
	Tower         *TowerInfo `json:"tower,omitempty"`
	Exam          *ExamInfo  `json:"exam,omitempty"`
	// Trainer is set on a map trainer's battle (see BuildTrainer).
	Trainer *TrainerInfo `json:"trainer,omitempty"`
	// Lives is the battle heart count from the parental settings: -1 means
	// unlimited, otherwise each wrong tap in a fight costs one and running out
	// loses the battle.
	Lives int `json:"lives"`
	// Speed is his usual answer time per item kind (ms), once known: the
	// client's "fast" is relative to it.
	Speed      map[string]int    `json:"speed,omitempty"`
	UI         map[string]string `json:"ui"`
	Activities []Activity        `json:"activities"`
}

// UIAudio returns the recorded chrome phrases so praise and feedback come out
// in the same voice as the lesson instead of the browser's default. Exported
// because the state endpoint ships it too: the home screen and the Pokédex
// speak ("Termine la route précédente !") outside any session.
func UIAudio() map[string]string {
	out := map[string]string{}
	for _, p := range UIPhrases {
		out[p] = speech.URL(p, speech.StyleNormal)
	}
	return out
}

const (
	maxEcoute  = 6
	maxLecture = 6
	maxFusion  = 6
	maxReview  = 5
	// Above this many new sounds, the discovery cards collapse into a gallery.
	maxSoloCards = 3
	// A legendary arena is a longer fight and a wider review than a normal route.
	maxLegendaryPhrases = 3
	maxLegendaryRounds  = 6
)

// difficulty scales the exercises with progression: a child at route 20 is not
// the same reader as on day one, and questions that never get harder read as
// the same game replayed. Tiers follow the highest route reached (README
// « Difficulté progressive »), so inserting an episode never shifts the ramp.
type difficulty struct {
	choices int // answer tiles per question (correct + distractors)
	tray    int // distractor syllables added to the battle tray
	rounds  int // words per normal fight
}

func (c *Curriculum) difficultyAt(idx int) difficulty {
	route := 0
	for i := 0; i <= idx && i < len(c.Episodes); i++ {
		route = max(route, c.Episodes[i].Route)
	}
	switch {
	case route >= AcademyRoute:
		return difficulty{choices: 4, tray: 3, rounds: 6}
	case route >= 19:
		return difficulty{choices: 4, tray: 2, rounds: 5}
	case route >= 10:
		return difficulty{choices: 4, tray: 1, rounds: 4}
	default:
		return difficulty{choices: 3, tray: 1, rounds: 4}
	}
}

// AcademyRoute is the first route of the Académie Pokémon, the hardest tier.
const AcademyRoute = 28

type pool struct {
	syllables []string
	words     []Word
	byWord    map[string]Word
}

// poolUpTo gathers every syllable and word taught up to episode idx, used both
// for spaced review and for picking plausible wrong answers.
func (c *Curriculum) poolUpTo(idx int) pool {
	p := pool{byWord: map[string]Word{}}
	seen := map[string]bool{}
	for i := 0; i <= idx && i < len(c.Episodes); i++ {
		for _, s := range c.Episodes[i].Syllables {
			if !seen["s"+s] {
				seen["s"+s] = true
				p.syllables = append(p.syllables, s)
			}
		}
		for _, w := range c.Episodes[i].Words {
			if !seen["w"+w.Text] {
				seen["w"+w.Text] = true
				p.words = append(p.words, w)
				p.byWord[w.Text] = w
			}
		}
	}
	return p
}

// BuildSession turns an episode declaration into an ordered activity list,
// injecting spaced review of older material at the front.
func (c *Curriculum) BuildSession(ep *Episode, st progress.State, now time.Time) Session {
	idx := c.Index(ep.ID)
	known := c.KnownGraphemes(idx)
	p := c.poolUpTo(idx)
	diff := c.difficultyAt(idx)

	// Shuffle the episode's own material so a replay is not the identical
	// screen four days in a row — an episode declares more syllables and words
	// than a single session shows.
	syllables := append([]string(nil), ep.Syllables...)
	words := append([]Word(nil), ep.Words...)
	sentences := append([]Sentence(nil), ep.Sentences...)

	// A legendary arena declares no material of its own: it pulls from everything
	// learned so far, which is what makes it a real milestone test rather than
	// one more route.
	if ep.Legendary {
		syllables = append(syllables, p.syllables...)
		words = append(words, p.words...)
		for i := 0; i < idx && i < len(c.Episodes); i++ {
			sentences = append(sentences, c.Episodes[i].Sentences...)
		}
	}

	rand.Shuffle(len(syllables), func(i, j int) { syllables[i], syllables[j] = syllables[j], syllables[i] })
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	rand.Shuffle(len(sentences), func(i, j int) { sentences[i], sentences[j] = sentences[j], sentences[i] })
	if ep.Legendary && len(sentences) > maxLegendaryPhrases {
		sentences = sentences[:maxLegendaryPhrases]
	}

	s := Session{
		EpisodeID:     ep.ID,
		Route:         ep.Route,
		Title:         ep.Title,
		RewardPokemon: ep.RewardPokemon,
		Legendary:     ep.Legendary,
		Lives:         st.Settings.BattleLives,
		UI:            UIAudio(),
	}

	if ep.Story != "" {
		story := Activity{
			Kind:  "story",
			Title: ep.Title,
			Text:  ep.Story,
			Audio: speech.URL(ep.Story, speech.StyleNormal),
			Peek:  c.TowerPeek(st, now),
		}
		for _, id := range ep.Cameo {
			story.Cameo = append(story.Cameo, c.face(id))
		}
		s.Activities = append(s.Activities, story)
	}

	if rev := c.reviewActivity(idx, p, st, now, diff); rev != nil {
		s.Activities = append(s.Activities, *rev)
	}

	cards := make([]GraphemeCard, 0, len(ep.NewGraphemes))
	for _, id := range ep.NewGraphemes {
		g, ok := c.Graphemes[id]
		if !ok {
			continue
		}
		exPic := g.Pic()
		card := GraphemeCard{
			ID:            g.ID,
			Display:       g.Display,
			Kind:          g.Kind,
			SoundAudio:    speech.URL(g.SayAs, speech.StyleSound),
			Mnemo:         g.Mnemo,
			Example:       g.Example,
			ExampleEmoji:  exPic.Emoji,
			ExampleSprite: exPic.Sprite,
			ExampleIcon:   exPic.Icon,
		}
		if g.Mnemo != "" {
			card.MnemoAudio = speech.URL(g.Mnemo, speech.StyleNormal)
		}
		if g.Example != "" {
			card.ExampleAudio = speech.URL(g.Example, speech.StyleWord)
		}
		cards = append(cards, card)
	}

	// One full card per sound teaches well, but a review episode introducing ten
	// sounds would become ten "next" taps before anything fun happens. Past
	// three, collapse them into a single gallery he sweeps through.
	if len(cards) > maxSoloCards {
		s.Activities = append(s.Activities, Activity{
			Kind:        "galerie",
			Title:       "Les sons du jour",
			Instruction: "Touche chaque lettre pour entendre son son.",
			Graphemes:   cards,
		})
	} else {
		for i := range cards {
			s.Activities = append(s.Activities, Activity{
				Kind:        "decouverte",
				Title:       "Le son du jour",
				Instruction: "Touche la lettre pour entendre son son.",
				Grapheme:    &cards[i],
			})
		}
	}

	if pairs := c.fusionPairs(syllables, known); len(pairs) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "fusion",
			Title:       "Colle les sons",
			Instruction: "Touche la première lettre, puis la deuxième. Écoute la syllabe !",
			Pairs:       pairs,
		})
	}

	if items := ecouteItems(syllables, p.syllables, maxEcoute, diff.choices); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "ecoute",
			Title:       "Écoute et trouve",
			Instruction: "Écoute la syllabe, puis touche-la.",
			Items:       items,
		})
	}

	if items := lectureItems(words, p.words, maxLecture, diff.choices); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "lecture",
			Title:       "Lis le mot",
			Instruction: "Lis le mot tout seul, puis touche la bonne image.",
			Items:       items,
		})
	}

	if items := phraseItems(sentences, p.words, diff.choices); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "phrase",
			Title:       "Lis la phrase",
			Instruction: "Lis la phrase à voix haute. Si un mot est difficile, touche-le !",
			Items:       items,
		})
	}

	if f := c.fight(ep, words, p.syllables, diff); f != nil {
		s.Activities = append(s.Activities, Activity{
			Kind:        "combat",
			Title:       "Combat !",
			Instruction: "Remets les syllabes dans l'ordre pour lancer l'attaque.",
			Fight:       f,
		})
	}

	if r := c.reward(ep.RewardPokemon); r != nil {
		// A replayed episode has a 1-in-6 chance of a shiny catch: the surprise
		// that keeps a finished route worth reading again.
		if st.Episodes[ep.ID].Completions > 0 && rand.IntN(6) == 0 {
			r.Shiny = true
		}
		s.Activities = append(s.Activities, Activity{
			Kind:   "recompense",
			Title:  "Tu l'as attrapé !",
			Reward: r,
		})
	}

	if ep.Badge != nil {
		s.Activities = append(s.Activities, Activity{
			Kind:  "badge",
			Title: ep.Badge.Name,
			Badge: &BadgeCard{
				ID:    ep.Badge.ID,
				Name:  ep.Badge.Name,
				Audio: speech.URL(badgePhrase(*ep.Badge), speech.StyleNormal),
			},
		})
	}

	return s
}

// badgePhrase is the sentence spoken at the badge ceremony; kept in one place so
// SpeechTexts records the exact same string BuildSession asks for.
func badgePhrase(b BadgeDecl) string {
	return "Tu as gagné le " + b.Name + " !"
}

// cand is one warm-up candidate; lower score = more urgent.
type cand struct {
	id    string
	item  Item
	score int
}

// maxReviewSounds: sound questions in every warm-up.
const maxReviewSounds = 2

// weakSounds picks the due sounds of earlier routes, weakest box first.
func (c *Curriculum) weakSounds(idx int, st progress.State, now time.Time) []cand {
	known := c.KnownGraphemes(idx)
	var ids []string
	for _, g := range c.KnownGraphemes(idx - 1) {
		id := SectionSons + ":" + g
		if rec, tracked := st.Items[id]; !tracked || rec.Due(now) {
			ids = append(ids, id)
		}
	}
	weakestFirst(ids, st)
	var out []cand
	for _, id := range ids {
		if len(out) == maxReviewSounds {
			break
		}
		if it, ok := c.soundItem(strings.TrimPrefix(id, SectionSons+":"), known); ok {
			out = append(out, cand{id: id, item: it, score: st.Items[id].Box})
		}
	}
	return out
}

// reviewActivity picks the weakest due material from earlier episodes. It mixes
// syllables and words in one screen so the warm-up stays short.
func (c *Curriculum) reviewActivity(idx int, p pool, st progress.State, now time.Time, diff difficulty) *Activity {
	if idx <= 0 {
		return nil
	}
	prev := c.poolUpTo(idx - 1)

	var cands []cand

	for _, syl := range prev.syllables {
		id := "syl:" + syl
		rec, tracked := st.Items[id]
		if tracked && !rec.Due(now) {
			continue
		}
		it := ecouteItem(syl, p.syllables, diff.choices)
		it.ID = id
		cands = append(cands, cand{id: id, item: it, score: rec.Box})
	}
	for _, w := range prev.words {
		id := "mot:" + w.Text
		rec, tracked := st.Items[id]
		if tracked && !rec.Due(now) {
			continue
		}
		it := lectureItem(w, p.words, diff.choices)
		it.ID = id
		cands = append(cands, cand{id: id, item: it, score: rec.Box})
	}
	if len(cands) == 0 && idx <= 0 {
		return nil
	}

	rand.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	// Stable sort on box keeps the shuffle as a tie-breaker inside a box.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j].score < cands[j-1].score; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	if len(cands) > maxReview-maxReviewSounds {
		cands = cands[:maxReview-maxReviewSounds]
	}
	// The grapheme→sound link is the core of reading: every warm-up asks the
	// weakest sounds of earlier routes too (hear it, touch its letters).
	cands = append(cands, c.weakSounds(idx, st, now)...)
	rand.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })

	if len(cands) == 0 {
		return nil
	}
	act := Activity{
		Kind:        "revision",
		Title:       "Échauffement",
		Instruction: "On révise avant de partir !",
	}
	for _, c := range cands {
		act.Items = append(act.Items, c.item)
	}
	return &act
}

// fusionPairs keeps only two-grapheme syllables: those are the ones where the
// "slam two sounds together" gesture teaches something.
//
// The audio must be resolved through the grapheme's SayAs, not the grapheme id:
// the recorded file for "t" is the syllable series "ta, te, ti, to, tu", and
// asking for /audio/t.m4a would 404 into the browser reading out the letter
// *name* ("té") — which is exactly the confusion this whole game exists to avoid.
func (c *Curriculum) fusionPairs(syllables []string, known []string) []Pair {
	sound := func(id string) (display, audio string) {
		g, ok := c.Graphemes[id]
		if !ok {
			return id, speech.URL(id, speech.StyleSound)
		}
		return g.Display, speech.URL(g.SayAs, speech.StyleSound)
	}

	var out []Pair
	for _, syl := range syllables {
		parts, err := Segment(syl, known)
		if err != nil || len(parts) != 2 {
			continue
		}
		leftDisplay, leftAudio := sound(parts[0])
		rightDisplay, rightAudio := sound(parts[1])
		out = append(out, Pair{
			Left:       leftDisplay,
			Right:      rightDisplay,
			Syllable:   syl,
			LeftAudio:  leftAudio,
			RightAudio: rightAudio,
			Audio:      speech.URL(SpokenSyllable(syl), speech.StyleSyllable),
		})
		if len(out) == maxFusion {
			break
		}
	}
	return out
}

func ecouteItems(target, allSyllables []string, limit, nChoices int) []Item {
	var out []Item
	for _, syl := range target {
		out = append(out, ecouteItem(syl, allSyllables, nChoices))
		if len(out) == limit {
			break
		}
	}
	return out
}

// ecouteItem plays a syllable and asks the child to point at it. Distractors
// are minimal pairs when possible ("ma" vs "mi"), which is where the real
// discrimination work happens.
func ecouteItem(syl string, allSyllables []string, nChoices int) Item {
	it := Item{
		ID:     "syl:" + syl,
		Prompt: syl,
		Audio:  speech.URL(SpokenSyllable(syl), speech.StyleSyllable),
	}
	distract := pickDistractors(syl, allSyllables, nChoices-1)
	labels := append([]string{syl}, distract...)
	rand.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
	for _, l := range labels {
		it.Choices = append(it.Choices, Choice{
			Label:   l,
			Audio:   speech.URL(SpokenSyllable(l), speech.StyleSyllable),
			Correct: l == syl,
		})
	}
	return it
}

func pickDistractors(target string, all []string, n int) []string {
	var near, far []string
	for _, s := range all {
		// A syllable that sounds the same is a second right answer, not a trap.
		if s == target || sameSound(s, target) {
			continue
		}
		if len(s) == len(target) && differsByOne(s, target) {
			near = append(near, s)
		} else {
			far = append(far, s)
		}
	}
	rand.Shuffle(len(near), func(i, j int) { near[i], near[j] = near[j], near[i] })
	rand.Shuffle(len(far), func(i, j int) { far[i], far[j] = far[j], far[i] })

	out := append(near, far...)
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func differsByOne(a, b string) bool {
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	return diff == 1
}

func lectureItems(target, allWords []Word, limit, nChoices int) []Item {
	var out []Item
	for _, w := range target {
		out = append(out, lectureItem(w, allWords, nChoices))
		if len(out) == limit {
			break
		}
	}
	return out
}

// lectureItem shows a written word and a few pictures. The audio is attached
// but the frontend only plays it after an answer: he must decode first.
func lectureItem(w Word, allWords []Word, nChoices int) Item {
	pic := w.Pic()
	it := Item{
		ID:        "mot:" + w.Text,
		Prompt:    w.Text,
		Emoji:     pic.Emoji,
		Sprite:    pic.Sprite,
		Icon:      pic.Icon,
		Syllables: w.Syllables,
		Audio:     speech.URL(w.Spoken(), speech.StyleWord),
	}
	// Per-syllable audio makes each coloured chunk of the word tappable, so he
	// can break a word he stalls on into the pieces he already knows.
	for _, syl := range w.Syllables {
		it.SyllableAudio = append(it.SyllableAudio, speech.URL(SpokenSyllable(syl), speech.StyleSyllable))
	}
	var others []Word
	for _, o := range allWords {
		if o.Text != w.Text && o.Pic().Key() != pic.Key() {
			others = append(others, o)
		}
	}
	rand.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	// Look-alike words first (same first syllable, then same first letter):
	// otherwise the first letter alone picks the image and he stops decoding.
	alike := func(o Word) int {
		switch {
		case len(o.Syllables) > 0 && len(w.Syllables) > 0 && strings.EqualFold(o.Syllables[0], w.Syllables[0]):
			return 0
		case strings.EqualFold(firstRune(o.Text), firstRune(w.Text)):
			return 1
		}
		return 2
	}
	sortStable(others, func(a, b Word) bool { return alike(a) < alike(b) })

	// Distractors must also look different from each other, or the child faces
	// two identical wrong tiles and the "wrong" answer becomes a coin flip.
	choices := []Choice{{Emoji: pic.Emoji, Sprite: pic.Sprite, Icon: pic.Icon, Label: w.Text, Correct: true}}
	used := map[string]bool{pic.Key(): true}
	for _, o := range others {
		op := o.Pic()
		if used[op.Key()] {
			continue
		}
		used[op.Key()] = true
		choices = append(choices, Choice{Emoji: op.Emoji, Sprite: op.Sprite, Icon: op.Icon, Label: o.Text})
		if len(choices) == nChoices {
			break
		}
	}
	rand.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
	it.Choices = choices
	return it
}

// SentenceWords is sentenceWords for the mini-games.
func SentenceWords(text string) []string { return sentenceWords(text) }

// SentencesUpTo lists the sentences of episodes 0..idx, in course order.
func (c *Curriculum) SentencesUpTo(idx int) []Sentence {
	var out []Sentence
	for i := 0; i <= idx && i < len(c.Episodes); i++ {
		out = append(out, c.Episodes[i].Sentences...)
	}
	return out
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// sentenceWords splits a sentence into its word tiles, punctuation stripped:
// the tiles the child reads and reorders in the sentence builder.
func sentenceWords(text string) []string {
	var out []string
	for _, tok := range strings.Fields(text) {
		if w := strings.Trim(tok, ".,!?"); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// phraseItems builds "Lis la phrase": the sentence to read aloud (each word
// tappable for help), a karaoke check that reads it back word by word, then
// the comprehension question — which picture goes with the sentence? The
// wrong pictures never show a word of the sentence itself, or "Otaria utilise
// Morsure." would have two right answers (Otaria and the Morsure icon).
func phraseItems(sentences []Sentence, words []Word, nChoices int) []Item {
	var out []Item
	nChoices = min(nChoices, 3)
	for i, s := range sentences {
		toks := sentenceWords(s.Text)
		if len(toks) < 2 {
			continue
		}
		pic := s.Pic()
		it := Item{
			ID:        "phrase:" + s.Text,
			Prompt:    s.Text,
			Emoji:     pic.Emoji,
			Sprite:    pic.Sprite,
			Icon:      pic.Icon,
			Audio:     speech.URL(s.Spoken(), speech.StyleWord),
			Syllables: toks,
		}
		inSentence := map[string]bool{}
		for _, w := range toks {
			it.SyllableAudio = append(it.SyllableAudio, speech.URL(w, speech.StyleWord))
			inSentence[strings.ToLower(w)] = true
		}

		var others []Picture
		for j, o := range sentences {
			if j != i {
				others = append(others, o.Pic())
			}
		}
		for _, w := range words {
			if !inSentence[strings.ToLower(w.Text)] {
				others = append(others, w.Pic())
			}
		}
		rand.Shuffle(len(others), func(a, b int) { others[a], others[b] = others[b], others[a] })
		used := map[string]bool{pic.Key(): true}
		for _, w := range words {
			if inSentence[strings.ToLower(w.Text)] {
				used[w.Pic().Key()] = true
			}
		}
		choices := []Choice{{Emoji: pic.Emoji, Sprite: pic.Sprite, Icon: pic.Icon, Correct: true}}
		for _, op := range others {
			if op.Empty() || used[op.Key()] {
				continue
			}
			used[op.Key()] = true
			choices = append(choices, Choice{Emoji: op.Emoji, Sprite: op.Sprite, Icon: op.Icon})
			if len(choices) == nChoices {
				break
			}
		}
		if len(choices) < 2 {
			continue
		}
		rand.Shuffle(len(choices), func(a, b int) { choices[a], choices[b] = choices[b], choices[a] })
		it.Choices = choices
		out = append(out, it)
	}
	return out
}

// fight builds the boss rounds: rebuild each attack word from shuffled
// syllables. Ordering syllables is segmentation practice disguised as a combo.
func (c *Curriculum) fight(ep *Episode, words []Word, allSyllables []string, diff difficulty) *Fight {
	boss, ok := c.Pokemon[ep.Boss.Pokemon]
	if !ok {
		return nil
	}
	hp := ep.Boss.HP
	if hp == 0 {
		hp = 3
	}
	rounds := diff.rounds
	if ep.Legendary {
		rounds = maxLegendaryRounds
	}

	f := &Fight{
		Pokemon:    boss.ID,
		Name:       boss.Name,
		Syllables:  boss.Syllables,
		HP:         hp,
		Legendary:  ep.Legendary,
		Taunt:      ep.Boss.Taunt,
		TauntAudio: speech.URL(ep.Boss.Taunt, speech.StyleNormal),
	}

	for _, w := range words {
		if len(w.Syllables) < 2 {
			continue
		}
		tray := append([]string(nil), w.Syllables...)
		// A distractor equal to one of the word's own syllables is a free extra
		// tile, not a trap — keep only syllables foreign to the word.
		inWord := map[string]bool{}
		for _, s := range w.Syllables {
			inWord[strings.ToLower(s)] = true
		}
		var candidates []string
		for _, s := range allSyllables {
			if !inWord[strings.ToLower(s)] {
				candidates = append(candidates, s)
			}
		}
		for _, d := range pickDistractors(w.Syllables[0], candidates, diff.tray) {
			tray = append(tray, d)
		}
		rand.Shuffle(len(tray), func(i, j int) { tray[i], tray[j] = tray[j], tray[i] })

		f.Rounds = append(f.Rounds, Item{
			ID:        "mot:" + w.Text,
			Prompt:    w.Text,
			Emoji:     w.Pic().Emoji,
			Sprite:    w.Pic().Sprite,
			Icon:      w.Pic().Icon,
			Syllables: w.Syllables,
			Audio:     speech.URL(w.Spoken(), speech.StyleWord),
			Tray:      tray,
		})
		if len(f.Rounds) == rounds {
			break
		}
	}
	if len(f.Rounds) == 0 {
		return nil
	}
	if ep.Legendary {
		dictateLast(f.Rounds)
	}
	f.HP = min(f.HP, len(f.Rounds))
	return f
}

// DicteeFrom: Tower floors from this one end on a dictation round.
const DicteeFrom = 10

// dictateLast turns the last round fair for dictation into one: every tile
// of its tray must sound different, or hearing the word could not tell
// "ta" from "tta". It is moved to the end, the Gigamax finale.
func dictateLast(rounds []Item) {
	for i := len(rounds) - 1; i >= 0; i-- {
		if !fairDictee(rounds[i].Tray) {
			continue
		}
		it := rounds[i]
		it.Dictee = true
		copy(rounds[i:], rounds[i+1:])
		rounds[len(rounds)-1] = it
		return
	}
}

func fairDictee(tray []string) bool {
	for i := range tray {
		for j := i + 1; j < len(tray); j++ {
			if sameSound(tray[i], tray[j]) {
				return false
			}
		}
	}
	return len(tray) > 1
}

func (c *Curriculum) reward(id int) *Reward {
	p, ok := c.Pokemon[id]
	if !ok {
		return nil
	}
	return &Reward{
		Pokemon:       p.ID,
		Name:          p.Name,
		Syllables:     p.Syllables,
		SyllableAudio: SyllableAudio(p.Syllables),
		NameAudio:     speech.URL(p.Spoken(), speech.StyleWord),
		Types:         p.Types,
		Dex:           p.Dex,
		DexAudio:      speech.URL(p.Dex, speech.StyleNormal),
	}
}

// SyllableAudio is the clip URL of each syllable, as the voice says it.
func SyllableAudio(syls []string) []string {
	out := make([]string, 0, len(syls))
	for _, s := range syls {
		out = append(out, speech.URL(SpokenSyllable(s), speech.StyleSyllable))
	}
	return out
}

// LearnedPool returns every word and syllable from completed episodes — the
// material the Tour de Combat and the mini-games draw from. False while no
// episode is completed yet.
func (c *Curriculum) LearnedPool(st progress.State) ([]Word, []string, bool) {
	lastDone := -1
	for i, ep := range c.Episodes {
		if st.Episodes[ep.ID].Completions > 0 {
			lastDone = i
		}
	}
	if lastDone < 0 {
		return nil, nil, false
	}
	p := c.poolUpTo(lastDone)
	return p.words, p.syllables, true
}

// TowerID is the virtual episode id of the Tour de Combat. It exists in no
// JSON file: the session is generated fresh from everything already learned.
const TowerID = "tour"

// TowerTaunt is spoken at the tower gate; a constant so SpeechTexts records it.
const TowerTaunt = "Bienvenue à la Tour de Combat ! Lis tous les mots pour monter d'un étage !"

// TowerInfo describes the floor being attempted: the client shows the target
// and counts points; the server recomputes the threshold on the result.
type TowerInfo struct {
	Floor    int  `json:"floor"`
	Rounds   int  `json:"rounds"`
	Need     int  `json:"need"`
	Guardian bool `json:"guardian,omitempty"` // a legendary guards this floor
}

// GuardianEvery: every tenth floor is guarded by a legendary — a longer
// fight, and a rare reward (the ladder puts its best Pokémon on these floors,
// and a guardian already in the Pokédex joins in its shiny form).
const GuardianEvery = 10

// TowerGuardianTaunt is spoken when a guardian floor opens.
const TowerGuardianTaunt = "Attention ! Un Pokémon légendaire garde cet étage. Lis tous les mots pour le battre !"

func towerTitle(floor int, guardian bool) string {
	if guardian {
		return fmt.Sprintf("⚡ Étage %d — Gardien légendaire", floor)
	}
	return fmt.Sprintf("Étage %d", floor)
}

// TowerOpponent is the floor's opponent: a different one per floor, drawn
// from the whole roster (seeded by the floor, so a retried floor is the same
// fight), and a legendary on every guardian floor — one he has caught when
// possible, so beating it can make it shiny.
func (c *Curriculum) TowerOpponent(floor int, st progress.State) (Pokemon, bool) {
	rng := rand.New(rand.NewPCG(uint64(floor), 0x70e7))
	if floor%GuardianEvery == 0 {
		caught := map[int]bool{}
		for _, id := range st.Pokedex {
			caught[id] = true
		}
		var legends, mine []Pokemon
		for _, id := range c.PokemonOrder {
			if p := c.Pokemon[id]; p.Legendary {
				legends = append(legends, p)
				if caught[id] {
					mine = append(mine, p)
				}
			}
		}
		if len(mine) > 0 {
			return mine[(floor/GuardianEvery-1)%len(mine)], true
		}
		if len(legends) > 0 {
			return legends[(floor/GuardianEvery-1)%len(legends)], true
		}
	}
	var pool []Pokemon
	for _, p := range c.Roster() {
		if !p.Legendary {
			pool = append(pool, p)
		}
	}
	return pool[rng.IntN(len(pool))], false
}

// towerRounds ramps the fight length with the floor: 4 words on floor 1 up to
// 8 from floor 9 on — "infinite words", but never an infinite session.
func towerRounds(floor int) int { return min(4+(floor-1)/2, 8) }

// towerTray ramps the trap syllables per word with the floor.
func towerTray(floor int) int { return min(1+floor/4, 3) }

// TowerNeed is the points required to clear a floor: every word scores 1, a
// word built without a single wrong tap scores 2 — so passing means at least
// half the words read perfectly on the first try.
func TowerNeed(rounds int) int { return rounds + (rounds+1)/2 }

// TowerRewardFor returns the tower-exclusive Pokémon awarded for clearing a
// floor, walking the ladder: one every second floor up to TowerEvenUntil,
// then one on every floor; 0 when there is none.
func (c *Curriculum) TowerRewardFor(floor int) int {
	i := towerLadderIndex(floor)
	if i < 0 || i >= len(c.TowerLadder) {
		return 0
	}
	return c.TowerLadder[i]
}

// TowerEvenUntil: up to this floor the ladder pays on even floors only.
const TowerEvenUntil = 40

func towerLadderIndex(floor int) int {
	switch {
	case floor > TowerEvenUntil:
		return TowerEvenUntil/2 + floor - TowerEvenUntil - 1
	case floor >= 2 && floor%2 == 0:
		return floor/2 - 1
	}
	return -1
}

// TowerFloorOf is the floor that awards ladder entry i (inverse of the above).
func TowerFloorOf(i int) int {
	if i < TowerEvenUntil/2 {
		return 2 * (i + 1)
	}
	return TowerEvenUntil + 1 + i - TowerEvenUntil/2
}

// BuildTower generates one floor of the endless battle mode: a fight over words
// drawn from every completed episode, longer and trickier as the floors climb,
// against a random Pokémon from the Pokédex. It grants no episode completion —
// but every word read feeds the spaced repetition, clearing a floor unlocks the
// next, and every second floor a tower-exclusive Pokémon joins the Pokédex.
// Returns false while no episode is completed yet.
func (c *Curriculum) BuildTower(st progress.State, floor int, now time.Time) (Session, bool) {
	lastDone := -1
	for i, ep := range c.Episodes {
		if st.Episodes[ep.ID].Completions > 0 {
			lastDone = i
		}
	}
	if lastDone < 0 {
		return Session{}, false
	}
	if floor < 1 {
		floor = 1
	}

	p := c.poolUpTo(lastDone)
	var words []Word
	for _, w := range p.words {
		if len(w.Syllables) >= 2 {
			words = append(words, w)
		}
	}
	if len(words) < 3 {
		return Session{}, false
	}
	rand.Shuffle(len(words), func(i, j int) { words[i], words[j] = words[j], words[i] })
	// Higher floors prefer longer words; the shuffle above breaks ties.
	target := 2 + min(floor/3, 2)
	sortStable(words, func(a, b Word) bool {
		return abs(len(a.Syllables)-target) < abs(len(b.Syllables)-target)
	})
	rounds := towerRounds(floor)
	if floor%GuardianEvery == 0 {
		rounds = min(rounds+2, 10) // the guardian fights longer
	}
	if len(words) > rounds {
		words = words[:rounds]
	}

	boss, guardian := c.TowerOpponent(floor, st)
	taunt := TowerTaunt
	if guardian {
		taunt = TowerGuardianTaunt
	}
	f := &Fight{
		Pokemon:    boss.ID,
		Name:       boss.Name,
		Syllables:  boss.Syllables,
		HP:         len(words),
		Legendary:  true, // the tower borrows the arena's event staging
		Taunt:      taunt,
		TauntAudio: speech.URL(taunt, speech.StyleNormal),
	}
	for _, w := range words {
		tray := append([]string(nil), w.Syllables...)
		inWord := map[string]bool{}
		for _, s := range w.Syllables {
			inWord[strings.ToLower(s)] = true
		}
		var candidates []string
		for _, s := range p.syllables {
			if !inWord[strings.ToLower(s)] {
				candidates = append(candidates, s)
			}
		}
		for _, d := range pickDistractors(w.Syllables[0], candidates, towerTray(floor)) {
			tray = append(tray, d)
		}
		rand.Shuffle(len(tray), func(i, j int) { tray[i], tray[j] = tray[j], tray[i] })
		f.Rounds = append(f.Rounds, Item{
			ID:        "mot:" + w.Text,
			Prompt:    w.Text,
			Emoji:     w.Pic().Emoji,
			Sprite:    w.Pic().Sprite,
			Icon:      w.Pic().Icon,
			Syllables: w.Syllables,
			Audio:     speech.URL(w.Spoken(), speech.StyleWord),
			Tray:      tray,
		})
	}

	if floor >= DicteeFrom {
		dictateLast(f.Rounds)
	}
	return Session{
		EpisodeID: TowerID,
		Title:     fmt.Sprintf("Tour de Combat — Étage %d", floor),
		Legendary: true,
		Lives:     st.Settings.BattleLives,
		UI:        UIAudio(),
		Tower:     &TowerInfo{Floor: floor, Rounds: len(f.Rounds), Need: TowerNeed(len(f.Rounds)), Guardian: guardian},
		Activities: []Activity{{
			Kind:        "combat",
			Title:       towerTitle(floor, guardian),
			Instruction: "Remets les syllabes dans l'ordre pour lancer l'attaque.",
			Fight:       f,
		}},
	}, true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// sortStable is insertion sort: tiny inputs, and it keeps the shuffle as the
// tie-breaker, same trick as the review queue.
func sortStable(w []Word, less func(a, b Word) bool) {
	for i := 1; i < len(w); i++ {
		for j := i; j > 0 && less(w[j], w[j-1]); j-- {
			w[j], w[j-1] = w[j-1], w[j]
		}
	}
}

// SpeechItem is one recording to make: the same text can legitimately need two
// recordings at different speeds — "mur" is both a word to read and a single
// syllable to blend — so the pair, not the text, is the identity.
type SpeechItem struct {
	Text  string
	Style speech.Style
}

// SpeechTexts lists every string the voice must be able to say, with the style
// to record it in. cmd/pokecontent uses this to generate exactly what is needed.
func (c *Curriculum) SpeechTexts() []SpeechItem {
	var out []SpeechItem
	seen := map[string]bool{}
	add := func(text string, style speech.Style) {
		t := strings.TrimSpace(text)
		if t == "" {
			return
		}
		key := speech.Key(t, style)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, SpeechItem{Text: t, Style: style})
	}

	// Walk the curriculum in teaching order, so whoever is re-recording these in
	// their own voice hits the sounds needed for route 1 first and can stop when
	// they run out of patience.
	addGrapheme := func(id string) {
		g, ok := c.Graphemes[id]
		if !ok {
			return
		}
		add(g.SayAs, speech.StyleSound)
		add(g.Mnemo, speech.StyleNormal)
		add(g.Example, speech.StyleWord)
	}

	for i, ep := range c.Episodes {
		known := c.KnownGraphemes(i)
		for _, id := range ep.NewGraphemes {
			addGrapheme(id)
		}
		add(ep.Story, speech.StyleNormal)
		add(ep.Boss.Taunt, speech.StyleNormal)
		if ep.Badge != nil {
			add(badgePhrase(*ep.Badge), speech.StyleNormal)
		}
		for _, s := range ep.Syllables {
			add(SpokenSyllable(s), speech.StyleSyllable)
			if parts, err := Segment(s, known); err == nil {
				for _, p := range parts {
					if g, ok := c.Graphemes[p]; ok {
						add(g.SayAs, speech.StyleSound)
					}
				}
			}
		}
		for _, w := range ep.Words {
			add(w.Spoken(), speech.StyleWord)
			for _, s := range w.Syllables {
				add(SpokenSyllable(s), speech.StyleSyllable)
			}
		}
		for _, s := range ep.Sentences {
			add(s.Spoken(), speech.StyleWord)
			for _, w := range sentenceWords(s.Text) {
				add(w, speech.StyleWord)
			}
		}
	}
	// Graphemes not yet reached by any episode: still recorded so a new episode
	// never ships with a silent sound.
	for _, id := range c.GraphemeOrder {
		addGrapheme(id)
	}
	for _, id := range c.PokemonOrder {
		p := c.Pokemon[id]
		add(p.Spoken(), speech.StyleWord)
		add(p.Dex, speech.StyleNormal)
		for _, s := range p.Syllables {
			add(SpokenSyllable(s), speech.StyleSyllable)
		}
	}
	for _, p := range c.Arcade {
		add(p.Spoken(), speech.StyleWord)
		for _, s := range p.Syllables {
			add(SpokenSyllable(s), speech.StyleSyllable)
		}
	}
	for _, phrase := range UIPhrases {
		add(phrase, speech.StyleNormal)
	}
	add(TowerTaunt, speech.StyleNormal)
	add(TowerGuardianTaunt, speech.StyleNormal)
	for _, ev := range c.Events {
		add(ev.Story, speech.StyleNormal)
	}
	add(TrainerTaunt, speech.StyleNormal)
	add(TrainerBeaten, speech.StyleNormal)
	add(TrainerWon, speech.StyleNormal)
	for _, phrase := range c.partnerSpeech() {
		add(phrase, speech.StyleNormal)
	}
	for _, phrase := range c.progressionSpeech() {
		add(phrase, speech.StyleNormal)
	}
	for _, phrase := range append([]string{FoundBalls, FoundSuperBall, FoundCandy, FoundBerry}, QuestPhrases...) {
		add(phrase, speech.StyleNormal)
	}
	return out
}

// UIPhrases are the spoken bits of chrome. Kept here so the generator records
// them in the same voice as the content.
var UIPhrases = []string{
	"Bravo !",
	"Super !",
	"Génial !",
	"Bien joué !",
	"Presque ! Écoute encore.",
	"Essaie encore, tu y es presque.",
	"On y va !",
	"Touche la bonne image.",
	"Lis le mot tout seul.",
	"Attaque !",
	"Coup critique !",
	"Super rapide !",
	"K.O. ! Bravo !",
	"Combat parfait !",
	"Tu as gagné le combat !",
	"Oh non, tu as perdu ! Entraîne-toi et reviens plus fort !",
	"Tu as attrapé un nouveau Pokémon !",
	"Ouah ! Un Pokémon chromatique !",
	"Tu es le Champion de la Ligue !",
	"Termine la route précédente !",
	"Celui-là, tu ne l'as pas encore attrapé !",
	"À demain !",

	// The partner, the gift and the tall grass (web/world.js). The gift line
	// says "trois": it must match giftBalls in internal/api/partner.go.
	"Cadeau du jour !",
	"Tu gagnes trois Poké Balls !",
	"Un Pokémon sauvage apparaît !",
	"Lis son nom pour l'attraper !",
	"Lance la Poké Ball !",
	"Relance vite une Poké Ball !",
	"Oh non ! Il est sorti de la Poké Ball !",
	"Oh non, il s'est enfui !",
	"Et hop ! Attrapé !",
	"Tu n'as plus de Poké Balls ! Lis une route pour en gagner.",
	"Il n'y a personne ici. Apprends de nouveaux sons pour trouver des Pokémon !",
	"Il se cache dans les hautes herbes !",
	"Apprends de nouveaux sons pour le trouver !",

	// Lesson instructions (BuildSession): the child cannot read them, so
	// web/app.js hint() reads each one out as its screen opens.
	"Touche chaque lettre pour entendre son son.",
	"Touche la lettre pour entendre son son.",
	"Touche la première lettre, puis la deuxième. Écoute la syllabe !",
	"Écoute la syllabe, puis touche-la.",
	"Lis le mot tout seul, puis touche la bonne image.",
	"Lis la phrase à voix haute. Si un mot est difficile, touche-le !",
	"Quelle image va avec la phrase ?",
	"Remets les syllabes dans l'ordre pour lancer l'attaque.",
	"On révise avant de partir !",
	"Bravo ! Tu peux continuer.",
	"Il faut lire au moins la moitié des mots du premier coup. Réessaie !",
	"La Tour t'a battu cette fois. Réessaie l'étage !",
	"Le combat est perdu : l'épisode recommence du début. Tu vas y arriver !",
	"Touche JOUER pour continuer l'aventure !",

	// Missions, the partner's hunger, the Super Ball, map trainers.
	"Qui joue ? Touche ton Pokémon !",
	"Touche ton code secret : tes quatre Pokémon, dans l'ordre !",
	"Ce n'est pas le bon code. Essaie encore !",
	"Trop d'essais. Attends un peu, puis réessaie.",
	"Ta partie est fermée. Demande à un grand de la rouvrir.",
	"Comment tu t'appelles ? Demande à un grand de t'aider à l'écrire.",
	"Mission accomplie !",
	"Tes missions du jour !",
	"Miam ! Ton partenaire adore les baies !",
	"Ton partenaire a faim ! Trouve une baie sur la carte ou dans les missions.",
	"Choisis ta Poké Ball ! La Super Ball attrape mieux.",
	TrainerTaunt,
	TrainerBeaten,
	TrainerWon,

	// The overworld map (web/overworld.js).
	"Touche une case pour te déplacer, ou utilise les flèches !",
	"C'est la prochaine route ! Touche JOUER !",
	"Tu as déjà gagné ici. Tu veux rejouer ?",
	"La Tour de Combat ! Monte les étages en lisant !",
	"Le coin des mini-jeux !",
	"Ton Pokédex ! Regarde tous tes Pokémon.",
	"Termine la première route pour ouvrir ça !",
	"Cette région est encore fermée ! Termine les routes d'ici.",
	"En route vers la région suivante !",
	"Retour à la région précédente !",

	// Result screens: what the screen shows, said aloud.
	"Il rejoint ta collection !",
	"Tu gagnes des Poké Balls !",
	"Un Pokémon rare de la Tour rejoint ton Pokédex !",
	"Nouveau record !",
	"Incroyable ! Ton Pokémon légendaire devient chromatique !",
	"Il rejoint ton Pokédex sauvage !",

	// Screens of world.js.
	"Il grandit à chaque mot que tu lis.",
	"Touche le cadeau !",
	"Touche les herbes. Qui s'y cache ?",

	// Mini-games (the hint of each game's screen).
	"Choisis un jeu, et bats tes records !",
	"Retrouve les paires !",
	"Touche les Pokémon, mais pas Voltorbe : il explose !",
	"Trop fort !",
	"Incroyable !",
	"Tu es une fusée !",
	"Continue comme ça !",
	"Lis le mot, touche la bonne image !",
	"Remets les syllabes dans l'ordre, encore et encore !",
	"Qui est cette ombre ? Lis les noms !",
	"Associe chaque mot à son image !",
	"Écoute la syllabe, puis touche-la !",
	"Combien de syllabes ? Tape sur le bon tambour !",
	"Quel son manque ? Écoute le mot, trouve les lettres !",

	// Surprise exams (event.go), speed stars and mini-game unlocks.
	ExamSoundsHint,
	ExamPhraseHint,
	ExamRule,
	ExamPassed,
	ExamFailed,
	ExamArrived,
	ExamDialog,
	ExamWhere,
	ExamDone,
	ExamGate,
	ExamList,
	NoTicket,
	TicketWon,
	NewGame,
	GameLocked,
	DicteeHint,
	BookHint,
	BookEnd,
}

// Voice lines of the mini-game unlocks, the battle dictation and « Mon livre ».
const (
	NewGame    = "Un nouveau mini-jeu est débloqué !"
	GameLocked = "Ce jeu s'ouvrira plus loin dans l'aventure !"
	DicteeHint = "Le mot est caché ! Écoute-le, et remets ses syllabes dans l'ordre."
	BookHint   = "Lis la page à voix haute, puis touche J'AI LU !"
	BookEnd    = "Bravo ! Tu as lu tout le livre !"
)

// Fmt is a tiny helper for episode titles in logs.
func (e Episode) Fmt() string { return fmt.Sprintf("%s (route %d) — %s", e.ID, e.Route, e.Title) }
