// Package server is LanDrop's HTTP API: auth, items, text, SSE and peers,
// wrapped in session, origin, host-role and security-header middleware.
package server

import (
	"net/http"
	"time"

	"github.com/lupguo/landrop/internal/auth"
	"github.com/lupguo/landrop/internal/hub"
	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
)

// Deps are the server's collaborators and settings.
type Deps struct {
	Store *store.Store
	Hub   *hub.Hub
	Auth  *auth.Manager
	Share *share.Manager

	NoAuth      bool
	MaxSize     int64
	PublicBase  string // e.g. "http://192.168.1.23:8848"
	MDNSURL     string
	Dir         string // --dir root or ""
	LocalSecret string
	Hostname    string

	OnFileStored func(store.Item) // autosave hook, may be nil
	Web          http.Handler
	Now          func() time.Time
	IsHost       func(*http.Request) bool // nil → RemoteAddr is loopback
}

type server struct {
	Deps
}

// New builds the LanDrop HTTP handler.
func New(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.IsHost == nil {
		d.IsHost = isLoopback
	}
	if d.Web == nil {
		d.Web = http.NotFoundHandler()
	}
	s := &server{Deps: d}

	// Middleware: s.session (valid session required), s.origin (same-origin
	// writes), s.hostOnly (loopback host role).
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth", s.origin(s.handleAuth))
	mux.HandleFunc("GET /api/me", s.session(s.handleMe))
	mux.HandleFunc("GET /api/items", s.session(s.handleListItems))
	mux.HandleFunc("POST /api/text", s.session(s.origin(s.handlePostText)))
	mux.HandleFunc("POST /api/upload", s.session(s.origin(s.handleUpload)))
	mux.HandleFunc("GET /api/items/{id}/raw", s.session(s.handleRaw))
	mux.HandleFunc("DELETE /api/items/{id}", s.session(s.origin(s.handleDeleteItem)))
	mux.HandleFunc("DELETE /api/items", s.session(s.origin(s.handleClearItems)))
	mux.HandleFunc("POST /api/items/{id}/share", s.session(s.origin(s.handleCreateShare)))
	mux.HandleFunc("GET /api/qr", s.session(s.handleQR))
	mux.HandleFunc("GET /api/s/{id}", s.handleShareMeta)
	mux.HandleFunc("GET /s/{id}", s.handleSharePage)
	mux.HandleFunc("GET /s/{id}/raw", s.handleShareRaw)
	mux.HandleFunc("GET /events", s.session(s.handleEvents))
	mux.HandleFunc("GET /api/peers", s.session(s.handlePeers))
	mux.HandleFunc("DELETE /api/peers/{peer_id}", s.session(s.origin(s.hostOnly(s.handleKick))))
	mux.HandleFunc("/", s.handleRoot) // GET /?t= token login, else Deps.Web

	return securityHeaders(mux)
}
