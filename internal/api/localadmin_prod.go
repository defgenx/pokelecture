//go:build !dev

package api

import "net/http"

// DevBuild: this binary was built without -tags dev — the production build.
// The parent side then always needs Google sign-in (localadmin.go is not
// compiled in).
const DevBuild = false

// WithLocalAdmin does nothing outside a dev build.
func (s *Server) WithLocalAdmin(bool) *Server { return s }

// LocalAdminKey is always empty outside a dev build.
func (s *Server) LocalAdminKey() string { return "" }

func (s *Server) devAdmin(http.ResponseWriter, *http.Request) bool { return false }
