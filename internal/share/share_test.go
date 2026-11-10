package share

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultsAndExpiry(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	l := m.Create("i1", 0, 0)
	if l.Remaining != 1 || l.ExpiresAt != now.Add(10*time.Minute) || len(l.ID) != 32 {
		t.Fatal(l)
	}
	now = now.Add(11 * time.Minute)
	if _, err := m.Peek(l.ID); !errors.Is(err, ErrGone) {
		t.Fatal(err)
	}
}

func TestConsumeConcurrent(t *testing.T) {
	m := New(time.Now)
	l := m.Create("i1", 1, time.Minute)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Consume(l.ID); err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatal(ok.Load())
	}
}

func TestDropItem(t *testing.T) {
	m := New(time.Now)
	l := m.Create("i1", 3, time.Minute)
	m.DropItem("i1")
	if _, err := m.Peek(l.ID); !errors.Is(err, ErrGone) {
		t.Fatal(err)
	}
}

func TestClampPeekSweep(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	l := m.Create("i1", 1000, 1)
	if l.Remaining != 100 || l.ExpiresAt != now.Add(60*time.Second) {
		t.Fatal(l)
	}
	l = m.Create("i1", 2, 48*time.Hour)
	if l.ExpiresAt != now.Add(24*time.Hour) {
		t.Fatal(l)
	}
	if p, _ := m.Peek(l.ID); p.Remaining != 2 {
		t.Fatal("peek must not decrement")
	}
	m.Consume(l.ID)
	if c, err := m.Consume(l.ID); err != nil || c.Remaining != 0 {
		t.Fatal(c, err)
	}
	if _, err := m.Peek(l.ID); !errors.Is(err, ErrGone) {
		t.Fatal("exhausted")
	}
	m.Sweep()
	m.mu.Lock()
	n := len(m.links)
	m.mu.Unlock()
	if n != 1 {
		t.Fatal(n)
	}
}

func TestUnlimitedLink(t *testing.T) {
	now := time.Unix(0, 0)
	m := New(func() time.Time { return now })
	l := m.Create("i1", Unlimited, time.Minute)
	if l.Remaining != -1 {
		t.Fatal(l)
	}
	for i := 0; i < 50; i++ {
		got, err := m.Consume(l.ID)
		if err != nil || got.Remaining != -1 {
			t.Fatalf("consume %d: %v %v", i, got, err)
		}
	}
	m.Sweep()
	if _, err := m.Peek(l.ID); err != nil {
		t.Fatal("sweep dropped live unlimited link", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := m.Consume(l.ID); !errors.Is(err, ErrGone) {
		t.Fatal(err)
	}
	m.Sweep()
	if len(m.links) != 0 {
		t.Fatal("expired link not swept")
	}
}
