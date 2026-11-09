package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/lupguo/landrop/internal/store"
)

// handleListItems: GET /api/items?since=N.
func (s *server) handleListItems(w http.ResponseWriter, r *http.Request) {
	var since uint64
	if v := r.URL.Query().Get("since"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "bad_request")
			return
		}
		since = n
	}
	// Read LastID first so a client resuming SSE from it never misses an
	// item created after the listing.
	last := s.Hub.LastID()
	writeJSON(w, http.StatusOK, map[string]any{
		"items":         s.Store.List(since),
		"last_event_id": last,
	})
}

// handlePostText: POST /api/text {content}.
func (s *server) handlePostText(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if !readJSON(w, r, store.MaxText+4096, &body) {
		return
	}
	from := store.From{PeerID: sessionFrom(r).PeerID, Name: s.deviceName(r, r.Header.Get(nameHeader))}
	it, err := s.Store.AddText(from, body.Content)
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

// handleDeleteItem: DELETE /api/items/{id}.
func (s *server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.Store.Delete(id) {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	s.Share.DropItem(id)
	s.Hub.Publish("item.deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

// handleClearItems: DELETE /api/items.
func (s *server) handleClearItems(w http.ResponseWriter, r *http.Request) {
	for _, id := range s.Store.Clear() {
		s.Share.DropItem(id)
	}
	s.Hub.Publish("items.cleared", map[string]any{})
	w.WriteHeader(http.StatusNoContent)
}
