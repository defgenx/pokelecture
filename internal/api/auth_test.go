package api

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

// fakeGoogle stands in for Google's token endpoint: it hands back an ID token
// carrying whatever claims the test sets.
type fakeGoogle struct {
	claims func(nonce string) map[string]any
	nonce  string
}

func (g *fakeGoogle) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "the-code" || r.Form.Get("client_secret") != "secret" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		payload, _ := json.Marshal(g.claims(g.nonce))
		tok := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": tok})
	}))
}

func newAuthApp(t *testing.T, g *fakeGoogle) (*httptest.Server, *http.Client) {
	t.Helper()
	cur, err := curriculum.Load(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := progress.Open(filepath.Join(t.TempDir(), "p.json"), "")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(cur, store, filepath.Join("..", "..", "web"), "/")
	if g != nil {
		a, err := NewAuth("client-id", "secret", "Papa@Example.com, maman@example.com", "", strings.Repeat("k", 40))
		if err != nil {
			t.Fatal(err)
		}
		google := g.server(t)
		t.Cleanup(google.Close)
		a.authURL, a.tokenURL = google.URL+"/auth", google.URL+"/token"
		srv.WithAuth(a)
	}
	app := httptest.NewServer(srv.Routes())
	t.Cleanup(app.Close)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return app, client
}

func goodClaims(email string) func(string) map[string]any {
	return func(nonce string) map[string]any {
		return map[string]any{
			"iss": "https://accounts.google.com", "aud": "client-id", "exp": time.Now().Add(time.Hour).Unix(),
			"email": email, "email_verified": true, "nonce": nonce,
		}
	}
}

// signIn walks the flow: /auth/login, then the callback Google would call.
func signIn(t *testing.T, app *httptest.Server, c *http.Client, g *fakeGoogle) *http.Response {
	t.Helper()
	resp, err := c.Get(app.URL + "/auth/login?next=studio")
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	q := loc.Query()
	if q.Get("client_id") != "client-id" || q.Get("redirect_uri") != app.URL+"/auth/callback" {
		t.Fatalf("redirection vers Google inattendue : %s", loc)
	}
	g.nonce = q.Get("nonce")
	resp, err = c.Get(app.URL + "/auth/callback?code=the-code&state=" + url.QueryEscape(q.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func status(t *testing.T, c *http.Client, u string) int {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// Only the allowlist ever gets in: with no sign-in configured there is no
// allowlist, so the parent side is closed — and the game still open.
func TestAuthOffClosesTheParentSide(t *testing.T) {
	app, c := newAuthApp(t, nil)
	for _, u := range []string{"/api/admin/summary", "/api/texts", "/admin", "/studio"} {
		if got := status(t, c, app.URL+u); got == http.StatusOK {
			t.Errorf("sans configuration, %s doit être fermé : %d", u, got)
		}
	}
	req, _ := http.NewRequest(http.MethodPost, app.URL+"/api/admin/reset", nil)
	if resp, _ := c.Do(req); resp.StatusCode != http.StatusForbidden {
		t.Errorf("sans configuration, la remise à zéro doit être refusée : %d", resp.StatusCode)
	}
	if got := status(t, c, app.URL+"/api/state"); got != http.StatusOK {
		t.Errorf("le jeu doit rester ouvert : %d", got)
	}
}

func TestAuthGuardsOnlyTheParentSide(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	for _, u := range []string{"/api/admin/summary", "/api/texts"} {
		if got := status(t, c, app.URL+u); got != http.StatusUnauthorized {
			t.Errorf("%s sans connexion : %d, attendu 401", u, got)
		}
	}
	// Not signed in, the page shows a sign-in button — it does not send the
	// browser to Google by itself.
	resp, _ := c.Get(app.URL + "/admin")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("Location") != "" ||
		!strings.Contains(string(body), "auth/login?next=admin") || !strings.Contains(string(body), "Se connecter avec Google") {
		t.Errorf("/admin sans connexion : %d, Location %q — attendu une page avec le bouton", resp.StatusCode, resp.Header.Get("Location"))
	}
	// The child's game is never behind the sign-in.
	for _, u := range []string{"/", "/api/state"} {
		if got := status(t, c, app.URL+u); got != http.StatusOK {
			t.Errorf("le jeu %s ne doit pas demander de connexion : %d", u, got)
		}
	}
}

func TestAllowedEmailSignsIn(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("Papa@example.com")} // case-insensitive
	app, c := newAuthApp(t, g)
	resp := signIn(t, app, c, g)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/studio" {
		t.Fatalf("connexion autorisée : %d → %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if got := status(t, c, app.URL+"/api/admin/summary"); got != http.StatusOK {
		t.Fatalf("connecté, l'admin doit répondre : %d", got)
	}
	var me map[string]any
	r, _ := c.Get(app.URL + "/auth/me")
	_ = json.NewDecoder(r.Body).Decode(&me)
	if me["email"] != "papa@example.com" {
		t.Errorf("auth/me : %v", me)
	}
	if got := status(t, c, app.URL+"/auth/logout"); got != http.StatusFound {
		t.Errorf("déconnexion : %d", got)
	}
	if got := status(t, c, app.URL+"/api/admin/summary"); got != http.StatusUnauthorized {
		t.Errorf("après déconnexion : %d, attendu 401", got)
	}
}

func TestOtherEmailIsRefused(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("intrus@example.com")}
	app, c := newAuthApp(t, g)
	if resp := signIn(t, app, c, g); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("adresse hors liste : %d, attendu 403", resp.StatusCode)
	}
	if got := status(t, c, app.URL+"/api/admin/summary"); got != http.StatusUnauthorized {
		t.Fatalf("une adresse refusée ne doit pas ouvrir l'admin : %d", got)
	}
}

func TestBadTokensAreRefused(t *testing.T) {
	for name, tweak := range map[string]func(map[string]any){
		"audience":     func(c map[string]any) { c["aud"] = "someone-else" },
		"expiré":       func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"nonce":        func(c map[string]any) { c["nonce"] = "replayed" },
		"non vérifiée": func(c map[string]any) { c["email_verified"] = false },
		"émetteur":     func(c map[string]any) { c["iss"] = "https://evil.example" },
	} {
		g := &fakeGoogle{claims: func(nonce string) map[string]any {
			c := goodClaims("papa@example.com")(nonce)
			tweak(c)
			return c
		}}
		app, c := newAuthApp(t, g)
		if resp := signIn(t, app, c, g); resp.StatusCode == http.StatusFound {
			t.Errorf("%s : jeton accepté alors qu'il ne devait pas l'être", name)
		}
	}
}

func TestTamperedSessionCookieIsRefused(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	a, _ := NewAuth("client-id", "secret", "papa@example.com", "", strings.Repeat("x", 40))
	forged := a.issue("session", "papa@example.com", time.Hour) // signed with the wrong key
	u, _ := url.Parse(app.URL)
	c.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: forged, Path: "/"}})
	if got := status(t, c, app.URL+"/api/admin/summary"); got != http.StatusUnauthorized {
		t.Fatalf("cookie forgé accepté : %d", got)
	}
}

