package auth

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"
)

func TestCodeFormat(t *testing.T) {
	m := New(time.Now)
	if ok, _ := regexp.MatchString(`^\d{4}$`, m.Code()); !ok || len(m.Token()) != 32 {
		t.Fatal(m.Code(), m.Token())
	}
}

func TestLockout(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	for i := 0; i < 5; i++ {
		if err := m.CheckCode("1.1.1.1", "x"); !errors.Is(err, ErrBadCode) {
			t.Fatal(i, err)
		}
	}
	if err := m.CheckCode("1.1.1.1", m.Code()); !errors.Is(err, ErrLocked) {
		t.Fatal(err)
	}
	if err := m.CheckCode("2.2.2.2", m.Code()); err != nil {
		t.Fatal("other IP must not be locked")
	}
	now = now.Add(61 * time.Second)
	if err := m.CheckCode("1.1.1.1", m.Code()); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalLockout(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	for i := 0; i < 31; i++ {
		m.CheckCode(fmt.Sprintf("10.0.%d.%d", i/250, i%250), "x")
	}
	if err := m.CheckCode("10.9.9.9", m.Code()); !errors.Is(err, ErrLocked) {
		t.Fatal("want global lock", err)
	}
	now = now.Add(61 * time.Second)
	if err := m.CheckCode("10.9.9.9", m.Code()); err != nil {
		t.Fatal(err)
	}
}

func TestSuccessResetsAndPrunes(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	for i := 0; i < 4; i++ {
		m.CheckCode("1.1.1.1", "x")
	}
	if err := m.CheckCode("1.1.1.1", m.Code()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		m.CheckCode("1.1.1.1", "x")
	}
	if err := m.CheckCode("1.1.1.1", m.Code()); err != nil {
		t.Fatal("success must reset counter", err)
	}
	for i := 0; i < 100; i++ {
		m.CheckCode(fmt.Sprintf("9.9.9.%d", i), "x")
	}
	now = now.Add(5 * time.Minute)
	m.CheckCode("3.3.3.3", "x")
	m.mu.Lock()
	n := len(m.ips)
	m.mu.Unlock()
	if n != 1 {
		t.Fatal("stale ip entries not pruned", n)
	}
}

func TestToken(t *testing.T) {
	m := New(time.Now)
	if !m.CheckToken(m.Token()) || m.CheckToken("nope") {
		t.Fatal("token check")
	}
}

func TestSessionsAndRevoke(t *testing.T) {
	m := New(time.Now)
	s := m.NewSession()
	if _, ok := m.Lookup(s.ID); !ok {
		t.Fatal()
	}
	m.RevokePeer(s.PeerID)
	if _, ok := m.Lookup(s.ID); ok {
		t.Fatal("revoked")
	}
}
