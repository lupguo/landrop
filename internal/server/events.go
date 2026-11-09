package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/lupguo/landrop/internal/hub"
)

// heartbeatInterval is the SSE keep-alive period; tests shorten it.
var heartbeatInterval = 15 * time.Second

// handleEvents: GET /events?name= (SSE).
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	sess := sessionFrom(r)
	peer := hub.Peer{
		ID:   sess.PeerID,
		Name: s.deviceName(r, r.URL.Query().Get("name")),
		UA:   truncateRunes(r.UserAgent(), maxUARunes),
		IP:   remoteIP(r),
		Host: s.IsHost(r),
	}
	var lastID uint64
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		lastID, _ = strconv.ParseUint(v, 10, 64)
	}

	cur := s.Hub.LastID()
	sub, replay, resync := s.Hub.Subscribe(peer, lastID)
	defer s.Hub.Unsubscribe(sub)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := io.WriteString(w, "retry: 500\n\n"); err != nil {
		return
	}
	if resync {
		// Always carry an id (even 0) so the client's Last-Event-ID moves to
		// the current position and a reconnect does not resync again.
		if _, err := fmt.Fprintf(w, "id: %d\nevent: resync\ndata: {}\n\n", cur); err != nil {
			return
		}
	}
	for _, ev := range replay {
		if writeEvent(w, ev) != nil {
			return
		}
	}
	if rc.Flush() != nil {
		return
	}

	tick := time.NewTicker(heartbeatInterval)
	defer tick.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return
		case ev := <-sub.C:
			err = writeEvent(w, ev)
		case <-sub.Done:
			// Deliver anything queued before the close (e.g. session.revoked).
			for {
				select {
				case ev := <-sub.C:
					if writeEvent(w, ev) != nil {
						return
					}
				default:
					rc.Flush()
					return
				}
			}
		case <-tick.C:
			_, err = io.WriteString(w, ": ping\n\n")
		}
		if err != nil || rc.Flush() != nil {
			return
		}
	}
}

// writeEvent writes one SSE event. Events with ID 0 (session.revoked, which
// is not in the replay ring) omit the id line so the client's Last-Event-ID
// is not reset.
func writeEvent(w io.Writer, ev hub.Event) error {
	data := []byte("{}")
	if ev.Data != nil {
		b, err := json.Marshal(ev.Data)
		if err != nil {
			return err
		}
		data = b
	}
	var err error
	if ev.ID > 0 {
		_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.ID, ev.Type, data)
	} else {
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
	}
	return err
}
