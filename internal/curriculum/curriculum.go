// Package curriculum loads the reading progression from data/ and validates
// that every word an episode asks the child to decode is actually decodable
// with the graphemes he has been taught so far.
package curriculum

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Picture is how a word is illustrated. Exactly one field is set.
//
// Icon exists because emoji cannot be trusted: an abstract term like the type
// "Normal" or the stat "Vitesse" has no emoji that reads as anything to a
// five-year-old, and newer emoji (🪽, 🪺) are simply absent from the emoji font
// on older Android builds — they show up as a blank tile. Icons are inline SVG
// we ship ourselves, so they always draw and always mean something.
type Picture struct {
	Emoji  string `json:"emoji,omitempty"`
	Sprite string `json:"sprite,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

// Empty reports whether nothing would be drawn.
func (p Picture) Empty() bool { return p.Emoji == "" && p.Sprite == "" && p.Icon == "" }

// Key identifies the picture, so distractors stay visually distinct.
func (p Picture) Key() string {
	switch {
	case p.Sprite != "":
		return "s" + p.Sprite
	case p.Icon != "":
		return "i" + p.Icon
	default:
		return "e" + p.Emoji
	}
}

func pictureOf(emoji, icon string, pokemon int) Picture {
	switch {
	case pokemon != 0:
		return Picture{Sprite: fmt.Sprintf("sprites/art/%d.png", pokemon)}
	case icon != "":
		return Picture{Icon: icon}
	default:
		return Picture{Emoji: emoji}
	}
}

// Icons the frontend knows how to draw. Kept here so `pokecontent check` fails
// on a typo instead of the child seeing an empty tile.
var Icons = map[string]bool{
	"bite": true, "fang": true, "water": true, "flame": true, "leaf": true,
	"bolt": true, "wing": true, "rock": true, "shield": true, "star": true,
	"tornado": true, "sleep": true, "powder": true, "cut": true, "ball": true,
	"trainer": true, "heart": true, "fall": true, "bomb": true, "shadow": true,
	"cocoon": true, "candy": true, "slash": true, "dance": true, "sand": true,
	"impact": true, "rage": true, "speed": true, "sparkle": true, "bulb": true,
	"normal": true, "psy": true, "ice": true, "mirror": true, "bug": true,
}

// Grapheme is one written sign mapped to one sound: "m", "ou", "ch".
type Grapheme struct {
	ID      string `json:"id"`      // "ou"
	Display string `json:"display"` // what the child sees
	SayAs   string `json:"say_as"`  // what the voice pronounces, one single sound
	Kind    string `json:"kind"`    // voyelle | consonne | digraphe | muet
	Example string `json:"example"` // the key word that carries the sound
	Emoji   string `json:"emoji"`   // picture for the example word
	Icon    string `json:"icon"`    // …or one of our own icons
	Pokemon int    `json:"pokemon"` // …or a Pokémon sprite
	Mnemo   string `json:"mnemo"`   // spoken memory hook
}

// Pic returns how to illustrate the example word.
func (g Grapheme) Pic() Picture { return pictureOf(g.Emoji, g.Icon, g.Pokemon) }

// Word is a decodable word plus the picture that gives it meaning. Set Pokemon
// to a dex number and the word is illustrated by that Pokémon's sprite — which
// is what makes "Otaria" or "Salamèche" a reading exercise rather than a label.
// Otherwise Emoji carries the meaning: zero assets, instantly readable at 5.
type Word struct {
	Text      string   `json:"text"`
	SayAs     string   `json:"say_as,omitempty"` // phonetic respelling, for the voice only
	Emoji     string   `json:"emoji,omitempty"`
	Icon      string   `json:"icon,omitempty"`
	Pokemon   int      `json:"pokemon,omitempty"`
	Syllables []string `json:"syllables"`
}

// Pic returns how to illustrate the word.
func (w Word) Pic() Picture { return pictureOf(w.Emoji, w.Icon, w.Pokemon) }

// Spoken is what the voice should say. Invented Pokémon names need a phonetic
// respelling ("Pikatchou") or the synthesiser mangles them, while the child must
// still see the real spelling.
func (w Word) Spoken() string {
	if w.SayAs != "" {
		return w.SayAs
	}
	return w.Text
}

// Sentence is a short decodable sentence with a picture that confirms meaning.
type Sentence struct {
	Text    string `json:"text"`
	SayAs   string `json:"say_as,omitempty"`
	Emoji   string `json:"emoji,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Pokemon int    `json:"pokemon,omitempty"`
}

// Pic mirrors Word.Pic.
func (s Sentence) Pic() Picture { return pictureOf(s.Emoji, s.Icon, s.Pokemon) }

