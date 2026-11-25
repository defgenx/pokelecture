package api

// The child's own Pokémon (the starter, levelled by reading), the Poké Balls
// economy, the daily gift and the tall grass. Every reading activity pays
// experience; the course pays Poké Balls; the tall grass spends them.

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// Rewards. Balls: the course is where they are earned, so reading the
// adventure is what keeps the tall grass open.
const (
	giftBalls       = 3
	episodeBalls    = 2 // + one per star
	towerBalls      = 2
	trainerBalls    = 2
	recordBalls     = 1
	wildXP          = 6
	wildPerfectXP   = 4
	sessionBonusXP  = 10
	starXP          = 5
	maxSessionXP    = 90
	arcadeMaxXP     = 15
	arcadeTimedXP   = 8
	headStartPerEp  = 8
	headStartCap    = 300 // just short of the first evolution
	wildShinyChance = 16
	superBallBonus  = 0.20
)

type voiceLine struct {
	Text  string `json:"text"`
	Audio string `json:"audio"`
}

func (s *Server) line(text, spoken string) voiceLine {
	return voiceLine{Text: text, Audio: speech.URL(spoken, speech.StyleNormal)}
}

type partnerView struct {
	Line      int       `json:"line"`
	Species   int       `json:"species"`
	Name      string    `json:"name"`
	NameAudio string    `json:"name_audio"`
	Level     int       `json:"level"`
	XP        int       `json:"xp"`   // inside the current level
	Need      int       `json:"need"` // to reach the next level
	NextID    int       `json:"next_species,omitempty"`
	NextName  string    `json:"next_name,omitempty"`
	NextLevel int       `json:"next_level,omitempty"`
	LevelUp   voiceLine `json:"level_up"`
	Types     []string  `json:"types"` // its first type picks its battle attack
}

type starterView struct {
	Species   int       `json:"species"`
	Name      string    `json:"name"`
	NameAudio string    `json:"name_audio"`
	Types     []string  `json:"types"`
	Chosen    voiceLine `json:"chosen"`
}

// partnerEvent tells the client what the last award did: levels gained and,
// at most once per award, an evolution to stage.
type partnerEvent struct {
	XP        int        `json:"xp"`
	FromLevel int        `json:"from_level"`
	ToLevel   int        `json:"to_level"`
	From      int        `json:"from,omitempty"`
	To        int        `json:"to,omitempty"`
	ToName    string     `json:"to_name,omitempty"`
	Evolving  *voiceLine `json:"evolving,omitempty"`
	Evolved   *voiceLine `json:"evolved,omitempty"`
	LevelUp   *voiceLine `json:"level_up,omitempty"`
	NewCatch  bool       `json:"new_catch,omitempty"` // the evolved form is new to a collection
}

func (s *Server) pokemonName(id int) (name, spoken string) {
	p, _ := s.cur.AnyPokemon(id)
	return p.Name, p.Spoken()
}

// activeXP is the active partner's line base and XP.
func activeXP(st progress.State) (base, xp int) {
	if st.Partner == nil {
		return 0, 0
	}
	if st.Active == 0 || st.Active == st.Partner.Line {
		return st.Partner.Line, st.Partner.XP
	}
	return st.Active, st.Team[st.Active]
}

func (s *Server) memberView(base, xp int) *partnerView {
	level, into, need := curriculum.PartnerLevel(xp)
	species := s.cur.FormAt(base, level)
	name, spoken := s.pokemonName(species)
	v := &partnerView{
		Line: base, Species: species, Name: name,
		NameAudio: speech.URL(spoken, speech.StyleWord),
		Level:     level, XP: into, Need: need,
		LevelUp: s.line(curriculum.PartnerLevelPhrase(name), curriculum.PartnerLevelPhrase(spoken)),
	}
	if p, ok := s.cur.AnyPokemon(species); ok {
		v.Types = p.Types
	}
	if l, ok := s.cur.LineOf(base); ok {
		if next, at := l.NextEvolution(level); next != 0 {
			v.NextID, v.NextLevel = next, at
			v.NextName, _ = s.pokemonName(next)
		}
	}
	return v
}

func (s *Server) partnerView(st progress.State) *partnerView {
	if st.Partner == nil {
		return nil
	}
	base, xp := activeXP(st)
	return s.memberView(base, xp)
}

