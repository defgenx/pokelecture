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

// LineOf returns the evolution chain a species belongs to.
func (c *Curriculum) LineOf(species int) (PartnerLine, bool) {
	for _, l := range c.Partners {
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

func (c *Curriculum) validatePartners() []error {
	var errs []error
	if len(c.Partners) == 0 {
		return []error{fmt.Errorf("partners.json: aucun starter")}
	}
	for i, l := range c.Partners {
		where := "partners.json[" + strconv.Itoa(i) + "]"
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
	for _, l := range c.Partners {
		out = append(out, PartnerChosenPhrase(name(l.Stages[0])))
		for k, s := range l.Stages {
			out = append(out, PartnerLevelPhrase(name(s)))
			if k+1 < len(l.Stages) {
				out = append(out, PartnerEvolvingPhrase(name(s)), PartnerEvolvedPhrase(name(s), name(l.Stages[k+1])))
			}
		}
	}
	return out
}
