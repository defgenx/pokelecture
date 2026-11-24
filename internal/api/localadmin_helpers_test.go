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

// localAdminApp serves a test app; key is its -local-admin key ("" when off
// or in the production build).
func localAdminApp(t *testing.T, on bool) (app *httptest.Server, key string) {
	cur, _ := curriculum.Load(filepath.Join("..", "..", "data"))
	reg, _ := progress.OpenRegistry(t.TempDir(), "Léo")
	srv := NewPlayers(cur, reg, filepath.Join("..", "..", "web"), "/").WithLocalAdmin(on)
	app = httptest.NewServer(srv.Routes())
	t.Cleanup(app.Close)
	return app, srv.LocalAdminKey()
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
