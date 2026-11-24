package api

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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
	// 🔒 Privé / 🌐 Public: only the palette's Pokémon, exactly three; [] clears.
	for _, bad := range []string{`{"code":[25,4]}`, `{"code":[25,4,999]}`, `{"code":[25,4,7,1]}`} {
		if got := post("/api/admin/lock?player="+mia, bad); got != http.StatusBadRequest {
			t.Errorf("code refusé attendu pour %s : %d", bad, got)
		}
	}
	if got := post("/api/admin/lock?player="+mia, `{"code":[25,4,7]}`); got != http.StatusOK {
		t.Fatalf("rendre Mia privée : %d", got)
	}
	if st, _ := reg.Get(mia); !st.CheckLock([]int{25, 4, 7}) || st.CheckLock(nil) {
		t.Fatal("Mia doit être privée avec le code Pikachu, Salamèche, Carapuce")
	}
	if st, _ := reg.Get(leo); st.LockTag() != "" {
		t.Fatal("Léo doit rester public")
	}
	if got := post("/api/admin/lock?player="+mia, `{"code":[]}`); got != http.StatusOK {
		t.Fatalf("rendre Mia publique : %d", got)
	}
	if st, _ := reg.Get(mia); st.LockTag() != "" {
		t.Fatal("Mia doit être redevenue publique")
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

func TestTheGameNamesAndCreatesPlayers(t *testing.T) {
	cur, _ := curriculum.Load(filepath.Join("..", "..", "data"))
	reg, _ := progress.OpenRegistry(t.TempDir(), "") // fresh install: one unnamed "Dresseur"
	app := httptest.NewServer(NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").Routes())
	defer app.Close()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	post := func(path, body string) int {
		resp, err := c.Post(app.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	named := func() (string, bool) {
		resp, _ := c.Get(app.URL + "/api/state")
		var s map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&s)
		return s["name"].(string), s["named"].(bool)
	}
	if name, ok := named(); ok || name != progress.DefaultName {
		t.Fatalf("installation neuve : %q named=%v, attendu Dresseur sans prénom", name, ok)
	}
	if got := post("/api/name", `{"name":"Léo"}`); got != http.StatusOK {
		t.Fatalf("donner son prénom : %d", got)
	}
	if name, ok := named(); !ok || name != "Léo" {
		t.Fatalf("après le prénom : %q named=%v", name, ok)
	}
	// A named child is renamed in the admin only.
	if got := post("/api/name", `{"name":"Intrus"}`); got != http.StatusConflict {
		t.Fatalf("renommer un joueur qui a un prénom depuis le jeu : %d, attendu 409", got)
	}
	// "➕ Nouveau joueur": the new one becomes this device's player.
	if got := post("/api/players", `{"name":"Mia"}`); got != http.StatusOK {
		t.Fatalf("nouveau joueur : %d", got)
	}
	if name, _ := named(); name != "Mia" {
		t.Fatalf("l'appareil doit passer au nouveau joueur : %q", name)
	}
	for len(reg.List()) < maxPlayers {
		if _, err := reg.Create("X"); err != nil {
			t.Fatal(err)
		}
	}
	if got := post("/api/players", `{"name":"Un de trop"}`); got != http.StatusConflict {
		t.Fatalf("au-delà de %d joueurs : %d, attendu 409", maxPlayers, got)
	}
}

func TestPrivatePlayerNeedsItsCode(t *testing.T) {
	cur, _ := curriculum.Load(filepath.Join("..", "..", "data"))
	reg, _ := progress.OpenRegistry(t.TempDir(), "Nino")
	nino := reg.List()[0].ID
	mia, _ := reg.Create("Mia") // public
	st, _ := reg.Get(nino)
	st.SetLock([]int{25, 4, 7}) // private: Pikachu, Salamèche, Carapuce
	app := httptest.NewServer(NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").Routes())
	defer app.Close()
	device := func() *http.Client { jar, _ := cookiejar.New(nil); return &http.Client{Jar: jar} }
	pick := func(c *http.Client, id, code string) int {
		body := `{"id":"` + id + `"`
		if code != "" {
			body += `,"code":` + code
		}
		resp, err := c.Post(app.URL+"/api/players/select", "application/json", strings.NewReader(body+"}"))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	stateCode := func(c *http.Client) int {
		resp, _ := c.Get(app.URL + "/api/state")
		resp.Body.Close()
		return resp.StatusCode
	}

	tablet := device()
	if got := pick(tablet, nino, ""); got != http.StatusForbidden {
		t.Fatalf("privé sans code : %d, attendu 403", got)
	}
	if got := pick(tablet, nino, "[25,7,4]"); got != http.StatusForbidden {
		t.Fatalf("mauvais ordre : %d, attendu 403", got)
	}
	if got := stateCode(tablet); got != http.StatusConflict {
		t.Fatalf("sans le bon code, la sauvegarde ne s'ouvre pas : %d", got)
	}
	if got := pick(tablet, nino, "[25,4,7]"); got != http.StatusOK {
		t.Fatalf("bon code : %d", got)
	}
	if got := stateCode(tablet); got != http.StatusOK {
		t.Fatalf("après le bon code : %d", got)
	}
	// Public players need nothing.
	if got := pick(device(), mia, ""); got != http.StatusOK {
		t.Fatalf("joueur public : %d", got)
	}

	// A hand-made cookie naming Nino does not open the save.
	forger := device()
	u, _ := url.Parse(app.URL)
	forger.Jar.SetCookies(u, []*http.Cookie{{Name: playerCookie, Value: nino, Path: "/"}})
	if got := stateCode(forger); got != http.StatusConflict {
		t.Fatalf("cookie forgé (sans signature) : %d, attendu 409", got)
	}
	forger.Jar.SetCookies(u, []*http.Cookie{{Name: playerCookie, Value: nino + ".AAAA", Path: "/"}})
	if got := stateCode(forger); got != http.StatusConflict {
		t.Fatalf("cookie à fausse signature : %d, attendu 409", got)
	}

	// A new code asks every device again.
	st.SetLock([]int{1, 133, 39})
	if got := stateCode(tablet); got != http.StatusConflict {
		t.Fatalf("code changé : l'appareil doit redemander, %d", got)
	}

	// Five wrong codes: wait.
	guess := device()
	for i := 0; i < lockMaxFails; i++ {
		pick(guess, nino, "[52,52,52]")
	}
	if got := pick(guess, nino, "[1,133,39]"); got != http.StatusTooManyRequests {
		t.Fatalf("après %d essais ratés, même le bon code doit attendre : %d", lockMaxFails, got)
	}
	// The player list says which are private — never the code.
	resp, _ := device().Get(app.URL + "/api/players")
	var list struct {
		Players []map[string]any `json:"players"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	for _, p := range list.Players {
		if p["id"] == nino && p["locked"] != true {
			t.Errorf("Nino doit apparaître privé")
		}
		for k := range p {
			if strings.Contains(k, "hash") || strings.Contains(k, "salt") || k == "lock" {
				t.Errorf("la liste des joueurs expose %q", k)
			}
		}
	}
}
