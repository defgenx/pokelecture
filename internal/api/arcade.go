package api

// The mini-games corner ("l'Arcade"): timed games built client-side from the
// material the child has actually learned. The server hands out the pool and
// keeps the best scores; the comprehension games also report their answers so
// the spaced repetition learns from play.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

type arcadeWord struct {
	Text          string   `json:"text"`
	Audio         string   `json:"audio"`
	Emoji         string   `json:"emoji,omitempty"`
	Sprite        string   `json:"sprite,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Syllables     []string `json:"syllables"`
	SyllableAudio []string `json:"syllable_audio"`
	// Graphemes is the word cut into the signs taught so far (le son manquant).
	Graphemes []string `json:"graphemes,omitempty"`
}

// arcadeSound is one taught grapheme and its sound, for le son manquant.
type arcadeSound struct {
	ID    string `json:"id"`
	Audio string `json:"audio"`
	Kind  string `json:"kind"`
}

type arcadeSyllable struct {
	Text  string `json:"text"`
	Audio string `json:"audio"`
	// Sound is what the ear hears ("s·an" for san and sen): two syllables with
	// the same Sound are never offered together. Near lists the pool's
	// minimal pairs (ba/da, fa/va) for L'oreille fine.
	Sound string   `json:"sound"`
	Near  []string `json:"near,omitempty"`
}

// arcadeSentence is one page of « Mon livre »: a sentence already read.
type arcadeSentence struct {
	Text      string   `json:"text"`
	Audio     string   `json:"audio"`
	Words     []string `json:"words"`
	WordAudio []string `json:"word_audio"`
	Emoji     string   `json:"emoji,omitempty"`
	Sprite    string   `json:"sprite,omitempty"`
	Icon      string   `json:"icon,omitempty"`
}

// arcadePokemon is one face of the mini-games' roster: the course's Pokémon,
// caught or not, plus the arcade-only extras.
type arcadePokemon struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	NameAudio string   `json:"name_audio"`
	Sprite    string   `json:"sprite"`
	Syllables []string `json:"syllables,omitempty"`
}

type arcadeView struct {
	Words     []arcadeWord     `json:"words"`
	Syllables []arcadeSyllable `json:"syllables"`
	Pokemon   []arcadePokemon  `json:"pokemon"`
	Sounds    []arcadeSound    `json:"sounds"`
	// Homophones: graphemes never offered side by side (curriculum.Homophones).
	Homophones [][]string       `json:"homophones"`
	Sentences  []arcadeSentence `json:"sentences"`
}

// handleArcade returns everything learned so far, for the games to draw from.
func (s *Server) handleArcade(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	words, syllables, ok := s.cur.LearnedPool(st)
	if !ok {
		http.Error(w, "les mini-jeux ouvrent après le premier épisode terminé", http.StatusConflict)
		return
	}

	known := s.cur.KnownGraphemes(s.lastDone(st))
	out := arcadeView{Words: []arcadeWord{}, Syllables: []arcadeSyllable{}, Pokemon: []arcadePokemon{}, Sounds: []arcadeSound{},
		Homophones: curriculum.Homophones, Sentences: []arcadeSentence{}}
	for _, id := range s.cur.GraphemeOrder {
		for _, k := range known {
			if k == id {
				g := s.cur.Graphemes[id]
				out.Sounds = append(out.Sounds, arcadeSound{ID: id, Kind: g.Kind, Audio: speech.URL(g.SayAs, speech.StyleSound)})
			}
		}
	}
	for _, p := range s.cur.Roster() {
		out.Pokemon = append(out.Pokemon, arcadePokemon{
			ID:        p.ID,
			Name:      p.Name,
			NameAudio: speech.URL(p.Spoken(), speech.StyleWord),
			Sprite:    fmt.Sprintf("sprites/art/%d.png", p.ID),
			Syllables: p.Syllables,
		})
	}
	for _, wd := range words {
		pic := wd.Pic()
		gs := s.cur.WordGraphemes(wd.Text)
		var sylAudio []string
		for _, syl := range wd.Syllables {
			sylAudio = append(sylAudio, speech.URL(curriculum.SpokenSyllable(syl), speech.StyleSyllable))
		}
		out.Words = append(out.Words, arcadeWord{
			Graphemes:     gs,
			SyllableAudio: sylAudio,
			Text:          wd.Text,
			Audio:         speech.URL(wd.Spoken(), speech.StyleWord),
			Emoji:         pic.Emoji,
			Sprite:        pic.Sprite,
			Icon:          pic.Icon,
			Syllables:     wd.Syllables,
		})
	}
	for _, syl := range syllables {
		v := arcadeSyllable{
			Text:  syl,
			Audio: speech.URL(curriculum.SpokenSyllable(syl), speech.StyleSyllable),
			Sound: curriculum.SoundKey(syl),
		}
		for _, o := range syllables {
			if curriculum.MinimalPair(syl, o) {
				v.Near = append(v.Near, o)
			}
		}
		out.Syllables = append(out.Syllables, v)
	}
	for _, sn := range s.cur.SentencesUpTo(s.lastDone(st)) {
		pic := sn.Pic()
		v := arcadeSentence{Text: sn.Text, Audio: speech.URL(sn.Spoken(), speech.StyleWord),
			Emoji: pic.Emoji, Sprite: pic.Sprite, Icon: pic.Icon}
		for _, wd := range curriculum.SentenceWords(sn.Text) {
			v.Words = append(v.Words, wd)
			v.WordAudio = append(v.WordAudio, speech.URL(wd, speech.StyleWord))
		}
		out.Sentences = append(out.Sentences, v)
	}
	writeJSON(w, http.StatusOK, out)
}

type recordRequest struct {
	Game  string `json:"game"`
	Score int    `json:"score"`
	// Lower flips the comparison for time-based games (memory: fastest wins).
	Lower bool `json:"lower"`
	// Items lets the comprehension games feed the spaced repetition.
	Items []struct {
		ID      string `json:"id"`
		Correct bool   `json:"correct"`
	} `json:"items"`
}

var arcadeGames = map[string]bool{"livre": true,
	"memory": true, "chasse": true, "lecture": true, "oreille": true,
	"atelier": true, "mystere": true, "paires": true,
	"tambour": true, "trou": true,
}

func (s *Server) handleRecord(w http.ResponseWriter, r *http.Request) {
	var req recordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !arcadeGames[req.Game] {
		http.Error(w, "requête invalide", http.StatusBadRequest)
		return
	}

	now := time.Now()
	for _, it := range req.Items {
		if it.ID != "" {
			s.store.Answer(it.ID, it.Correct, now)
		}
	}
	beaten := s.store.Record(req.Game, req.Score, req.Lower)
	balls := 0
	if beaten {
		balls = recordBalls
		s.store.AddBalls(balls)
	}
	xp := min(arcadeMaxXP, max(0, req.Score))
	if req.Lower {
		xp = arcadeTimedXP
	}
	ev := s.award(xp)
	words := 0
	for _, it := range req.Items {
		if it.Correct && strings.HasPrefix(it.ID, "mot:") {
			words++
		}
	}
	tickets := 0
	if req.Game == "livre" { // reading the book earns a ticket
		tickets = s.payTicket()
	}
	done := append(s.progress("arcade", 1), s.progress("words", words)...)
	tev, m := s.settle()
	if err := s.store.Save(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"beaten":        beaten,
		"balls_won":     balls,
		"tickets_won":   tickets,
		"partner_event": ev,
		"quests_done":   done,
		"trainer_event": tev,
		"milestone":     milestoneLine(s, m),
		"state":         s.state(),
	})
}
