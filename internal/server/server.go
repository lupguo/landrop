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
	MDNSName    string // mDNS instance name (e.g. "drop"); "" → host of MDNSURL
	Dir         string // --dir root or ""
	LocalSecret string
	Hostname    string

	// OnFileStored is the autosave hook, may be nil. It is called
	// synchronously after a file item is stored and before the response is
	// written, so it must not block: do slow work in a goroutine.
	OnFileStored func(store.Item)
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

	// Middleware: s.hostCheck (NoAuth only: Host allowlist against DNS
	// rebinding, wraps everything), s.origin (same-origin writes; always
	// outside s.session so a cross-origin write never creates a NoAuth
	// session), s.session (valid session required), s.hostOnly (loopback host
	// role), s.local (loopback + instance secret, for `landrop send`).
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth", s.origin(s.handleAuth))
	mux.HandleFunc("GET /api/me", s.session(s.handleMe))
	mux.HandleFunc("GET /api/items", s.session(s.handleListItems))
	mux.HandleFunc("POST /api/text", s.origin(s.session(s.handlePostText)))
	mux.HandleFunc("POST /api/upload", s.origin(s.session(s.handleUpload)))
	mux.HandleFunc("GET /api/items/{id}/raw", s.session(s.handleRaw))
	mux.HandleFunc("POST /api/attachments", s.origin(s.session(s.handleStage)))
	mux.HandleFunc("POST /api/messages", s.origin(s.session(s.handlePostMessage)))
	mux.HandleFunc("GET /api/items/{id}/attachments/{aid}/raw", s.session(s.handleAttachmentRaw))
	mux.HandleFunc("DELETE /api/items/{id}", s.origin(s.session(s.handleDeleteItem)))
	mux.HandleFunc("DELETE /api/items", s.origin(s.session(s.handleClearItems)))
	mux.HandleFunc("POST /api/items/{id}/share", s.origin(s.session(s.handleCreateShare)))
	mux.HandleFunc("GET /api/qr", s.session(s.handleQR))
	mux.HandleFunc("GET /api/s/{id}", s.handleShareMeta)
	mux.HandleFunc("GET /s/{id}", s.handleSharePage)
	mux.HandleFunc("GET /s/{id}/raw", s.handleShareRaw)
	mux.HandleFunc("GET /s/{id}/attachments/{aid}/raw", s.handleShareAttachment)
	mux.HandleFunc("GET /api/dir", s.session(s.handleDirList))
	mux.HandleFunc("GET /api/dir/raw", s.session(s.handleDirRaw))
	mux.HandleFunc("GET /events", s.session(s.handleEvents))
	mux.HandleFunc("GET /api/peers", s.session(s.handlePeers))
	mux.HandleFunc("DELETE /api/peers/{peer_id}", s.origin(s.session(s.hostOnly(s.handleKick))))
	mux.HandleFunc("POST /api/local/text", s.local(s.handleLocalText))
	mux.HandleFunc("POST /api/local/upload", s.local(s.handleLocalUpload))
	mux.HandleFunc("/", s.handleRoot) // GET /?t= token login, else Deps.Web

	return securityHeaders(s.hostCheck(mux))
}