func (s *Server) starters() []starterView {
	var out []starterView
	for _, l := range s.cur.Partners {
		id := l.Stages[0]
		p, _ := s.cur.AnyPokemon(id)
		out = append(out, starterView{
			Species: id, Name: p.Name, Types: p.Types,
			NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
			Chosen:    s.line(curriculum.PartnerChosenPhrase(p.Name), curriculum.PartnerChosenPhrase(p.Spoken())),
		})
	}
	return out
}

// award gives the active partner (and the trainer) experience and
// describes what happened. An evolution also brings the new form into the
// collection it belongs to — the Pokédex, or the tall-grass one.
func (s *Server) award(xp int) *partnerEvent {
	base, before, after := s.store.GainXP(xp)
	if before == after {
		return nil
	}
	fromLvl, _, _ := curriculum.PartnerLevel(before)
	toLvl, _, _ := curriculum.PartnerLevel(after)
	ev := &partnerEvent{XP: after - before, FromLevel: fromLvl, ToLevel: toLvl}
	fromSp := s.cur.FormAt(base, fromLvl)
	toSp := s.cur.FormAt(base, toLvl)
	if toLvl > fromLvl {
		name, spoken := s.pokemonName(toSp)
		lu := s.line(curriculum.PartnerLevelPhrase(name), curriculum.PartnerLevelPhrase(spoken))
		ev.LevelUp = &lu
	}
	if toSp != fromSp {
		fn, fs := s.pokemonName(fromSp)
		tn, ts := s.pokemonName(toSp)
		evolving := s.line(curriculum.PartnerEvolvingPhrase(fn), curriculum.PartnerEvolvingPhrase(fs))
		evolved := s.line(curriculum.PartnerEvolvedPhrase(fn, tn), curriculum.PartnerEvolvedPhrase(fs, ts))
		ev.From, ev.To, ev.ToName = fromSp, toSp, tn
		ev.Evolving, ev.Evolved = &evolving, &evolved
		if _, inDex := s.cur.Pokemon[toSp]; inDex {
			ev.NewCatch = s.store.Catch(toSp)
		} else {
			ev.NewCatch = s.store.AddWild(toSp)
		}
	}
	return ev
}

