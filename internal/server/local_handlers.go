package server

import (
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"syscall"

	"github.com/lupguo/landrop/internal/store"
)

const secretHeader = "X-LanDrop-Secret"

// local guards /api/local/*: the caller must be on loopback and present the
// instance secret. There is no session and no Origin check.
func (s *server) local(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(secretHeader)
		if !isLoopback(r) || s.LocalSecret == "" ||
			subtle.ConstantTimeCompare([]byte(got), []byte(s.LocalSecret)) != 1 {
			writeErr(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

func (s *server) cliFrom() store.From {
	return store.From{PeerID: "cli", Name: s.Hostname + " (CLI)"}
}

// handleLocalText: POST /api/local/text, raw body is the text.
func (s *server) handleLocalText(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(io.LimitReader(r.Body, store.MaxText+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request")
		return
	}
	if len(b) > store.MaxText {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}
	it, err := s.Store.AddText(s.cliFrom(), string(b))
	switch {
	case errors.Is(err, store.ErrEmpty):
		writeErr(w, http.StatusBadRequest, "empty")
		return
	case errors.Is(err, store.ErrTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	s.Hub.Publish("item.created", it)
	writeJSON(w, http.StatusCreated, it)
}

// handleLocalUpload: POST /api/local/upload?name=, raw body streamed to disk.
func (s *server) handleLocalUpload(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if r.ContentLength > s.MaxSize {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}
	if r.ContentLength > 0 {
		free, err := freeSpace(s.Store.Dir())
		if err == nil && free < uint64(r.ContentLength)+diskReserve {
			writeErr(w, http.StatusInsufficientStorage, "disk_full")
			return
		}
	}
	it, err := s.Store.AddFile(s.cliFrom(), name, "", r.Body, s.MaxSize)
	switch {
	case tooLarge(err):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case errors.Is(err, syscall.ENOSPC):
		writeErr(w, http.StatusInsufficientStorage, "disk_full")
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "upload_failed")
		return
	}
	s.Hub.Publish("item.created", it)
	if s.OnFileStored != nil {
		s.OnFileStored(it)
	}
	writeJSON(w, http.StatusCreated, it)
}
