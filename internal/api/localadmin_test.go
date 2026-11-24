package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adelvecchio/pokelecture/internal/curriculum"
	"github.com/adelvecchio/pokelecture/internal/progress"
)

func localAdminApp(t *testing.T, on bool) *httptest.Server {
	cur, _ := curriculum.Load(filepath.Join("..", "..", "data"))
	reg, _ := progress.OpenRegistry(t.TempDir(), "Léo")
	app := httptest.NewServer(NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").WithLocalAdmin(on).Routes())
	t.Cleanup(app.Close)
	return app
}

func do(t *testing.T, method, u string, hdr map[string]string) int {
	req, _ := http.NewRequest(method, u, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestLocalAdminOnlyOnThisComputer(t *testing.T) {
	if got := do(t, "GET", localAdminApp(t, false).URL+"/api/admin/summary", nil); got != http.StatusForbidden {
		t.Fatalf("sans -local-admin, l'admin reste fermée : %d", got)
	}
	app := localAdminApp(t, true)
	if got := do(t, "GET", app.URL+"/api/admin/summary", nil); got != http.StatusOK {
		t.Fatalf("-local-admin depuis localhost : %d", got)
	}
	if got := do(t, "GET", app.URL+"/admin", nil); got != http.StatusOK {
		t.Fatalf("page /admin depuis localhost : %d", got)
	}
	if got := do(t, "POST", app.URL+"/api/admin/name", map[string]string{"Origin": app.URL}); got == http.StatusForbidden || got == http.StatusUnauthorized {
		t.Fatalf("écriture depuis nos pages : %d", got)
	}
	for name, hdr := range map[string]map[string]string{
		"autre origine":          {"Origin": "https://evil.example"},
		"derrière un proxy":      {"X-Forwarded-For": "203.0.113.9"},
		"proxy (Forwarded)":      {"Forwarded": "for=203.0.113.9"},
		"rebinding DNS (Host)":   {"Host": "evil.example"},
		"proxy (X-Real-IP)":      {"X-Real-IP": "203.0.113.9"},
		"proxy (Forwarded-Host)": {"X-Forwarded-Host": "delvecch.io"},
	} {
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
