package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
	"github.com/adelvecchio/pokelecture/internal/speech"
)

// The tablet has no French voice of its own: an audio URL with no file
// behind it falls back to the browser's voice — often an English one reading
// a French syllable. This walks every endpoint (every episode, the Tower, the
// mini-games, the tall grass) on a save where everything is unlocked, and
// fails on any URL whose clip is missing or was not made by the track's voice.
var audioURL = regexp.MustCompile(`audio/[a-z0-9-]+\` + speech.Ext)

func TestEveryServedClipExists(t *testing.T) {
	root := "../.."
	cur, err := curriculum.Load(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	store.ChoosePartner(4, 0)
	for _, ep := range cur.Episodes {
		store.CompleteEpisode(ep.ID, 3, time.Now())
	}
	srv := httptest.NewServer(New(cur, store, filepath.Join(root, "web"), "/").Routes())
	defer srv.Close()

	paths := []string{"api/state", "api/arcade", "api/wild", "api/session/tour"}
	for _, ep := range cur.Episodes {
		paths = append(paths, "api/session/"+ep.ID)
	}
	urls := map[string]string{}
	// Sessions are shuffled: a few draws per endpoint cover the variations.
	for round := 0; round < 4; round++ {
		for _, p := range paths {
			resp, err := http.Get(srv.URL + "/" + p)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: %s", p, resp.Status)
			}
			for _, u := range audioURL.FindAllString(string(b), -1) {
				urls[u] = p
			}
		}
	}

	voices := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(root, "web", "audio", "voices.json")); err == nil {
		_ = json.Unmarshal(b, &voices)
	}
	want := ""
	for _, v := range voices { // the track's voice: whichever made most clips
		if strings.HasPrefix(v, "elevenlabs:") {
			want = v
			break
		}
	}

	var bad []string
	for u, from := range urls {
		key := strings.TrimSuffix(strings.TrimPrefix(u, "audio/"), speech.Ext)
		if _, err := os.Stat(filepath.Join(root, "web", u)); err != nil {
			bad = append(bad, from+": manquant "+u)
		} else if want != "" && voices[key] != want {
			bad = append(bad, from+": "+u+" fait par "+voices[key])
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Error(b)
	}
	if len(urls) < 1000 {
		t.Fatalf("seulement %d URL(s) audio trouvées : le motif ne reconnaît plus les URLs servies", len(urls))
	}
	t.Logf("%d URL(s) audio vérifiées", len(urls))
}
