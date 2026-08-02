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

// Reward is the caught Pokémon.
type Reward struct {
	Pokemon   int      `json:"pokemon"`
	Name      string   `json:"name"`
	Syllables []string `json:"syllables"`
	NameAudio string   `json:"name_audio"`
	Types     []string `json:"types"`
	Dex       string   `json:"dex"`
	DexAudio  string   `json:"dex_audio"`
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
}

// Session is the generated playable episode.
type Session struct {
	EpisodeID     string            `json:"episode_id"`
	Route         int               `json:"route"`
	Title         string            `json:"title"`
	RewardPokemon int               `json:"reward_pokemon"`
	Legendary     bool              `json:"legendary,omitempty"`
	UI            map[string]string `json:"ui"`
	Activities    []Activity        `json:"activities"`
}

// uiAudio returns the recorded chrome phrases so praise and feedback come out
// in the same voice as the lesson instead of the browser's default.
func uiAudio() map[string]string {
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
	maxRounds  = 4
	// Above this many new sounds, the discovery cards collapse into a gallery.
	maxSoloCards = 3
	// A legendary arena is a longer fight and a wider review than a normal route.
	maxLegendaryPhrases = 3
	maxLegendaryRounds  = 6
)

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
		UI:            uiAudio(),
	}

	if ep.Story != "" {
		s.Activities = append(s.Activities, Activity{
			Kind:  "story",
			Title: ep.Title,
			Text:  ep.Story,
			Audio: speech.URL(ep.Story, speech.StyleNormal),
		})
	}

	if rev := c.reviewActivity(idx, p, st, now); rev != nil {
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

	if items := ecouteItems(syllables, p.syllables, maxEcoute); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "ecoute",
			Title:       "Écoute et trouve",
			Instruction: "Écoute la syllabe, puis touche-la.",
			Items:       items,
		})
	}

	if items := lectureItems(words, p.words, maxLecture); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "lecture",
			Title:       "Lis le mot",
			Instruction: "Lis le mot tout seul, puis touche la bonne image.",
			Items:       items,
		})
	}

	if items := phraseItems(sentences); len(items) > 0 {
		s.Activities = append(s.Activities, Activity{
			Kind:        "phrase",
			Title:       "Lis la phrase",
			Instruction: "Lis la phrase, puis touche l'image qui va avec.",
			Items:       items,
		})
	}

	if f := c.fight(ep, words, p.syllables); f != nil {
		s.Activities = append(s.Activities, Activity{
			Kind:        "combat",
			Title:       "Combat !",
			Instruction: "Remets les syllabes dans l'ordre pour lancer l'attaque.",
			Fight:       f,
		})
	}

	if r := c.reward(ep.RewardPokemon); r != nil {
		s.Activities = append(s.Activities, Activity{
			Kind:   "recompense",
			Title:  "Tu l'as attrapé !",
			Reward: r,
		})
	}

	return s
}

// reviewActivity picks the weakest due material from earlier episodes. It mixes
// syllables and words in one screen so the warm-up stays short.
func (c *Curriculum) reviewActivity(idx int, p pool, st progress.State, now time.Time) *Activity {
	if idx <= 0 {
		return nil
	}
	prev := c.poolUpTo(idx - 1)

	type cand struct {
		id    string
		item  Item
		score int // lower = more urgent
	}
	var cands []cand

	for _, syl := range prev.syllables {
		id := "syl:" + syl
		rec, tracked := st.Items[id]
		if tracked && !rec.Due(now) {
			continue
		}
		it := ecouteItem(syl, p.syllables)
		it.ID = id
		cands = append(cands, cand{id: id, item: it, score: rec.Box})
	}
	for _, w := range prev.words {
		id := "mot:" + w.Text
		rec, tracked := st.Items[id]
		if tracked && !rec.Due(now) {
			continue
		}
		it := lectureItem(w, p.words)
		it.ID = id
		cands = append(cands, cand{id: id, item: it, score: rec.Box})
	}
	if len(cands) == 0 {
		return nil
	}

	rand.Shuffle(len(cands), func(i, j int) { cands[i], cands[j] = cands[j], cands[i] })
	// Stable sort on box keeps the shuffle as a tie-breaker inside a box.
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && cands[j].score < cands[j-1].score; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	if len(cands) > maxReview {
		cands = cands[:maxReview]
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
			Audio:      speech.URL(syl, speech.StyleSyllable),
		})
		if len(out) == maxFusion {
			break
		}
	}
	return out
}

func ecouteItems(target, allSyllables []string, limit int) []Item {
	var out []Item
	for _, syl := range target {
		out = append(out, ecouteItem(syl, allSyllables))
		if len(out) == limit {
			break
		}
	}
	return out
}

