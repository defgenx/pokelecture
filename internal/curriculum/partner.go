package curriculum

import (
	"fmt"
	"strconv"
)

// PartnerLine is one starter's evolution chain (data/partners.json): the
// species in order and the level at which each next stage is reached.
type PartnerLine struct {
	Stages []int `json:"stages"`
	Levels []int `json:"levels"` // len(Stages)-1 entries, ascending
}

// Partner levels: every one starts at PartnerStartLevel and needs
// XPToNext(level) experience points to gain the next one.
const (
	PartnerStartLevel = 5
	PartnerMaxLevel   = 100
)

// XPToNext is the experience a level-L partner needs to reach L+1. Gentle on
// purpose: a session (~50 XP) is a level early on, the first evolution comes
// after the first arena, the last one around the end of the course.
func XPToNext(level int) int { return 10 + 2*level }

// PartnerLevel converts a total XP into a level and the progress inside it.
func PartnerLevel(xp int) (level, into, need int) {
	level = PartnerStartLevel
	for level < PartnerMaxLevel && xp >= XPToNext(level) {
		xp -= XPToNext(level)
		level++
	}
	return level, xp, XPToNext(level)
}

// LineOf returns the evolution chain a species belongs to: a starter's
// (partners.json) or any other (evolutions.json).
func (c *Curriculum) LineOf(species int) (PartnerLine, bool) {
	for _, l := range append(append([]PartnerLine(nil), c.Partners...), c.Evolutions...) {
		for _, s := range l.Stages {
			if s == species {
				return l, true
			}
		}
	}
	return PartnerLine{}, false
}

// IsStarter reports whether a species can be picked as the first partner.
func (c *Curriculum) IsStarter(species int) bool {
	for _, l := range c.Partners {
		if len(l.Stages) > 0 && l.Stages[0] == species {
			return true
		}
	}
	return false
}

// PartnerForm is the species a line has reached at a level.
func (l PartnerLine) PartnerForm(level int) (species, stage int) {
	for stage = 0; stage < len(l.Levels) && level >= l.Levels[stage]; stage++ {
	}
	return l.Stages[stage], stage
}

// NextEvolution is the next species and the level it comes at; 0 when final.
func (l PartnerLine) NextEvolution(level int) (species, at int) {
	_, stage := l.PartnerForm(level)
	if stage >= len(l.Levels) {
		return 0, 0
	}
	return l.Stages[stage+1], l.Levels[stage]
}

// AnyPokemon resolves a dex number against the course and the arcade roster.
func (c *Curriculum) AnyPokemon(id int) (Pokemon, bool) {
	if p, ok := c.Pokemon[id]; ok {
		return p, true
	}
	for _, p := range c.Arcade {
		if p.ID == id {
			return p, true
		}
	}
	return Pokemon{}, false
}

// Partner phrases. Shown with the real name, spoken with the respelling: the
// voice line is generated from Spoken(), the tablet only ever sees the URL.
func PartnerChosenPhrase(name string) string {
	return "Tu as choisi " + name + " ! Lis bien, et il deviendra très fort !"
}
func PartnerLevelPhrase(name string) string { return name + " monte d'un niveau !" }
func PartnerEvolvingPhrase(name string) string {
	return "Quoi ? " + name + " évolue !"
}
func PartnerEvolvedPhrase(from, to string) string {
	return "Bravo ! " + from + " a évolué en " + to + " !"
}

// StarterPrompt opens the choice of the first partner.
const StarterPrompt = "Choisis ton premier Pokémon ! Il t'accompagnera partout et grandira à chaque mot que tu lis."

// Base is the first stage of a species' line (itself when it has none):
// the key a team member is stored under.
func (c *Curriculum) Base(species int) int {
	if l, ok := c.LineOf(species); ok {
		return l.Stages[0]
	}
	return species
}

// StageLevel is the lowest level at which a line shows this species: a
// Pokémon joined already evolved starts at its stage's level.
func (c *Curriculum) StageLevel(species int) int {
	l, ok := c.LineOf(species)
	if !ok {
		return PartnerStartLevel
	}
	for k, s := range l.Stages {
		if s == species && k > 0 {
			return l.Levels[k-1]
		}
	}
	return PartnerStartLevel
}

// XPForLevel is the total XP a fresh partner needs to reach a level.
func XPForLevel(level int) int {
	xp := 0
	for l := PartnerStartLevel; l < level; l++ {
		xp += XPToNext(l)
	}
	return xp
}

