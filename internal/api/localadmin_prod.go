//go:build !dev

package api

import "net/http"

// DevBuild: this binary was built without -tags dev — the production build.
// The parent side then always needs Google sign-in (localadmin.go is not
// compiled in).
const DevBuild = false

// WithLocalAdmin does nothing outside a dev build.
func (s *Server) WithLocalAdmin(bool) *Server { return s }

func fromThisComputer(*http.Request) bool { return false }
