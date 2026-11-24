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