// FormAt is the species a member of base `base` shows at a level.
func (c *Curriculum) FormAt(base, level int) int {
	if l, ok := c.LineOf(base); ok {
		sp, _ := l.PartnerForm(level)
		return sp
	}
	return base
}

func (c *Curriculum) validatePartners() []error {
	var errs []error
	if len(c.Partners) == 0 {
		return []error{fmt.Errorf("partners.json: aucun starter")}
	}
	all := append(append([]PartnerLine(nil), c.Partners...), c.Evolutions...)
	seen := map[int]string{}
	for i, l := range all {
		where := "partners.json[" + strconv.Itoa(i) + "]"
		if i >= len(c.Partners) {
			where = "evolutions.json[" + strconv.Itoa(i-len(c.Partners)) + "]"
		}
		for _, s := range l.Stages {
			if prev, dup := seen[s]; dup {
				errs = append(errs, fmt.Errorf("%s: #%d est déjà dans %s", where, s, prev))
			}
			seen[s] = where
		}
		if len(l.Stages) == 0 || len(l.Levels) != len(l.Stages)-1 {
			errs = append(errs, fmt.Errorf("%s: il faut un niveau d'évolution par stade après le premier", where))
			continue
		}
		for _, s := range l.Stages {
			if _, ok := c.AnyPokemon(s); !ok {
				errs = append(errs, fmt.Errorf("%s: #%d absent de pokemon.json et d'arcade.json", where, s))
			}
		}
		for k := 1; k < len(l.Levels); k++ {
			if l.Levels[k] <= l.Levels[k-1] {
				errs = append(errs, fmt.Errorf("%s: niveaux d'évolution non croissants", where))
			}
		}
	}
	return errs
}

// partnerSpeech lists every partner voice line for the generator.
func (c *Curriculum) partnerSpeech() []string {
	var out []string
	name := func(id int) string { p, _ := c.AnyPokemon(id); return p.Spoken() }
	out = append(out, StarterPrompt)
	for i, l := range append(append([]PartnerLine(nil), c.Partners...), c.Evolutions...) {
		if i < len(c.Partners) {
			out = append(out, PartnerChosenPhrase(name(l.Stages[0])))
		}
		for k, s := range l.Stages {
			out = append(out, PartnerLevelPhrase(name(s)))
			if k+1 < len(l.Stages) {
				out = append(out, PartnerEvolvingPhrase(name(s)), PartnerEvolvedPhrase(name(s), name(l.Stages[k+1])))
			}
		}
	}
	return out
}

// Voice lines of the map's hidden objects (internal/api/quests.go).
const (
	FoundBalls     = "Super ! Tu as trouvé des Poké Balls !"
	FoundSuperBall = "Ouah ! Une Super Ball ! Elle attrape mieux les Pokémon."
	FoundCandy     = "Un Super Bonbon ! Ton partenaire devient plus fort !"
	FoundBerry     = "Une baie ! Ton partenaire va se régaler."
)

// QuestPhrases are the daily missions' texts, read out to the child; they
// must stay in sync with questDefs in internal/api/quests.go.
var QuestPhrases = []string{
	"Attrape deux Pokémon dans les hautes herbes.",
	"Lis dix mots sans te tromper.",
	"Termine une route.",
	"Bats un Dresseur sur la carte.",
	"Trouve un objet caché sur la carte.",
	"Donne une baie à ton partenaire.",
	"Joue à un mini-jeu.",
	"Monte un étage de la Tour de Combat.",
	"Pêche un Pokémon avec ta canne.",
	"Lis ton livre jusqu'au bout.",
	"Bats un record dans un mini-jeu.",
	"Atteins le niveau trois dans un mini-jeu.",
	"Téracristallise ton partenaire contre un Dresseur.",
	"Gagne cinq étoiles sur une route.",
	"Gagne un combat sans une seule erreur.",
}

// LessonQuestPhrases is the daily limit's mission, one per objective (1–5
// lessons): QuestPhrases' sibling, read out the same way.
var LessonQuestPhrases = []string{
	"Fais une leçon aujourd'hui.",
	"Fais deux leçons aujourd'hui.",
	"Fais trois leçons aujourd'hui.",
	"Fais quatre leçons aujourd'hui.",
	"Fais cinq leçons aujourd'hui.",
}

