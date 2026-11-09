package server

import (
	"net/http"
	"time"

	"github.com/lupguo/landrop/internal/qr"
	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
	"github.com/lupguo/landrop/internal/webui"
)

// indexFn is swappable in tests.
var indexFn = webui.Index

const maxQRData = 1024

// handleCreateShare: POST /api/items/{id}/share.
func (s *server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		MaxDownloads int `json:"max_downloads"`
		TTLSeconds   int `json:"ttl_seconds"`
	}
	if r.ContentLength != 0 && !readJSON(w, r, 4096, &body) {
		return
	}
	if _, ok := s.Store.Get(id); !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	l := s.Share.Create(id, body.MaxDownloads, time.Duration(body.TTLSeconds)*time.Second)
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         l.ID,
		"url":        s.PublicBase + "/s/" + l.ID,
		"remaining":  l.Remaining,
		"expires_at": l.ExpiresAt,
	})
}

// handleQR: GET /api/qr?data=.
func (s *server) handleQR(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("data")
	if data == "" || len(data) > maxQRData {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	svg, err := qr.SVG(data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(svg)
}

// liveShare peeks the link and verifies its item still exists.
func (s *server) liveShare(id string) (share.Link, store.Item, bool) {
	l, err := s.Share.Peek(id)
	if err != nil {
		return share.Link{}, store.Item{}, false
	}
	it, ok := s.Store.Get(l.ItemID)
	if !ok {
		return share.Link{}, store.Item{}, false
	}
	return l, it, true
}

// handleShareMeta: GET /api/s/{id}, without consuming.
func (s *server) handleShareMeta(w http.ResponseWriter, r *http.Request) {
	l, it, ok := s.liveShare(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusGone, "gone")
		return
	}
	m := map[string]any{"type": it.Type, "remaining": l.Remaining, "expires_at": l.ExpiresAt}
	if it.Type == store.KindText {
		m["name"], m["size"], m["mime"] = "", len(it.Content), "text/plain"
	} else {
		m["name"], m["size"], m["mime"] = it.File.Name, it.File.Size, it.File.MIME
	}
	writeJSON(w, http.StatusOK, m)
}

// handleSharePage: GET /s/{id} serves the SPA with 200, or 410 when gone.
func (s *server) handleSharePage(w http.ResponseWriter, r *http.Request) {
	b, ok := indexFn()
	if !ok {
		http.Error(w, `frontend not built: run "make web"`, http.StatusServiceUnavailable)
		return
	}
	status := http.StatusOK
	if _, _, live := s.liveShare(r.PathValue("id")); !live {
		status = http.StatusGone
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write(b)
}

// handleShareRaw: GET /s/{id}/raw consumes one download.
func (s *server) handleShareRaw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if r.Method == http.MethodHead {
		// Probes must not spend a download.
		_, it, ok := s.liveShare(id)
		if !ok {
			writeErr(w, http.StatusGone, "gone")
			return
		}
		s.serveItem(w, r, it, true)
		return
	}
	// Check the item first so a missing item doesn't burn a count.
	if _, _, ok := s.liveShare(id); !ok {
		writeErr(w, http.StatusGone, "gone")
		return
	}
	l, err := s.Share.Consume(id)
	if err != nil {
		writeErr(w, http.StatusGone, "gone")
		return
	}
	it, ok := s.Store.Get(l.ItemID)
	if !ok {
		writeErr(w, http.StatusGone, "gone")
		return
	}
	s.serveItem(w, r, it, true)
}