func (s *Server) handlePartner(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Species int `json:"species"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !s.cur.IsStarter(req.Species) {
		http.Error(w, "starter inconnu", http.StatusBadRequest)
		return
	}
	st := s.store.Snapshot()
	done := 0
	for _, e := range st.Episodes {
		done += min(e.Completions, 1)
	}
	if !s.store.ChoosePartner(req.Species, min(done*headStartPerEp, headStartCap)) {
		http.Error(w, "le partenaire est déjà choisi", http.StatusConflict)
		return
	}
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"state": s.state()})
}

func (s *Server) handleGift(w http.ResponseWriter, r *http.Request) {
	opened := s.store.Snapshot().Partner != nil && s.store.OpenGift(time.Now(), giftBalls)
	if opened {
		s.save()
	}
	writeJSON(w, http.StatusOK, map[string]any{"opened": opened, "balls": giftBalls, "state": s.state()})
}

/* ------------------------------------------------------------ tall grass --- */

type wildView struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	NameAudio     string   `json:"name_audio"`
	Syllables     []string `json:"syllables"`
	SyllableAudio []string `json:"syllable_audio"`
	Sprite        string   `json:"sprite"`
	Caught        bool     `json:"caught"`
	Shiny         bool     `json:"shiny,omitempty"` // caught in shiny form
	Available     bool     `json:"available"`       // readable at the child's level
	Water         bool     `json:"water,omitempty"` // fished, not met in the grass
}

type wildPoolView struct {
	Balls   int        `json:"balls"`
	Pokemon []wildView `json:"pokemon"`
	Traps   []string   `json:"traps"`      // learned syllables, for the tray
	TrapAud []string   `json:"trap_audio"` // aligned with Traps
	Shiny   int        `json:"shiny_odds"`
}

// lastDone is the index of the furthest completed episode, -1 if none.
func (s *Server) lastDone(st progress.State) int {
	last := -1
	for i, ep := range s.cur.Episodes {
		if st.Episodes[ep.ID].Completions > 0 {
			last = i
		}
	}
	return last
}

func (s *Server) wildViews(st progress.State) []wildView {
	last := s.lastDone(st)
	caught := toSet(st.Wild)
	shiny := toSet(st.WildShiny)
	var out []wildView
	for _, p := range s.cur.Arcade {
		if !p.Wild() && !p.Fish() {
			continue
		}
		at := s.cur.WildUnlock(p)
		if at < 0 {
			continue
		}
		v := wildView{
			ID: p.ID, Name: p.Name, Syllables: p.Syllables,
			NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
			Sprite:    fmt.Sprintf("sprites/art/%d.png", p.ID),
			Caught:    caught[p.ID], Shiny: shiny[p.ID],
			Available: at <= last,
			Water:     p.Water,
		}
		for _, syl := range p.Syllables {
			v.SyllableAudio = append(v.SyllableAudio, speech.URL(curriculum.SpokenSyllable(syl), speech.StyleSyllable))
		}
		out = append(out, v)
	}
	return out
}

func toSet(xs []int) map[int]bool {
	m := make(map[int]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func (s *Server) handleWild(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	_, syllables, ok := s.cur.LearnedPool(st)
	if !ok {
		http.Error(w, "les hautes herbes ouvrent après le premier épisode terminé", http.StatusConflict)
		return
	}
	out := wildPoolView{Balls: st.Balls, Pokemon: s.wildViews(st), Shiny: wildShinyChance, Traps: []string{}, TrapAud: []string{}}
	for _, syl := range syllables {
		out.Traps = append(out.Traps, syl)
		out.TrapAud = append(out.TrapAud, speech.URL(curriculum.SpokenSyllable(syl), speech.StyleSyllable))
	}
	writeJSON(w, http.StatusOK, out)
}

// catchChance is the odds one Poké Ball holds: reading the name well is what
// makes the catch likely, a shiny fights harder, and each new throw at the
// same Pokémon gets a little easier — a miss stings, it never locks him out.
func catchChance(slips, attempt int, shiny bool) float64 {
	p := 0.85
	switch {
	case slips == 1:
		p = 0.65
	case slips >= 2:
		p = 0.45
	}
	if shiny {
		p -= 0.15
	}
	p += 0.10 * float64(max(0, attempt-1))
	return min(p, 0.95)
}

// handleWildCatch throws one Poké Ball. It is spent either way; the roll
// lives here, so the odds are the server's.
func (s *Server) handleWildCatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID      int  `json:"id"`
		Shiny   bool `json:"shiny"`
		Slips   int  `json:"slips"`
		Attempt int  `json:"attempt"` // 1 for the first throw at this Pokémon
		Super   bool `json:"super"`   // a Super Ball from the bag: better odds
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	st := s.store.Snapshot()
	allowed := false
	for _, v := range s.wildViews(st) {
		if v.ID == req.ID && v.Available && (!v.Water || s.cur.HasRod(st)) {
			allowed = true
		}
	}
	if !allowed {
		http.Error(w, "ce Pokémon ne vit pas encore dans les hautes herbes", http.StatusBadRequest)
		return
	}
	chance := catchChance(req.Slips, max(1, req.Attempt), req.Shiny)
	if req.Super {
		if !s.store.AddItem("superball", -1) {
			http.Error(w, "plus de Super Balls", http.StatusConflict)
			return
		}
		chance = min(chance+superBallBonus, 0.98)
		s.store.AddBalls(1) // CatchWild below spends a ball: the Super Ball replaces it
	} else if st.Balls <= 0 {
		http.Error(w, "plus de Poké Balls", http.StatusConflict)
		return
	}
	if rand.Float64() >= chance {
		s.store.AddBalls(-1)
		s.save()
		writeJSON(w, http.StatusOK, map[string]any{"caught": false, "chance": chance, "state": s.state()})
		return
	}
	_, isNew := s.store.CatchWild(req.ID, req.Shiny)
	xp := wildXP
	if req.Slips == 0 {
		xp += wildPerfectXP
	}
	ev := s.award(xp)
	s.store.TrainerGain(catchTrainerXP)
	done := s.progress("catch", 1)
	tev, m := s.settle()
	s.save()
	writeJSON(w, http.StatusOK, map[string]any{"caught": true, "new": isNew, "chance": chance, "partner_event": ev,
		"quests_done": done, "trainer_event": tev, "milestone": milestoneLine(s, m), "state": s.state()})
}

func (s *Server) save() {
	if err := s.store.Save(); err != nil {
		log.Printf("sauvegarde: %v", err)
	}
}