// Trainer levels: the child's own, fed by everything he does.

// TrainerLevel converts trainer XP to a level (from 1) and the progress in it.
func TrainerLevel(xp int) (level, into, need int) {
	level = 1
	for xp >= trainerNeed(level) {
		xp -= trainerNeed(level)
		level++
	}
	return level, xp, trainerNeed(level)
}

func trainerNeed(level int) int { return 60 + 15*level }

// TrainerTitles: the title earned at each threshold level.
var TrainerTitles = []struct {
	Level int
	Title string
}{
	{1, "Dresseur débutant"}, {5, "Dresseur apprenti"}, {10, "Dresseur confirmé"},
	{15, "Dresseur expert"}, {20, "Champion d'arène"}, {30, "Maître Pokémon"},
}

// TrainerTitle is the title of a level.
func TrainerTitle(level int) string {
	t := TrainerTitles[0].Title
	for _, tt := range TrainerTitles {
		if level >= tt.Level {
			t = tt.Title
		}
	}
	return t
}

// TrainerTitlePhrase announces a new title.
func TrainerTitlePhrase(title string) string { return "Tu deviens " + title + " !" }

// Voice lines of the trainer level, the team and the Pokédex milestones.
const (
	TrainerLevelUp = "Bravo ! Tu montes de niveau de Dresseur !"
	TeamPrompt     = "Choisis ton partenaire ! Chaque Pokémon gagne des niveaux avec toi."
	TeamChosen     = "Il devient ton partenaire !"
	EvolveToGet    = "Fais évoluer un de tes Pokémon pour l'avoir !"
)

var milestoneWords = map[int]string{
	10: "Dix", 20: "Vingt", 30: "Trente", 40: "Quarante", 50: "Cinquante",
	60: "Soixante", 70: "Soixante-dix", 80: "Quatre-vingts", 90: "Quatre-vingt-dix",
}

// MilestonePhrase celebrates n Pokémon in the Pokédex (n a multiple of 10).
func MilestonePhrase(n int) string {
	if w, ok := milestoneWords[n]; ok {
		return "Bravo ! " + w + " Pokémon dans ton Pokédex !"
	}
	return "Bravo ! Ton Pokédex grandit !"
}

// WherePhrase tells where a Pokémon still to catch is waiting.
func WherePhrase(ep Episode) string {
	switch {
	case !ep.Legendary && ep.Route > 0:
		return fmt.Sprintf("Il t'attend sur la route %d !", ep.Route)
	case ep.Badge != nil:
		return "Il t'attend dans une arène légendaire !"
	default:
		return "Il t'attend dans la région " + ep.Region + " !"
	}
}

// TowerTopPhrase: a tower-exclusive above TowerEvenUntil.
const TowerTopPhrase = "Il t'attend tout en haut de la Tour de Combat ! Monte encore !"

// TowerWherePhrase: a tower-exclusive's floor.
func TowerWherePhrase(floor int) string {
	return fmt.Sprintf("Il t'attend à l'étage %d de la Tour de Combat !", floor)
}

// Where is the line that says where to find a Pokémon: the first episode
// that catches it, else its tower floor, else an evolution of another one.
func (c *Curriculum) Where(id int) string {
	for _, ep := range c.Episodes {
		for _, got := range ep.CatchablePokemon() {
			if got == id {
				return WherePhrase(ep)
			}
		}
	}
	for i, t := range c.TowerLadder {
		if t == id {
			if f := TowerFloorOf(i); f <= TowerEvenUntil {
				return TowerWherePhrase(f)
			}
			// One line for the whole top: a clip per floor costs a voice line each.
			return TowerTopPhrase
		}
	}
	for _, ev := range c.Events {
		if ev.Pokemon == id {
			return ExamWhere
		}
	}
	return EvolveToGet
}

// progressionSpeech lists the trainer, team, milestone and "where" lines.
func (c *Curriculum) progressionSpeech() []string {
	out := []string{TrainerLevelUp, TeamPrompt, TeamChosen, EvolveToGet}
	for _, t := range TrainerTitles[1:] {
		out = append(out, TrainerTitlePhrase(t.Title))
	}
	for n := 10; n <= 100; n += 10 {
		out = append(out, MilestonePhrase(n))
	}
	for _, id := range c.PokemonOrder {
		out = append(out, c.Where(id))
	}
	return out
}