// Spoken mirrors Word.Spoken.
func (s Sentence) Spoken() string {
	if s.SayAs != "" {
		return s.SayAs
	}
	return s.Text
}

// Boss is the Pokémon fought at the end of an episode.
type Boss struct {
	Pokemon int    `json:"pokemon"`
	HP      int    `json:"hp"`
	Taunt   string `json:"taunt"`
}

// BadgeDecl marks an episode as awarding a gym badge (or the champion trophy).
// The id must exist in Badges so a typo fails validation instead of rendering
// an empty medal.
type BadgeDecl struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Badges the frontend knows how to draw (web/icons.js, BADGES map).
var Badges = map[string]bool{
	"glace": true, "orage": true, "psy": true, "volcan": true,
	"maree": true, "arcenciel": true, "tonnerre": true, "ciel": true,
	"champion": true,
}

// Episode is one ~12 minute session: a declaration of content, not a script.
// The playable activity list is generated from it by BuildSession.
type Episode struct {
	ID    string `json:"id"`
	Route int    `json:"route"`
	Title string `json:"title"`
	// Region groups episodes on the world map ("Forêt Émeraude", "Îles du
	// Levant"…). Purely presentational; the play order stays the file order.
	Region       string     `json:"region"`
	Story        string     `json:"story"`
	NewGraphemes []string   `json:"new_graphemes"`
	Syllables    []string   `json:"syllables"`
	Words        []Word     `json:"words"`
	Sentences    []Sentence `json:"sentences"`

	// Legendary marks a milestone arena: it teaches no new sound and draws all
	// its material from everything learned so far, so it doubles as a cumulative
	// review and as the moment a legendary Pokémon joins the Pokédex.
	Legendary bool `json:"legendary,omitempty"`

	// Badge, when set, is awarded after the episode's fight — the arenas hand
	// out gym badges, the champion episode hands out the trophy.
	Badge *BadgeDecl `json:"badge,omitempty"`

	RewardPokemon int  `json:"reward_pokemon"`
	Boss          Boss `json:"boss"`
}

// CatchablePokemon lists every Pokémon completing this episode adds to the
// Pokédex: the featured reward, the boss, and every Pokémon whose name or
// picture the child met in a word or a sentence. Catching everything that was
// read is what makes the Pokédex completable.
func (e Episode) CatchablePokemon() []int {
	seen := map[int]bool{}
	var out []int
	add := func(id int) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	add(e.RewardPokemon)
	add(e.Boss.Pokemon)
	for _, w := range e.Words {
		add(w.Pokemon)
	}
	for _, s := range e.Sentences {
		add(s.Pokemon)
	}
	return out
}

// Pokemon is a curated Pokédex entry. Names are shown syllable-split and read
// aloud; they are motivation, never a decoding test.
type Pokemon struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	SayAs     string   `json:"say_as,omitempty"`
	Syllables []string `json:"syllables"`
	Types     []string `json:"types"`
	Dex       string   `json:"dex"`
	Legendary bool     `json:"legendary,omitempty"`

	// Tower marks a Pokémon obtainable only by climbing the Tour de Combat —
	// awarded every second floor, in pokemon.json file order.
	Tower bool `json:"tower,omitempty"`
}

// Spoken is the phonetic respelling if one is given, else the name itself.
func (p Pokemon) Spoken() string {
	if p.SayAs != "" {
		return p.SayAs
	}
	return p.Name
}

// Curriculum is the whole loaded content set.
//
// GraphemeOrder and PokemonOrder exist because Go map iteration is randomised:
// without them the recorder page would shuffle its rows on every reload, and the
// audio generator would report work in a different order each run.
type Curriculum struct {
	Graphemes     map[string]Grapheme
	GraphemeOrder []string
	Episodes      []Episode
	byID          map[string]*Episode
	Pokemon       map[int]Pokemon
	PokemonOrder  []int

	// TowerLadder lists the tower-exclusive Pokémon in award order: the reward
	// for clearing floor 2n is TowerLadder[n-1].
	TowerLadder []int
}

// sylPron maps a written syllable to the respelling the voice must say instead.
// A TTS reads a bare chunk like "ny" as letter names and a homograph like
// "cher" as the word /ʃɛʁ/ — not the /ʃe/ the child is blending. The override
// is applied identically when generating audio and when building URLs, so the
// file on disk and the URL the tablet asks for always agree.
//
// Package-level because the item builders are package functions; there is one
// curriculum per process and reloading just rewrites the same entries.
var sylPron = map[string]string{}

// SpokenSyllable resolves a syllable to what the voice should actually say.
func SpokenSyllable(s string) string {
	if p, ok := sylPron[strings.ToLower(s)]; ok {
		return p
	}
	return s
}

