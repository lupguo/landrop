// Package share implements one-time share links with download count and expiry.
package share

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var ErrGone = errors.New("share: link gone")

const (
	defaultMax = 1
	maxMax     = 100
	defaultTTL = 600 * time.Second
	minTTL     = 60 * time.Second
	maxTTL     = 24 * time.Hour
)

type Link struct {
	ID        string    `json:"id"`
	ItemID    string    `json:"item_id"`
	Remaining int       `json:"remaining"`
	ExpiresAt time.Time `json:"expires_at"`
}

type Manager struct {
	now   func() time.Time
	mu    sync.Mutex
	links map[string]*Link
}

func New(now func() time.Time) *Manager {
	return &Manager{now: now, links: map[string]*Link{}}
}

// Create makes a link; max<=0 → 1, ttl<=0 → 600s; max clamped to 1..100, ttl to 60s..24h.
func (m *Manager) Create(itemID string, maxDownloads int, ttl time.Duration) Link {
	if maxDownloads <= 0 {
		maxDownloads = defaultMax
	}
	if maxDownloads > maxMax {
		maxDownloads = maxMax
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if ttl < minTTL {
		ttl = minTTL
	}
	if ttl > maxTTL {
		ttl = maxTTL
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	l := &Link{ID: hex.EncodeToString(b), ItemID: itemID, Remaining: maxDownloads, ExpiresAt: m.now().Add(ttl)}
	m.mu.Lock()
	m.links[l.ID] = l
	m.mu.Unlock()
	return *l
}

// live reports whether l is usable. Caller holds mu.
func (m *Manager) live(l *Link) bool {
	return l != nil && l.Remaining > 0 && m.now().Before(l.ExpiresAt)
}

// Peek returns the link without consuming a download.
func (m *Manager) Peek(id string) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.links[id]
	if !m.live(l) {
		return Link{}, ErrGone
	}
	return *l, nil
}

// Consume atomically uses one download and returns the updated link.
func (m *Manager) Consume(id string) (Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l := m.links[id]
	if !m.live(l) {
		return Link{}, ErrGone
	}
	l.Remaining--
	return *l, nil
}

// DropItem removes every link for itemID.
func (m *Manager) DropItem(itemID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, l := range m.links {
		if l.ItemID == itemID {
			delete(m.links, id)
		}
	}
}

// Sweep removes expired and exhausted links.
func (m *Manager) Sweep() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, l := range m.links {
		if !m.live(l) {
			delete(m.links, id)
		}
	}
}