// ecouteItem plays a syllable and asks the child to point at it. Distractors
// are minimal pairs when possible ("ma" vs "mi"), which is where the real
// discrimination work happens.
func ecouteItem(syl string, allSyllables []string) Item {
	it := Item{
		ID:     "syl:" + syl,
		Prompt: syl,
		Audio:  speech.URL(syl, speech.StyleSyllable),
	}
	distract := pickDistractors(syl, allSyllables, 2)
	labels := append([]string{syl}, distract...)
	rand.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
	for _, l := range labels {
		it.Choices = append(it.Choices, Choice{
			Label:   l,
			Audio:   speech.URL(l, speech.StyleSyllable),
			Correct: l == syl,
		})
	}
	return it
}

func pickDistractors(target string, all []string, n int) []string {
	var near, far []string
	for _, s := range all {
		if s == target {
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

func lectureItems(target, allWords []Word, limit int) []Item {
	var out []Item
	for _, w := range target {
		out = append(out, lectureItem(w, allWords))
		if len(out) == limit {
			break
		}
	}
	return out
}

// lectureItem shows a written word and three pictures. The audio is attached
// but the frontend only plays it after an answer: he must decode first.
func lectureItem(w Word, allWords []Word) Item {
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
		it.SyllableAudio = append(it.SyllableAudio, speech.URL(syl, speech.StyleSyllable))
	}
	var others []Word
	for _, o := range allWords {
		if o.Text != w.Text && o.Pic().Key() != pic.Key() {
			others = append(others, o)
		}
	}
	rand.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
	if len(others) > 2 {
		others = others[:2]
	}

	choices := []Choice{{Emoji: pic.Emoji, Sprite: pic.Sprite, Icon: pic.Icon, Label: w.Text, Correct: true}}
	for _, o := range others {
		op := o.Pic()
		choices = append(choices, Choice{Emoji: op.Emoji, Sprite: op.Sprite, Icon: op.Icon, Label: o.Text})
	}
	rand.Shuffle(len(choices), func(i, j int) { choices[i], choices[j] = choices[j], choices[i] })
	it.Choices = choices
	return it
}

func phraseItems(sentences []Sentence) []Item {
	if len(sentences) == 0 {
		return nil
	}
	var out []Item
	for i, s := range sentences {
		pic := s.Pic()
		it := Item{
			ID:     "phrase:" + s.Text,
			Prompt: s.Text,
			Emoji:  pic.Emoji,
			Sprite: pic.Sprite,
			Icon:   pic.Icon,
			Audio:  speech.URL(s.Spoken(), speech.StyleWord),
		}
		choices := []Choice{{Emoji: pic.Emoji, Sprite: pic.Sprite, Icon: pic.Icon, Correct: true}}
		for j, o := range sentences {
			if j != i && o.Pic().Key() != pic.Key() {
				op := o.Pic()
				choices = append(choices, Choice{Emoji: op.Emoji, Sprite: op.Sprite, Icon: op.Icon})
			}
			if len(choices) == 3 {
				break
			}
		}
		rand.Shuffle(len(choices), func(a, b int) { choices[a], choices[b] = choices[b], choices[a] })
		it.Choices = choices
		out = append(out, it)
	}
	return out
}

// fight builds the boss rounds: rebuild each attack word from shuffled
// syllables. Ordering syllables is segmentation practice disguised as a combo.
func (c *Curriculum) fight(ep *Episode, words []Word, allSyllables []string) *Fight {
	boss, ok := c.Pokemon[ep.Boss.Pokemon]
	if !ok {
		return nil
	}
	hp := ep.Boss.HP
	if hp == 0 {
		hp = 3
	}
	rounds := maxRounds
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
		for _, d := range pickDistractors(w.Syllables[0], allSyllables, 1) {
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
	f.HP = min(f.HP, len(f.Rounds))
	return f
}

func (c *Curriculum) reward(id int) *Reward {
	p, ok := c.Pokemon[id]
	if !ok {
		return nil
	}
	return &Reward{
		Pokemon:   p.ID,
		Name:      p.Name,
		Syllables: p.Syllables,
		NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
		Types:     p.Types,
		Dex:       p.Dex,
		DexAudio:  speech.URL(p.Dex, speech.StyleNormal),
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
		for _, s := range ep.Syllables {
			add(s, speech.StyleSyllable)
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
				add(s, speech.StyleSyllable)
			}
		}
		for _, s := range ep.Sentences {
			add(s.Spoken(), speech.StyleWord)
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
	}
	for _, phrase := range UIPhrases {
		add(phrase, speech.StyleNormal)
	}
	return out
}

// UIPhrases are the spoken bits of chrome. Kept here so the generator records
// them in the same voice as the content.
var UIPhrases = []string{
	"Bravo !",
	"Super !",
	"Presque ! Écoute encore.",
	"Essaie encore, tu y es presque.",
	"On y va !",
	"Touche la bonne image.",
	"Lis le mot tout seul.",
	"Attaque !",
	"Tu as gagné le combat !",
	"Tu as attrapé un nouveau Pokémon !",
	"À demain !",
}

// Fmt is a tiny helper for episode titles in logs.
func (e Episode) Fmt() string { return fmt.Sprintf("%s (route %d) — %s", e.ID, e.Route, e.Title) }
