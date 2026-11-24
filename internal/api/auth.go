package api

// "Se connecter avec Google" for the parent side — the admin page, the voice
// studio and their endpoints — with an allowlist of email addresses. The
// child's game is never behind it. Standard library only: the OAuth 2.0
// authorization-code flow, the ID token taken straight from Google's token
// endpoint over TLS (so its signature need not be checked, per Google), and a
// signed session cookie. Only the addresses of POKELECTURE_ADMINS ever get
// in: not configured (no client id), the parent side is closed to everyone.
// Setup and the reasons: README, « Se connecter avec Google ».

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Auth is the Google sign-in configuration. A nil *Auth means "closed".
type Auth struct {
	ClientID     string
	ClientSecret string
	Allowed      map[string]bool // lowercased email addresses
	PublicURL    string          // optional, e.g. https://delvecch.io/pokelecture/
	key          []byte          // signs the cookies

	// Google's endpoints; overridden by the tests.
	authURL  string
	tokenURL string
	client   *http.Client
}

// authScope is everything Pokélecture asks Google for: who you are, your
// address. No Gmail, no Drive, no offline access, no refresh token — so
// nothing this server holds can act on the Google account.
const authScope = "openid email"

const (
	sessionCookie = "pk_admin"
	stateCookie   = "pk_oauth"
	sessionTTL    = 30 * 24 * time.Hour
	minSessionKey = 32
	stateTTL      = 10 * time.Minute
)

