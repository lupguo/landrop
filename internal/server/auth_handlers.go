package server

import (
	"errors"
	"net/http"

	"github.com/lupguo/landrop/internal/auth"
)

// handleRoot logs in via GET /?t=<token>; everything else goes to Deps.Web.
func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		if t := r.URL.Query().Get("t"); t != "" && s.Auth.CheckToken(t) {
			setSessionCookie(w, s.Auth.NewSession())
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	s.Web.ServeHTTP(w, r)
}

// handleAuth checks the access code: 204 + cookie, 401 bad_code, 429 locked.
func (s *server) handleAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, 4096, &body) {
		return
	}
	if s.NoAuth {
		if _, ok := s.lookupSession(r); !ok {
			setSessionCookie(w, s.Auth.NewSession())
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch err := s.Auth.CheckCode(remoteIP(r), body.Code); {
	case err == nil:
		setSessionCookie(w, s.Auth.NewSession())
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, auth.ErrLocked):
		writeErr(w, http.StatusTooManyRequests, "locked")
	default:
		writeErr(w, http.StatusUnauthorized, "bad_code")
	}
}

// handleMe describes the caller and server settings. It also carries what the
// terminal banner shows, so any member can invite others: the access code and
// the token link encoded in the QR code (scanning it skips the code).
func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	role := "peer"
	if s.IsHost(r) {
		role = "host"
	}
	code, joinURL := s.Auth.Code(), s.PublicBase+"/?t="+s.Auth.Token()
	if s.NoAuth {
		code, joinURL = "", s.PublicBase
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"peer_id":    sessionFrom(r).PeerID,
		"role":       role,
		"no_auth":    s.NoAuth,
		"public_url": s.PublicBase,
		"mdns_url":   s.MDNSURL,
		"has_dir":    s.Dir != "",
		"max_size":   s.MaxSize,
		"code":       code,
		"join_url":   joinURL,
	})
}
