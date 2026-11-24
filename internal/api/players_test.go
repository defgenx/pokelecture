package api

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

func TestEachPlayerHasItsOwnSave(t *testing.T) {
	cur, err := curriculum.Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := progress.OpenRegistry(t.TempDir(), "Léo")
	if err != nil {
		t.Fatal(err)
	}
	leo := reg.List()[0].ID
	mia, _ := reg.Create("Mia")
	app := httptest.NewServer(NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").Routes())
	defer app.Close()

	device := func() *http.Client { jar, _ := cookiejar.New(nil); return &http.Client{Jar: jar} }
	tablet, phone := device(), device()

	// Two players and none picked: the game asks "Qui joue ?".
	if resp, _ := tablet.Get(app.URL + "/api/state"); resp.StatusCode != http.StatusConflict {
		t.Fatalf("sans joueur choisi : %d, attendu 409", resp.StatusCode)
	}
	pick := func(c *http.Client, id string) {
		resp, err := c.Post(app.URL+"/api/players/select", "application/json", strings.NewReader(`{"id":"`+id+`"}`))
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("choisir %s : %v %v", id, err, resp.StatusCode)
		}
	}
	pick(tablet, leo)
	pick(phone, mia)

	if resp, _ := tablet.Post(app.URL+"/api/partner", "application/json", strings.NewReader(`{"species":4}`)); resp.StatusCode != http.StatusOK {
		t.Fatalf("Léo choisit Salamèche : %d", resp.StatusCode)
	}
	state := func(c *http.Client) map[string]any {
		resp, _ := c.Get(app.URL + "/api/state")
		var s map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&s)
		return s
	}
	if s := state(tablet); s["name"] != "Léo" || s["partner"] == nil {
		t.Fatalf("tablette de Léo : %v / %v", s["name"], s["partner"])
	}
	if s := state(phone); s["name"] != "Mia" || s["partner"] != nil {
		t.Fatalf("Mia ne doit pas avoir le partenaire de Léo : %v / %v", s["name"], s["partner"])
	}

	if resp, _ := tablet.Post(app.URL+"/api/players/select", "application/json", strings.NewReader(`{"id":"../../x"}`)); resp.StatusCode == http.StatusOK {
		t.Fatal("un identifiant inventé ne doit pas être accepté")
	}
}

// The admin acts on the player it names (?player=), and creates players.
func TestAdminActsOnTheNamedPlayer(t *testing.T) {
	cur, _ := curriculum.Load(filepath.Join("..", "..", "data"))
	reg, _ := progress.OpenRegistry(t.TempDir(), "Léo")
	leo := reg.List()[0].ID
	mia, _ := reg.Create("Mia")
	a, _ := NewAuth("client-id", "secret", "papa@example.com", "", strings.Repeat("k", 40))
	g := &fakeGoogle{claims: goodClaims("papa@example.com")}
	google := g.server(t)
	defer google.Close()
	a.authURL, a.tokenURL = google.URL+"/auth", google.URL+"/token"
	app := httptest.NewServer(NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").WithAuth(a).Routes())
	defer app.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	signIn(t, app, c, g)

	post := func(path, body string) int {
		req, _ := http.NewRequest(http.MethodPost, app.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("/api/admin/name?player="+mia, `{"name":"Mia-Rose"}`); got != http.StatusOK {
		t.Fatalf("renommer Mia : %d", got)
	}
	l, _ := reg.Get(leo)
	m, _ := reg.Get(mia)
	if l.Snapshot().Name != "Léo" || m.Snapshot().Name != "Mia-Rose" {
		t.Fatalf("le renommage doit toucher le seul joueur nommé : %q / %q", l.Snapshot().Name, m.Snapshot().Name)
	}
	if got := post("/api/admin/players", `{"name":"Tom"}`); got != http.StatusOK || len(reg.List()) != 3 {
		t.Fatalf("créer Tom : %d, %d joueur(s)", got, len(reg.List()))
	}
	if got := post("/api/admin/reset?player=nope", ``); got != http.StatusNotFound {
		t.Fatalf("joueur inconnu : %d, attendu 404", got)
	}
	// Signed out, nothing of this is reachable.
	anon := &http.Client{}
	resp, _ := anon.Post(app.URL+"/api/admin/players", "application/json", strings.NewReader(`{"name":"Intrus"}`))
	if resp.StatusCode != http.StatusUnauthorized || len(reg.List()) != 3 {
		t.Fatalf("créer un joueur sans connexion : %d", resp.StatusCode)
	}
}
