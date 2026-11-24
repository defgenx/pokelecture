package curriculum

import (
	"fmt"

	"github.com/adelvecchio/pokelecture/internal/progress"
)

// GameUnlocks opens the mini-games one by one along the story: each opens
// when its episode is completed. Ids match Arcade.GAMES in web/app.js and
// web/games.js; README « Mini-jeux » owns the schedule.
var GameUnlocks = []struct{ Game, After string }{
	{"memory", "ep01"},
	{"chasse", "ep02"},
	{"lecture", "ep03"},
	{"oreille", "ep03b"},
	{"atelier", "ep05"},
	{"mystere", "ep06b"},
	{"paires", "ep08"},
	{"tambour", "ep09b"},
	{"trou", "ep12b"},
}

// GamesOpen lists the unlocked mini-games, in schedule order. A game stays
// open once any episode from its own onward is done (replay-as-new, skips).
func (c *Curriculum) GamesOpen(st progress.State) []string {
	var out []string
	for _, g := range GameUnlocks {
		if st.Settings.FreePlay || c.ReachedEpisode(g.After, st) {
			out = append(out, g.Game)
		}
	}
	return out
}

// GameAfter is the episode that opens a game ("" if the game is unknown).
func GameAfter(game string) string {
	for _, g := range GameUnlocks {
		if g.Game == game {
			return g.After
		}
	}
	return ""
}

func (c *Curriculum) validateGames() []error {
	var errs []error
	for _, g := range GameUnlocks {
		if c.Index(g.After) < 0 {
			errs = append(errs, fmt.Errorf("mini-jeu %q : épisode %q inconnu", g.Game, g.After))
		}
	}
	return errs
}