// NewAuth returns nil when clientID is empty: sign-in is off. admins is a
// comma-separated allowlist; sessionKey signs cookies (random when empty, so
// sessions then end at each restart).
func NewAuth(clientID, clientSecret, admins, publicURL, sessionKey string) (*Auth, error) {
	if clientID == "" {
		return nil, nil
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET manquant")
	}
	a := &Auth{
		ClientID: clientID, ClientSecret: clientSecret, Allowed: map[string]bool{},
		authURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		tokenURL: "https://oauth2.googleapis.com/token",
		client:   &http.Client{Timeout: 15 * time.Second},
	}
	for _, e := range strings.Split(strings.Trim(strings.TrimSpace(admins), `"'`), ",") {
		// Quotes and spaces are trimmed here, on the env var only — never on
		// what Google answers.
		e = strings.Trim(strings.TrimSpace(e), `"' `)
		if e == "" {
			continue
		}
		n := normalizeEmail(e)
		if n == "" || strings.Contains(e, "*") { // no wildcards: a list of real addresses
			return nil, fmt.Errorf("POKELECTURE_ADMINS : adresse invalide %q (une adresse par élément, séparées par des virgules, sans *)", e)
		}
		a.Allowed[n] = true
	}
	if len(a.Allowed) == 0 {
		return nil, fmt.Errorf("POKELECTURE_ADMINS est vide : personne ne pourrait se connecter")
	}
	if publicURL != "" && !strings.HasSuffix(publicURL, "/") {
		publicURL += "/"
	}
	a.PublicURL = publicURL
	switch {
	case sessionKey != "" && len(sessionKey) < minSessionKey:
		// Every visitor gets a signed state cookie: a short key can be cracked
		// offline from it, and then any session forged.
		return nil, fmt.Errorf("POKELECTURE_SESSION_KEY trop courte (%d caractères, il en faut %d) : openssl rand -hex 32", len(sessionKey), minSessionKey)
	case sessionKey != "":
		a.key = []byte(sessionKey)
	default:
		a.key = make([]byte, 32)
		if _, err := rand.Read(a.key); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// validEmail is the only shape of address compared at all: printable ASCII
// with no quote, space or control character, exactly one "@", a non-empty
// name and domain. Anything else — unicode look-alikes that case-fold onto
// an allowed address (İ → i, the Kelvin sign → k), quotes, line breaks — is
// refused, from Google as from the allowlist.
func validEmail(e string) bool {
	if strings.Count(e, "@") != 1 {
		return false
	}
	for i := 0; i < len(e); i++ {
		c := e[i]
		if c <= ' ' || c >= 0x7f || c == '"' || c == '\'' || c == '`' || c == '\\' {
			return false
		}
	}
	name, domain, _ := strings.Cut(e, "@")
	return name != "" && domain != "" && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// normalizeEmail is how both sides are compared, "" for an invalid address:
// lowercased (plain ASCII only, see validEmail) and, for Gmail only, the way
// Gmail itself reads an address — dots and a +tag in the name do not count,
// googlemail.com is gmail.com. Every other domain is compared exactly.
func normalizeEmail(e string) string {
	if !validEmail(e) {
		return ""
	}
	e = strings.ToLower(e)
	name, domain, _ := strings.Cut(e, "@")
	if domain == "googlemail.com" {
		domain = "gmail.com"
	}
	if domain == "gmail.com" {
		name, _, _ = strings.Cut(name, "+")
		name = strings.ReplaceAll(name, ".", "")
		if name == "" {
			return ""
		}
	}
	return name + "@" + domain
}

// WithAuth turns the Google sign-in on for the parent side.
func (s *Server) WithAuth(a *Auth) *Server {
	s.auth = a
	return s
}

/* -------------------------------------------------------------- cookies --- */

func (a *Auth) sign(v string) string {
	m := hmac.New(sha256.New, a.key)
	m.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString([]byte(v)) + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (a *Auth) verify(signed string) (string, bool) {
	body, mac, ok := strings.Cut(signed, ".")
	if !ok {
		return "", false
	}
	v, err1 := base64.RawURLEncoding.DecodeString(body)
	got, err2 := base64.RawURLEncoding.DecodeString(mac)
	if err1 != nil || err2 != nil {
		return "", false
	}
	m := hmac.New(sha256.New, a.key)
	m.Write(v)
	if !hmac.Equal(got, m.Sum(nil)) {
		return "", false
	}
	return string(v), true
}

// A signed value is "<purpose>|<fields…>|<unix expiry>": the purpose keeps a
// state cookie from ever passing for a session; expired values do not open.
func (a *Auth) issue(purpose, fields string, ttl time.Duration) string {
	return a.sign(purpose + "|" + fields + "|" + strconv.FormatInt(time.Now().Add(ttl).Unix(), 10))
}

func (a *Auth) open(purpose, signed string) ([]string, bool) {
	v, ok := a.verify(signed)
	if !ok {
		return nil, false
	}
	v, ok = strings.CutPrefix(v, purpose+"|")
	if !ok {
		return nil, false
	}
	parts := strings.Split(v, "|")
	exp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return nil, false
	}
	return parts[:len(parts)-1], true
}

// email returns the signed-in, still-allowed address, or "".
func (s *Server) email(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	f, ok := s.auth.open("session", c.Value)
	if !ok || len(f) != 1 || !s.auth.Allowed[f[0]] {
		return "" // removing an address from the allowlist revokes it at once
	}
	return f[0]
}

func (s *Server) cookie(r *http.Request, name, value string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name: name, Value: value, Path: s.base, HttpOnly: true,
		Secure: s.scheme(r) == "https", SameSite: http.SameSiteLaxMode,
		MaxAge: int(ttl.Seconds()),
	}
}

/* ---------------------------------------------------------------- guard --- */

// guard protects a parent-side handler: pages redirect to the sign-in, API
// calls answer 401 (the page then sends the browser to the sign-in). Signed
// in, a write must still come from our own pages (Origin check, on top of
// the SameSite cookie), and the pages refuse to be framed by another site.
func (s *Server) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			// No sign-in configured means no allowlist: nobody gets in.
			if isParentPage(r) {
				s.authPage(w, http.StatusServiceUnavailable,
					"L'administration est fermée : configure la connexion Google (GOOGLE_CLIENT_ID et POKELECTURE_ADMINS).", "")
				return
			}
			http.Error(w, "administration fermée : connexion Google non configurée", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		if s.email(r) != "" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead && !s.sameOrigin(r) {
				http.Error(w, "requête d'une autre origine refusée", http.StatusForbidden)
				return
			}
			h(w, r)
			return
		}
		if isParentPage(r) {
			// Not signed in: a page with a sign-in button — never a jump
			// straight to Google. Signed in, the guard above lets it through.
			s.signInPage(w, safeNext(strings.TrimPrefix(r.URL.Path, "/")))
			return
		}
		http.Error(w, "connexion requise", http.StatusUnauthorized)
	}
}

