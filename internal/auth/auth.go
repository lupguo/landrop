// Package auth implements the access code, link token, in-memory sessions
// and brute-force lockout for LanDrop.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

const (
	CookieName   = "landrop_sid"
	CookieMaxAge = 30 * 24 * 3600

	ipMaxFails    = 5
	globalMaxFail = 30
	lockDuration  = 60 * time.Second
	window        = 60 * time.Second
)

var (
	ErrBadCode = errors.New("auth: bad code")
	ErrLocked  = errors.New("auth: locked")
)

type Session struct {
	ID        string
	PeerID    string
	CreatedAt time.Time
}

type ipState struct {
	fails    int
	lastFail time.Time
	lockedTo time.Time
}

type Manager struct {
	now   func() time.Time
	code  string
	token string

	mu         sync.Mutex
	ips        map[string]*ipState
	failTimes  []time.Time // global failures within the last window
	globalLock time.Time
	sessions   map[string]Session
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// New creates a Manager with a random 4-digit code and 32-hex token.
func New(now func() time.Time) *Manager {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		panic(err)
	}
	return &Manager{
		now:      now,
		code:     fmt.Sprintf("%04d", n.Int64()),
		token:    randHex(16),
		ips:      map[string]*ipState{},
		sessions: map[string]Session{},
	}
}

func (m *Manager) Code() string  { return m.code }
func (m *Manager) Token() string { return m.token }

func (m *Manager) CheckToken(tok string) bool {
	return subtle.ConstantTimeCompare([]byte(tok), []byte(m.token)) == 1
}

// CheckCode validates an access code from ip. While a lock is active the code
// is not evaluated and the attempt is not counted.
func (m *Manager) CheckCode(ip, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.prune(now)

	if now.Before(m.globalLock) {
		return ErrLocked
	}
	st := m.ips[ip]
	if st != nil && now.Before(st.lockedTo) {
		return ErrLocked
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(m.code)) == 1 {
		delete(m.ips, ip)
		return nil
	}

	if st == nil {
		st = &ipState{}
		m.ips[ip] = st
	}
	st.fails++
	st.lastFail = now
	if st.fails >= ipMaxFails {
		st.lockedTo = now.Add(lockDuration)
		st.fails = 0
	}
	m.failTimes = append(m.failTimes, now)
	if len(m.failTimes) > globalMaxFail {
		m.globalLock = now.Add(lockDuration)
		m.failTimes = nil
	}
	return ErrBadCode
}

// prune drops stale failure timestamps and idle per-IP entries. Caller holds mu.
func (m *Manager) prune(now time.Time) {
	i := 0
	for i < len(m.failTimes) && now.Sub(m.failTimes[i]) >= window {
		i++
	}
	m.failTimes = m.failTimes[i:]
	for ip, st := range m.ips {
		if !now.Before(st.lockedTo) && now.Sub(st.lastFail) >= window {
			delete(m.ips, ip)
		}
	}
}

func (m *Manager) NewSession() Session {
	s := Session{ID: randHex(16), PeerID: "p_" + randHex(4), CreatedAt: m.now()}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

func (m *Manager) Lookup(id string) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) RevokePeer(peerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.PeerID == peerID {
			delete(m.sessions, id)
		}
	}
}
