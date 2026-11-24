//go:build dev

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalAdminOnlyOnThisComputer(t *testing.T) {
	off, _ := localAdminApp(t, false)
	if got := do(t, "GET", off.URL+"/api/admin/summary", nil); got != http.StatusForbidden {
		t.Fatalf("sans -local-admin, l'admin reste fermée : %d", got)
	}
	app, key := localAdminApp(t, true)
	if len(key) < 40 {
		t.Fatalf("clé trop courte : %q", key)
	}
	// Loopback alone is not enough: a tunnel (ssh -R, socat) arrives that way too.
	if got := do(t, "GET", app.URL+"/api/admin/summary", nil); got == http.StatusOK {
		t.Fatalf("sans la clé, même depuis localhost, l'admin reste fermée : %d", got)
	}
	if got := do(t, "GET", app.URL+"/api/admin/summary?k=wrong", nil); got == http.StatusOK {
		t.Fatalf("mauvaise clé : %d", got)
	}
	// The startup link sets the cookie; then the cookie is enough.
	req, _ := http.NewRequest("GET", app.URL+"/admin?k="+key, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var cookie string
	for _, c := range resp.Cookies() {
		if c.Name == devAdminCookie {
			cookie = c.Name + "=" + c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Errorf("cookie d'admin locale : HttpOnly et SameSite=Strict attendus")
			}
		}
	}
	if resp.StatusCode != http.StatusOK || cookie == "" {
		t.Fatalf("lien de démarrage : %d, cookie %q", resp.StatusCode, cookie)
	}
	ok := map[string]string{"Cookie": cookie}
	if got := do(t, "GET", app.URL+"/api/admin/summary", ok); got != http.StatusOK {
		t.Fatalf("avec le cookie : %d", got)
	}
	if got := do(t, "POST", app.URL+"/api/admin/name", map[string]string{"Cookie": cookie, "Origin": app.URL}); got == http.StatusForbidden || got == http.StatusUnauthorized {
		t.Fatalf("écriture depuis nos pages : %d", got)
	}
	for name, hdr := range map[string]map[string]string{
		"autre origine":          {"Origin": "https://evil.example"},
		"derrière un proxy":      {"X-Forwarded-For": "203.0.113.9"},
		"proxy (Forwarded)":      {"Forwarded": "for=203.0.113.9"},
		"rebinding DNS (Host)":   {"Host": "evil.example"},
		"proxy (X-Real-IP)":      {"X-Real-IP": "203.0.113.9"},
		"proxy (Forwarded-Host)": {"X-Forwarded-Host": "delvecch.io"},
		"proxy (Proto)":          {"X-Forwarded-Proto": "https"},
		"proxy (Via)":            {"Via": "1.1 nginx"},
		"proxy (Cloudflare)":     {"CF-Connecting-IP": "203.0.113.9"},
	} {
		hdr["Cookie"] = cookie // even holding the key
		method := "GET"
		if hdr["Origin"] != "" {
			method = "POST"
		}
		if got := do(t, method, app.URL+"/api/admin/summary", hdr); got == http.StatusOK {
			t.Errorf("%s : l'admin ne doit pas s'ouvrir (%d)", name, got)
		}
	}
}

func TestFromThisComputer(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:5000": true, "[::1]:5000": true, "[::ffff:127.0.0.1]:5000": true,
		"192.168.1.20:5000": false, "10.0.0.2:5000": false, "172.18.0.3:5000": false,
	} {
		r := httptest.NewRequest("GET", "http://localhost:8080/admin", nil)
		r.RemoteAddr = addr
		if got := fromThisComputer(r); got != want {
			t.Errorf("%s : %v, attendu %v", addr, got, want)
		}
	}
}
