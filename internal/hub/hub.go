// Package hub fans real-time events out to SSE subscribers, keeps a replay
// ring for reconnects and tracks which peers are online.
package hub

import (
	"sort"
	"sync"
)

const subBuf = 64

// Event is a hub event. IDs start at 1 and increase by 1 per Publish.
type Event struct {
	ID   uint64
	Type string
	Data any
}

// Peer describes a connected device.
type Peer struct {
	ID   string `json:"peer_id"`
	Name string `json:"name"`
	UA   string `json:"ua"`
	IP   string `json:"ip"`
	Host bool   `json:"is_host"`
}

// Sub is a subscription handle. Done is closed when the sub is dropped,
// kicked or unsubscribed.
type Sub struct {
	C    <-chan Event
	Done <-chan struct{}

	c      chan Event
	done   chan struct{}
	peer   Peer
	closed bool // guarded by Hub.mu
}

// Hub is the event bus and peer registry.
type Hub struct {
	mu     sync.Mutex
	size   int
	ring   []Event // oldest first, at most size entries
	nextID uint64
	subs   map[*Sub]struct{}
}

// New creates a hub whose replay ring holds bufSize events.
func New(bufSize int) *Hub {
	if bufSize < 1 {
		bufSize = 1
	}
	return &Hub{size: bufSize, nextID: 1, subs: map[*Sub]struct{}{}}
}

// Publish stores the event in the ring and delivers it to all subscribers
// without blocking. Subscribers with a full buffer are dropped.
func (h *Hub) Publish(typ string, data any) Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.publishLocked(typ, data)
}

// publishLocked never blocks: sends are non-blocking and channel closes are
// immediate.
func (h *Hub) publishLocked(typ string, data any) Event {
	ev := Event{ID: h.nextID, Type: typ, Data: data}
	h.nextID++
	if len(h.ring) >= h.size {
		n := copy(h.ring, h.ring[len(h.ring)-h.size+1:])
		h.ring = h.ring[:n]
	}
	h.ring = append(h.ring, ev)

	var dropped []*Sub
	for s := range h.subs {
		select {
		case s.c <- ev:
		default:
			dropped = append(dropped, s)
		}
	}
	for _, s := range dropped {
		h.removeLocked(s)
	}
	return ev
}

// removeLocked closes s exactly once and, if it was the peer's last sub,
// publishes peer.left. Returns false if s was already removed.
func (h *Hub) removeLocked(s *Sub) bool {
	if s.closed {
		return false
	}
	s.closed = true
	delete(h.subs, s)
	close(s.done)
	if !h.hasPeerLocked(s.peer.ID) {
		h.publishLocked("peer.left", map[string]string{"peer_id": s.peer.ID})
	}
	return true
}

func (h *Hub) hasPeerLocked(id string) bool {
	for s := range h.subs {
		if s.peer.ID == id {
			return true
		}
	}
	return false
}

// LastID returns the ID of the most recent ring event (0 if none).
func (h *Hub) LastID() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.nextID - 1
}

// Subscribe registers a subscriber. lastID==0 is a fresh connect (no replay,
// no resync). If lastID>0 and the ring cannot supply lastID+1, resync is true
// and no replay is returned.
func (h *Hub) Subscribe(p Peer, lastID uint64) (*Sub, []Event, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var replay []Event
	resync := false
	if lastID > 0 {
		last := h.nextID - 1
		switch {
		case lastID > last:
			resync = true // unknown future ID, e.g. server restarted
		case lastID == last:
		case len(h.ring) == 0 || h.ring[0].ID > lastID+1:
			resync = true
		default:
			start := int(lastID + 1 - h.ring[0].ID)
			replay = append([]Event(nil), h.ring[start:]...)
		}
	}

	if !h.hasPeerLocked(p.ID) {
		h.publishLocked("peer.joined", p)
	}

	c := make(chan Event, subBuf)
	s := &Sub{C: c, Done: nil, c: c, done: make(chan struct{}), peer: p}
	s.Done = s.done
	h.subs[s] = struct{}{}
	return s, replay, resync
}

// Unsubscribe removes s. It is a no-op if s was already removed.
func (h *Hub) Unsubscribe(s *Sub) {
	if s == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(s)
}

// Kick sends session.revoked to every sub of the peer (not stored in the
// ring, no ID consumed) and then closes them.
func (h *Hub) Kick(peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var targets []*Sub
	for s := range h.subs {
		if s.peer.ID == peerID {
			targets = append(targets, s)
		}
	}
	for _, s := range targets {
		select {
		case s.c <- Event{Type: "session.revoked"}:
		default:
		}
		h.removeLocked(s)
	}
}

// Peers returns online peers, unique by ID, sorted by name.
func (h *Hub) Peers() []Peer {
	h.mu.Lock()
	defer h.mu.Unlock()
	seen := map[string]bool{}
	var out []Peer
	for s := range h.subs {
		if !seen[s.peer.ID] {
			seen[s.peer.ID] = true
			out = append(out, s.peer)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}