/* ---------------------------------------------------------------- flow --- */

// sameOrigin reports whether a write comes from our own pages: its Origin
// (or, failing that, Referer) must be this server. A request carrying
// neither is a tool, not a browser on another site, and is let through.
func (s *Server) sameOrigin(r *http.Request) bool {
	src := r.Header.Get("Origin")
	if src == "" {
		src = r.Header.Get("Referer")
	}
	if src == "" {
		return true
	}
	u, err := url.Parse(src)
	if err != nil {
		return false
	}
	if s.auth.PublicURL != "" {
		pub, err := url.Parse(s.auth.PublicURL)
		return err == nil && strings.EqualFold(u.Host, pub.Host) && u.Scheme == pub.Scheme
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return strings.EqualFold(u.Host, host)
}

// scheme: from POKELECTURE_URL when set — then no client-sent header is
// trusted — else from the proxy's header or the connection.
func (s *Server) scheme(r *http.Request) string {
	if s.auth != nil && s.auth.PublicURL != "" {
		if u, err := url.Parse(s.auth.PublicURL); err == nil && u.Scheme != "" {
			return u.Scheme
		}
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return strings.ToLower(strings.TrimSpace(strings.Split(p, ",")[0]))
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// callbackURL must match, character for character, one of the redirect URIs
// registered for the client in the Google Cloud console.
func (s *Server) callbackURL(r *http.Request) string {
	if s.auth.PublicURL != "" {
		return s.auth.PublicURL + "auth/callback"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return s.scheme(r) + "://" + host + s.base + "auth/callback"
}

// isParentPage tells a parent page (sign-in screen if needed) from its API
// calls (a bare 401).
func isParentPage(r *http.Request) bool {
	_, ok := parentPages[strings.TrimPrefix(r.URL.Path, "/")]
	return ok
}

// safeNext keeps the post-sign-in redirect inside the app: one of the parent
// pages, nothing else.
func safeNext(next string) string {
	if _, ok := parentPages[next]; ok {
		return next
	}
	return "admin"
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		s.authPage(w, http.StatusServiceUnavailable,
			"La connexion Google n'est pas configurée (GOOGLE_CLIENT_ID et POKELECTURE_ADMINS).", "")
		return
	}
	state, nonce := randomHex(16), randomHex(16)
	next := safeNext(r.URL.Query().Get("next"))
	http.SetCookie(w, s.cookie(r, stateCookie, s.auth.issue("state", state+"|"+nonce+"|"+next, stateTTL), stateTTL))
	q := url.Values{
		"client_id":     {s.auth.ClientID},
		"redirect_uri":  {s.callbackURL(r)},
		"response_type": {"code"},
		"scope":         {authScope},
		"state":         {state},
		"nonce":         {nonce},
		"prompt":        {"select_account"},
	}
	http.Redirect(w, r, s.auth.authURL+"?"+q.Encode(), http.StatusFound)
}

type idClaims struct {
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Exp           int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Nonce         string `json:"nonce"`
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		http.NotFound(w, r)
		return
	}
	c, err := r.Cookie(stateCookie)
	f, ok := []string(nil), false
	if err == nil {
		f, ok = s.auth.open("state", c.Value)
	}
	if !ok || len(f) != 3 || r.URL.Query().Get("state") != f[0] {
		s.authPage(w, http.StatusBadRequest, "La connexion a expiré. Réessaie.", "")
		return
	}
	nonce, next := f[1], f[2]
	http.SetCookie(w, s.cookie(r, stateCookie, "", -time.Second))
	if e := r.URL.Query().Get("error"); e != "" {
		s.authPage(w, http.StatusUnauthorized, "Connexion annulée.", "")
		return
	}

	claims, err := s.exchange(r, r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("connexion Google : %v", err)
		s.authPage(w, http.StatusBadGateway, "Google n'a pas répondu comme prévu. Réessaie.", "")
		return
	}
	switch {
	case claims.Iss != "accounts.google.com" && claims.Iss != "https://accounts.google.com",
		claims.Aud != s.auth.ClientID, time.Now().Unix() > claims.Exp, claims.Nonce != nonce:
		s.authPage(w, http.StatusUnauthorized, "Jeton Google invalide.", "")
		return
	case !claims.EmailVerified:
		s.authPage(w, http.StatusForbidden, "Cette adresse Google n'est pas vérifiée.", claims.Email)
		return
	}
	email := normalizeEmail(claims.Email)
	if email == "" || !s.auth.Allowed[email] {
		log.Printf("connexion refusée : %q (lu %q) n'est pas dans POKELECTURE_ADMINS (%d adresse(s))", claims.Email, email, len(s.auth.Allowed))
		s.authPage(w, http.StatusForbidden,
			"Ce compte Google n'est pas autorisé : ajoute exactement cette adresse à POKELECTURE_ADMINS.", claims.Email)
		return
	}
	http.SetCookie(w, s.cookie(r, sessionCookie, s.auth.issue("session", email, sessionTTL), sessionTTL))
	http.Redirect(w, r, s.base+next, http.StatusFound)
}

// exchange trades the code for tokens at Google's token endpoint and reads
// the ID token. Received directly from Google over TLS, it needs no
// signature check; its claims are still checked by the caller.
func (s *Server) exchange(r *http.Request, code string) (idClaims, error) {
	var claims idClaims
	resp, err := s.auth.client.PostForm(s.auth.tokenURL, url.Values{
		"code":          {code},
		"client_id":     {s.auth.ClientID},
		"client_secret": {s.auth.ClientSecret},
		"redirect_uri":  {s.callbackURL(r)},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return claims, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return claims, fmt.Errorf("token: %s: %s", resp.Status, b)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(b, &tok); err != nil {
		return claims, err
	}
	parts := strings.Split(tok.IDToken, ".")
	if len(parts) != 3 {
		return claims, fmt.Errorf("id_token malformé")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, err
	}
	return claims, json.Unmarshal(payload, &claims)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, s.cookie(r, sessionCookie, "", -time.Second))
	http.Redirect(w, r, s.base+"admin", http.StatusFound)
}

// handleMe tells the parent pages who is signed in.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": true, "email": s.email(r)})
}

// signInPage is what a parent page shows to someone not signed in.
func (s *Server) signInPage(w http.ResponseWriter, next string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	title := "Espace parents"
	if next == "studio" {
		title = "Studio voix"
	}
	fmt.Fprintf(w, `<!doctype html><html lang="fr"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pokélecture — %[1]s</title>
<body style="font-family:system-ui,sans-serif;max-width:30em;margin:14vh auto;padding:0 16px;text-align:center;background:#eef3fb;color:#1a1d24">
<h1 style="margin-bottom:.2em">Pokélecture</h1>
<p style="font-size:1.25em;font-weight:700;margin-top:0">🔒 %[1]s</p>
<p style="color:#4d5566">Réservé aux parents : connecte-toi avec un compte Google autorisé.</p>
<p><a href="%[2]sauth/login?next=%[3]s" style="display:inline-flex;gap:10px;align-items:center;padding:14px 24px;background:#fff;color:#1a1d24;border:1px solid #c6ccd6;border-radius:12px;text-decoration:none;font-weight:700;font-size:1.05em;box-shadow:0 2px 6px rgba(0,0,0,.08)">Se connecter avec Google</a></p>
<p><a href="%[2]s" style="color:#3b4cca;font-weight:600">← Retour au jeu</a></p>
</body></html>`, html.EscapeString(title), s.base, url.QueryEscape(next))
}

func (s *Server) authPage(w http.ResponseWriter, code int, msg, email string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	who := ""
	if email != "" {
		who = "<p><b>" + html.EscapeString(email) + "</b></p>"
	}
	fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pokélecture — connexion</title>
<body style="font-family:system-ui,sans-serif;max-width:32em;margin:15vh auto;padding:0 16px;text-align:center;background:#eef3fb;color:#1a1d24">
<h1>Pokélecture</h1><p>%s</p>%s
<p><a href="%[3]sauth/login" style="display:inline-block;padding:12px 22px;background:#3b4cca;color:#fff;border-radius:12px;text-decoration:none;font-weight:700">Se connecter avec un autre compte Google</a></p>
<p><a href="%[3]s" style="color:#3b4cca;font-weight:600">← Retour au jeu</a></p>
</body>`, html.EscapeString(msg), who, s.base)
}