func TestLoginAsksForNothingButWhoYouAre(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	resp, _ := c.Get(app.URL + "/auth/login")
	loc, _ := url.Parse(resp.Header.Get("Location"))
	q := loc.Query()
	if q.Get("scope") != "openid email" {
		t.Fatalf("Pokélecture ne doit demander que « openid email », pas %q", q.Get("scope"))
	}
	if q.Get("access_type") != "" || strings.Contains(q.Get("scope"), "offline") {
		t.Fatal("pas d'accès hors ligne : aucun jeton de rafraîchissement ne doit exister")
	}
	if q.Get("state") == "" || q.Get("nonce") == "" {
		t.Fatal("state et nonce sont obligatoires")
	}
}

func TestCrossSiteWritesAreRefused(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("papa@example.com")}
	app, c := newAuthApp(t, g)
	signIn(t, app, c, g)
	post := func(origin string) int {
		req, _ := http.NewRequest(http.MethodPost, app.URL+"/api/admin/name", strings.NewReader(`{"name":"X"}`))
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if got := post("https://evil.example"); got != http.StatusForbidden {
		t.Errorf("écriture depuis un autre site : %d, attendu 403", got)
	}
	if got := post(app.URL); got != http.StatusOK {
		t.Errorf("écriture depuis nos pages : %d, attendu 200", got)
	}
}

func TestParentPagesRefuseToBeFramed(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("papa@example.com")}
	app, c := newAuthApp(t, g)
	signIn(t, app, c, g)
	resp, _ := c.Get(app.URL + "/admin")
	if resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("/admin doit refuser d'être encadrée : %q", resp.Header.Get("X-Frame-Options"))
	}
}

func TestWeakSessionKeyIsRefused(t *testing.T) {
	if _, err := NewAuth("client-id", "secret", "papa@example.com", "", "pikachu"); err == nil {
		t.Fatal("une clé de session courte doit empêcher le démarrage : elle se casse hors ligne")
	}
	if _, err := NewAuth("client-id", "secret", "papa@example.com", "", ""); err != nil {
		t.Fatalf("sans clé, une clé aléatoire forte est tirée : %v", err)
	}
}

// A state cookie (free to any visitor) must never pass for a session.
func TestStateCookieIsNotASession(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	resp, _ := c.Get(app.URL + "/auth/login")
	resp.Body.Close()
	u, _ := url.Parse(app.URL)
	for _, ck := range c.Jar.Cookies(u) {
		if ck.Name == stateCookie {
			c.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: ck.Value, Path: "/"}})
		}
	}
	if got := status(t, c, app.URL+"/api/admin/summary"); got != http.StatusUnauthorized {
		t.Fatalf("cookie d'état présenté comme session : %d, attendu 401", got)
	}
}

