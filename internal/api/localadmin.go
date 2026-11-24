//go:build dev

package api

// Local admin: `make run` (a `-tags dev` build) opens the parent side without
// Google sign-in, to the computer the server runs on only. A build without
// the tag — the Docker image — has no bypass at all (localadmin_prod.go).
// README, « Administration ».

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DevBuild: this binary was built with -tags dev.
const DevBuild = true

// WithLocalAdmin lets requests from this computer into the parent side
// without signing in. Only the -local-admin flag turns it on — never an
// environment variable, which a deployment's env_file could carry.
func (s *Server) WithLocalAdmin(on bool) *Server {
	s.localAdmin = on
	return s
}

// fromThisComputer: a direct loopback connection, addressed to a loopback
// name, with no proxy in between. A reverse proxy on the same host connects
// from loopback too, so any forwarding header disqualifies the request; the
// Host check stops a DNS-rebinding page from posing as localhost.
func fromThisComputer(r *http.Request) bool {
	for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip", "Forwarded"} {
		if r.Header.Get(h) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.Unmap().IsLoopback() {
		return false
	}
	name := r.Host
	if h, _, err := net.SplitHostPort(name); err == nil {
		name = h
	}
	name = strings.Trim(strings.ToLower(name), "[]")
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}