// Load reads data/graphemes.json, data/pokemon.json, data/episodes/*.json and
// the optional data/pronunciation.json syllable respellings.
func Load(dir string) (*Curriculum, error) {
	c := &Curriculum{
		Graphemes: map[string]Grapheme{},
		byID:      map[string]*Episode{},
		Pokemon:   map[int]Pokemon{},
	}

	pron := map[string]string{}
	if err := readJSON(filepath.Join(dir, "pronunciation.json"), &pron); err == nil {
		for k, v := range pron {
			sylPron[strings.ToLower(k)] = v
		}
	} else if !os.IsNotExist(errors.Unwrap(err)) {
		return nil, err
	}

	var graphemes []Grapheme
	if err := readJSON(filepath.Join(dir, "graphemes.json"), &graphemes); err != nil {
		return nil, err
	}
	for _, g := range graphemes {
		if g.Display == "" {
			g.Display = g.ID
		}
		if g.SayAs == "" {
			g.SayAs = g.ID
		}
		if _, dup := c.Graphemes[g.ID]; !dup {
			c.GraphemeOrder = append(c.GraphemeOrder, g.ID)
		}
		c.Graphemes[g.ID] = g
	}

	var pokemon []Pokemon
	if err := readJSON(filepath.Join(dir, "pokemon.json"), &pokemon); err != nil {
		return nil, err
	}
	for _, p := range pokemon {
		if _, dup := c.Pokemon[p.ID]; !dup {
			c.PokemonOrder = append(c.PokemonOrder, p.ID)
			if p.Tower {
				c.TowerLadder = append(c.TowerLadder, p.ID)
			}
		}
		c.Pokemon[p.ID] = p
	}

	files, err := filepath.Glob(filepath.Join(dir, "episodes", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		var ep Episode
		if err := readJSON(f, &ep); err != nil {
			return nil, err
		}
		c.Episodes = append(c.Episodes, ep)
	}
	for i := range c.Episodes {
		c.byID[c.Episodes[i].ID] = &c.Episodes[i]
	}

	if len(c.Episodes) == 0 {
		return nil, fmt.Errorf("curriculum: no episode found in %s/episodes", dir)
	}
	for i := range c.Episodes {
		if c.Episodes[i].Region == "" {
			c.Episodes[i].Region = "Lecturia"
		}
	}
	return c, nil
}

func readJSON(path string, dst any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("curriculum: %w", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("curriculum: %s: %w", filepath.Base(path), err)
	}
	return nil
}

// Episode returns an episode by id.
func (c *Curriculum) Episode(id string) (*Episode, bool) {
	ep, ok := c.byID[id]
	return ep, ok
}

// Index returns the position of an episode in the progression, or -1.
func (c *Curriculum) Index(id string) int {
	for i, ep := range c.Episodes {
		if ep.ID == id {
			return i
		}
	}
	return -1
}

// KnownGraphemes returns every grapheme taught up to and including episode idx,
// longest first so that greedy segmentation prefers "ou" over "o"+"u".
func (c *Curriculum) KnownGraphemes(idx int) []string {
	seen := map[string]bool{}
	var out []string
	for i := 0; i <= idx && i < len(c.Episodes); i++ {
		for _, id := range c.Episodes[i].NewGraphemes {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return len(out[a]) > len(out[b]) })
	return out
}

// Segment splits a word into graphemes using greedy longest-match over the
// allowed set. It returns an error naming the first undecodable position, which
// is what makes `pokecontent check` a real safety net against content that the
// child cannot possibly read yet.
func Segment(word string, allowed []string) ([]string, error) {
	lower := strings.ToLower(word)
	var out []string
	for i := 0; i < len(lower); {
		if lower[i] == ' ' || lower[i] == '\'' || lower[i] == '-' {
			i++
			continue
		}
		matched := ""
		for _, g := range allowed {
			if strings.HasPrefix(lower[i:], g) && len(g) > len(matched) {
				matched = g
			}
		}
		if matched == "" {
			return out, fmt.Errorf("%q: caractère non enseigné à la position %d (%q)", word, i, lower[i:])
		}
		out = append(out, matched)
		i += len(matched)
	}
	return out, nil
}

// checkSentenceVariety flags an episode whose sentences all lean on the same
// verb. Two lines of "X utilise Y." in a row read as one exercise repeated, and
// the child stops reading the middle of the sentence.
func (c *Curriculum) checkSentenceVariety(ep Episode) []error {
	if len(ep.Sentences) < 2 {
		return nil
	}
	verbs := map[string]int{}
	for _, s := range ep.Sentences {
		toks := strings.Fields(strings.ToLower(strings.Trim(s.Text, ".!?")))
		if len(toks) > 1 {
			verbs[toks[1]]++
		}
	}
	var errs []error
	for v, n := range verbs {
		if n > 1 {
			errs = append(errs, fmt.Errorf("%s: le verbe %q revient dans %d phrases — varie les tournures", ep.ID, v, n))
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

// Validate reports content problems: unknown graphemes, undecodable words or
// sentences, missing Pokémon, malformed syllable splits.
func (c *Curriculum) Validate() []error {
	var errs []error
	for i, ep := range c.Episodes {
		for _, id := range ep.NewGraphemes {
			if _, ok := c.Graphemes[id]; !ok {
				errs = append(errs, fmt.Errorf("%s: grapheme inconnu %q", ep.ID, id))
			}
		}
		known := c.KnownGraphemes(i)

		for _, s := range ep.Syllables {
			if _, err := Segment(s, known); err != nil {
				errs = append(errs, fmt.Errorf("%s: syllabe %v", ep.ID, err))
			}
		}
		for _, w := range ep.Words {
			if _, err := Segment(w.Text, known); err != nil {
				errs = append(errs, fmt.Errorf("%s: mot %v", ep.ID, err))
			}
			// Pokémon names are capitalised, so compare case-insensitively.
			if join := strings.ToLower(strings.Join(w.Syllables, "")); join != strings.ToLower(w.Text) {
				errs = append(errs, fmt.Errorf("%s: mot %q: découpage %v ne recompose pas le mot", ep.ID, w.Text, w.Syllables))
			}
			if w.Pic().Empty() {
				errs = append(errs, fmt.Errorf("%s: mot %q: il faut un emoji, une icône ou un pokemon", ep.ID, w.Text))
			}
			if w.Icon != "" && !Icons[w.Icon] {
				errs = append(errs, fmt.Errorf("%s: mot %q: icône inconnue %q", ep.ID, w.Text, w.Icon))
			}
			if w.Pokemon != 0 {
				if _, ok := c.Pokemon[w.Pokemon]; !ok {
					errs = append(errs, fmt.Errorf("%s: mot %q: pokémon #%d absent de pokemon.json", ep.ID, w.Text, w.Pokemon))
				}
			}
		}
		for _, s := range ep.Sentences {
			for _, tok := range strings.Fields(strings.Trim(s.Text, ".!?")) {
				tok = strings.Trim(tok, ".,!?")
				if _, err := Segment(tok, known); err != nil {
					errs = append(errs, fmt.Errorf("%s: phrase %v", ep.ID, err))
				}
			}
			if s.Pic().Empty() {
				errs = append(errs, fmt.Errorf("%s: phrase %q: il faut un emoji, une icône ou un pokemon", ep.ID, s.Text))
			}
			if s.Icon != "" && !Icons[s.Icon] {
				errs = append(errs, fmt.Errorf("%s: phrase %q: icône inconnue %q", ep.ID, s.Text, s.Icon))
			}
			if s.Pokemon != 0 {
				if _, ok := c.Pokemon[s.Pokemon]; !ok {
					errs = append(errs, fmt.Errorf("%s: phrase %q: pokémon #%d absent de pokemon.json", ep.ID, s.Text, s.Pokemon))
				}
			}
		}
		errs = append(errs, c.checkSentenceVariety(ep)...)
		for _, id := range []int{ep.RewardPokemon, ep.Boss.Pokemon} {
			if id != 0 {
				if _, ok := c.Pokemon[id]; !ok {
					errs = append(errs, fmt.Errorf("%s: pokémon #%d absent de pokemon.json", ep.ID, id))
				}
			}
		}
		if ep.Badge != nil {
			if !Badges[ep.Badge.ID] {
				errs = append(errs, fmt.Errorf("%s: badge inconnu %q", ep.ID, ep.Badge.ID))
			}
			if ep.Badge.Name == "" {
				errs = append(errs, fmt.Errorf("%s: badge %q sans nom", ep.ID, ep.Badge.ID))
			}
		}
	}
	errs = append(errs, c.checkPokedexCompletable()...)
	return errs
}

// checkPokedexCompletable fails when a Pokémon of pokemon.json can never enter
// the Pokédex — a silhouette the child could stare at forever is exactly the
// kind of dead end this game must not have. Tower Pokémon are catchable by
// climbing the Tour de Combat, so they are exempt from the episode check.
func (c *Curriculum) checkPokedexCompletable() []error {
	catchable := map[int]bool{}
	for _, ep := range c.Episodes {
		for _, id := range ep.CatchablePokemon() {
			catchable[id] = true
		}
	}
	var errs []error
	for _, id := range c.PokemonOrder {
		if !catchable[id] && !c.Pokemon[id].Tower {
			errs = append(errs, fmt.Errorf("pokédex: %s (#%d) n'est attrapable dans aucun épisode", c.Pokemon[id].Name, id))
		}
	}
	return errs
}
