package server

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lupguo/landrop/internal/auth"
)

const (
	csp           = "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'"
	maxNameRunes  = 40
	maxUARunes    = 200
	unknownDevice = "Unknown device"
	nameHeader    = "X-LanDrop-Name"
)

type ctxKey struct{}

func sessionFrom(r *http.Request) auth.Session {
	s, _ := r.Context().Value(ctxKey{}).(auth.Session)
	return s
}

func isLoopback(r *http.Request) bool {
	ip := net.ParseIP(remoteIP(r))
	return ip != nil && ip.IsLoopback()
}

// remoteIP is the host part of RemoteAddr.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func setSessionCookie(w http.ResponseWriter, sess auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    sess.ID,
		Path:     "/",
		MaxAge:   auth.CookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// lookupSession returns the session named by the request cookie, if valid.
func (s *server) lookupSession(r *http.Request) (auth.Session, bool) {
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return auth.Session{}, false
	}
	return s.Auth.Lookup(c.Value)
}

// session requires a valid session (401 otherwise). In NoAuth mode a missing
// or invalid cookie silently gets a fresh session and cookie.
func (s *server) session(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := s.lookupSession(r)
		if !ok {
			if !s.NoAuth {
				writeErr(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			sess = s.Auth.NewSession()
			setSessionCookie(w, sess)
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, sess)))
	}
}

// origin requires the Origin host:port to equal the request Host.
func (s *server) origin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		o := r.Header.Get("Origin")
		u, err := url.Parse(o)
		if o == "" || err != nil || u.Host == "" || u.Host != r.Host {
			writeErr(w, http.StatusForbidden, "bad_origin")
			return
		}
		next(w, r)
	}
}

// hostOnly restricts the route to the host role.
func (s *server) hostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.IsHost(r) {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

// deviceName resolves the requester's display name from raw (header or
// query value), falling back to Hostname for the host and "Unknown device".
func (s *server) deviceName(r *http.Request, raw string) string {
	if dec, err := url.PathUnescape(raw); err == nil {
		raw = dec
	}
	name := truncateRunes(strings.TrimSpace(stripControl(raw)), maxNameRunes)
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	if s.IsHost(r) && s.Hostname != "" {
		return s.Hostname
	}
	return unknownDevice
}

func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// securityHeaders sets nosniff and no-referrer on every response, plus CSP on
// HTML responses.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(&secWriter{ResponseWriter: w}, r)
	})
}

type secWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *secWriter) WriteHeader(code int) {
	if !w.wrote {
		w.wrote = true
		if strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			w.Header().Set("Content-Security-Policy", csp)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *secWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(p))
		}
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach Flush etc.
func (w *secWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *secWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	http.NewResponseController(w.ResponseWriter).Flush()
}
