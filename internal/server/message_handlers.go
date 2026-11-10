package server

import (
	"errors"
	"net/http"

	"github.com/lupguo/landrop/internal/store"
)

// handleStage: POST /api/attachments, multipart part "file". Stages the file
// for the caller's next message; publishes nothing.
func (s *server) handleStage(w http.ResponseWriter, r *http.Request) {
	part, ok := s.filePart(w, r)
	if !ok {
		return
	}
	att, err := s.Store.Stage(sessionFrom(r).PeerID, part.FileName(), part.Header.Get("Content-Type"), part, s.MaxSize)
	switch {
	case tooLarge(err):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case isDiskFull(err):
		writeErr(w, http.StatusInsufficientStorage, "disk_full")
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, "upload_failed")
		return
	}
	writeJSON(w, http.StatusCreated, att)
}

// handlePostMessage: POST /api/messages {content, attachments:[id…]}.
func (s *server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content     string   `json:"content"`
		Attachments []string `json:"attachments"`
	}
	if !readJSON(w, r, store.MaxText+64<<10, &body) {
		return
	}
	from := store.From{PeerID: sessionFrom(r).PeerID, Name: s.deviceName(r, r.Header.Get(nameHeader))}
	// AddMessage only commits stagings owned by this peer.
	it, err := s.Store.AddMessage(from, body.Content, body.Attachments)
	switch {
	case errors.Is(err, store.ErrEmpty):
		writeErr(w, http.StatusBadRequest, "empty")
		return
	case errors.Is(err, store.ErrTooMany):
		writeErr(w, http.StatusBadRequest, "too_many")
		return
	case errors.Is(err, store.ErrBadAttachment):
		writeErr(w, http.StatusBadRequest, "bad_attachment")
		return
	case errors.Is(err, store.ErrTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	s.Hub.Publish("item.created", it)
	if s.OnFileStored != nil {
		for _, a := range it.Attachments {
			s.OnFileStored(store.Item{
				ID: a.ID, Type: store.KindFile,
				File: &store.FileMeta{Name: a.Name, Size: a.Size, MIME: a.MIME},
				From: it.From, CreatedAt: it.CreatedAt,
			})
		}
	}
	writeJSON(w, http.StatusCreated, it)
}

// handleAttachmentRaw: GET /api/items/{id}/attachments/{aid}/raw[?inline=1].
func (s *server) handleAttachmentRaw(w http.ResponseWriter, r *http.Request) {
	f, att, it, err := s.Store.OpenAttachment(r.PathValue("id"), r.PathValue("aid"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	defer f.Close()
	s.serveFile(w, r, f, att.Name, att.MIME, it.CreatedAt, false, r.URL.Query().Get("inline") == "1")
}
