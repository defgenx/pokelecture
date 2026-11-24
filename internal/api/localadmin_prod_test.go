//go:build !dev

package api

import (
	"net/http"
	"testing"
)

// The production build has no bypass: even asked for, from localhost, the
// admin stays closed without Google sign-in.
func TestNoLocalAdminInProductionBuild(t *testing.T) {
	if DevBuild {
		t.Fatal("ce test tourne sans -tags dev")
	}
	app, key := localAdminApp(t, true)
	if key != "" {
		t.Fatal("pas de clé d'admin locale dans la build de production")
	}
	for _, u := range []string{"/api/admin/summary", "/admin", "/admin/studio", "/api/texts"} {
		if got := do(t, "GET", app.URL+u+"?k=x", map[string]string{"Cookie": "pk_dev_admin=x"}); got == http.StatusOK {
			t.Errorf("%s ouvert sans connexion dans la build de production : %d", u, got)
		}
	}
}
