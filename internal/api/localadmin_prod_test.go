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
	app := localAdminApp(t, true)
	for _, u := range []string{"/api/admin/summary", "/admin", "/admin/studio", "/api/texts"} {
		if got := do(t, "GET", app.URL+u, nil); got == http.StatusOK {
			t.Errorf("%s ouvert sans connexion dans la build de production : %d", u, got)
		}
	}
}