// The guarded pages cannot be reached by another method or spelling through
// the static file server, signed in or not.
func TestGuardedPagesHaveNoBackDoor(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	for _, m := range []string{http.MethodPost, http.MethodPut} {
		req, _ := http.NewRequest(m, app.URL+"/admin", nil)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("%s /admin sans connexion : 200", m)
		}
	}
	for _, p := range []string{"/ADMIN", "/Admin", "/admin/", "/Studio", "/ADMIN.HTML", "/Parent.html", "/audio/recorded/x.m4a", "/audio/", "/sprites/"} {
		if got := status(t, c, app.URL+p); got == http.StatusOK {
			t.Errorf("%s sans connexion : 200", p)
		}
	}
}

func TestEmailMatching(t *testing.T) {
	for in, want := range map[string]string{
		"Papa@Example.com":         "papa@example.com",
		"Prenom.Nom@Gmail.com":     "prenomnom@gmail.com",
		"prenom.nom+jeu@gmail.com": "prenomnom@gmail.com",
		"prenomnom@googlemail.com": "prenomnom@gmail.com",
		"prenom.nom@exemple.fr":    "prenom.nom@exemple.fr", // other domains: exact
		// Refused outright: no comparison is even attempted.
		"owner@gma\u0130l.com":     "", // İ would fold onto i
		"\u212Aevin@corp.com":      "", // Kelvin sign would fold onto k
		`"owner@gmail.com"`:        "", // quotes are trimmed on the env var only
		"'bob@corp.com":            "",
		"+x@gmail.com":             "",
		"...@gmail.com":            "",
		"owner@gmail.com@evil.com": "",
		"owner@gmail.com\nforged":  "",
		"a b@gmail.com":            "",
		"@gmail.com":               "",
	} {
		if got := normalizeEmail(in); got != want {
			t.Errorf("normalizeEmail(%q) = %q, attendu %q", in, got, want)
		}
	}
	// A quoted list from a .env still lets its owner in…
	a, err := NewAuth("client-id", "secret", `"Prenom.Nom@gmail.com, maman@example.com"`, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Allowed["prenomnom@gmail.com"] || !a.Allowed["maman@example.com"] {
		t.Fatalf("liste entre guillemets mal lue : %v", a.Allowed)
	}
	// …and a malformed one stops the server instead of failing silently.
	for _, bad := range []string{"papa@example.com; maman@example.com", "papa example.com", "@gmail.com", "*@gmail.com"} {
		if _, err := NewAuth("client-id", "secret", bad, "", ""); err == nil {
			t.Errorf("liste %q acceptée : elle doit empêcher le démarrage", bad)
		}
	}
}

func TestLookAlikeClaimIsRefused(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("papa@examp\u0130e.com")}
	app, c := newAuthApp(t, g)
	if resp := signIn(t, app, c, g); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("adresse unicode sosie : %d, attendu 403", resp.StatusCode)
	}
}

func TestGmailVariantSignsIn(t *testing.T) {
	// Google answers with the account's own spelling; the list has another.
	g := &fakeGoogle{claims: goodClaims("papa@googlemail.com")}
	app, c := newAuthApp(t, g)
	// newAuthApp's list has "Papa@Example.com" and "maman@example.com":
	// a gmail variant of an unlisted address must still be refused.
	if resp := signIn(t, app, c, g); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("adresse hors liste : %d, attendu 403", resp.StatusCode)
	}
}

func TestSignedInGoesStraightToTheAdmin(t *testing.T) {
	g := &fakeGoogle{claims: goodClaims("papa@example.com")}
	app, c := newAuthApp(t, g)
	signIn(t, app, c, g)
	resp, _ := c.Get(app.URL + "/admin")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.Contains(string(body), "Se connecter avec Google") {
		t.Fatalf("connecté, /admin doit s'ouvrir directement : %d", resp.StatusCode)
	}
}

// The old .html addresses only redirect — they never show the page itself.
func TestOldPageAddressesRedirect(t *testing.T) {
	app, c := newAuthApp(t, &fakeGoogle{claims: goodClaims("papa@example.com")})
	for old, now := range map[string]string{"/admin.html": "/admin", "/parent.html": "/studio"} {
		resp, _ := c.Get(app.URL + old)
		resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != now {
			t.Errorf("%s : %d → %q, attendu 301 → %s", old, resp.StatusCode, resp.Header.Get("Location"), now)
		}
		req, _ := http.NewRequest(http.MethodPost, app.URL+old, nil)
		if resp, _ := c.Do(req); resp.StatusCode == http.StatusOK {
			t.Errorf("POST %s : 200", old)
		}
	}
}
