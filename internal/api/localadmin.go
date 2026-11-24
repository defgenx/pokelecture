//go:build dev

package api

// Local admin: `make run` (a `-tags dev` build) opens the parent side without
// Google sign-in, to the computer the server runs on, holding the key in the
// link printed at startup. A build without the tag — the Docker image — has
// no bypass at all (localadmin_prod.go). README, « Administration ».

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// DevBuild: this binary was built with -tags dev.
const DevBuild = true

const devAdminCookie = "pk_dev_admin"

// WithLocalAdmin lets this computer into the parent side without signing in.
// Only the -local-admin flag turns it on — never an environment variable,
// which a deployment's env_file could carry. The key is derived from the
// device key in var/players, so it survives restarts.
func (s *Server) WithLocalAdmin(on bool) *Server {
	s.localAdmin = ""
	if on && s.players != nil {
		m := hmac.New(sha256.New, s.players.Key())
		m.Write([]byte("local-admin"))
		s.localAdmin = base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	}
	return s
}

// LocalAdminKey is the ?k= of the link printed at startup.
func (s *Server) LocalAdminKey() string { return s.localAdmin }

// devAdmin: this computer, with this run's key — from the startup link
// (?k=, then kept in a cookie). The key is what stops a tunnel or port
// forward (ssh -R, socat…) that delivers outside traffic over loopback.
func (s *Server) devAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.localAdmin == "" || !fromThisComputer(r) {
		return false
	}
	key := []byte(s.localAdmin)
	if c, err := r.Cookie(devAdminCookie); err == nil && subtle.ConstantTimeCompare([]byte(c.Value), key) == 1 {
		return true
	}
	if k := r.URL.Query().Get("k"); k != "" && subtle.ConstantTimeCompare([]byte(k), key) == 1 {
		http.SetCookie(w, &http.Cookie{Name: devAdminCookie, Value: s.localAdmin, Path: s.base,
			HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 3600})
		return true
	}
	return false
}

// fromThisComputer: a direct loopback connection, addressed to a loopback
// name, with no proxy in between. A reverse proxy on the same host connects
// from loopback too, so any forwarding header disqualifies the request; the
// Host check stops a DNS-rebinding page from posing as localhost.
func fromThisComputer(r *http.Request) bool {
	for _, h := range []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Port",
		"X-Forwarded-Server", "X-Real-Ip", "Forwarded", "Via", "Cf-Connecting-Ip", "True-Client-Ip"} {
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
