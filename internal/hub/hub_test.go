package hub

import (
	"sync"
	"testing"
	"time"
)

func TestReplayAfterLastID(t *testing.T) {
	h := New(8)
	e1 := h.Publish("item.created", 1)
	h.Publish("item.created", 2)
	_, replay, resync := h.Subscribe(Peer{ID: "p"}, e1.ID)
	if resync || len(replay) != 1 || replay[0].Data != 2 {
		t.Fatal(replay, resync)
	}
}

func TestSubscribeGapNeedsResync(t *testing.T) {
	h := New(2)
	for i := 0; i < 5; i++ {
		h.Publish("x", i)
	}
	if _, _, resync := h.Subscribe(Peer{ID: "p"}, 1); !resync {
		t.Fatal("want resync")
	}
}

func recv(t *testing.T, c <-chan Event) Event {
	t.Helper()
	select {
	case ev := <-c:
		return ev
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
		return Event{}
	}
}

func waitDone(t *testing.T, d <-chan struct{}) {
	t.Helper()
	select {
	case <-d:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Done")
	}
}

func TestSlowSubscriberDropped(t *testing.T) {
	h := New(1024)
	slow, _, _ := h.Subscribe(Peer{ID: "slow"}, 0)
	fast, _, _ := h.Subscribe(Peer{ID: "fast"}, 0)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := 0; i < 200; i++ {
			h.Publish("x", i)
			for { // skip presence events (peer.left of slow)
				select {
				case ev := <-fast.C:
					if ev.Type != "x" {
						continue
					}
					if ev.Data != i {
						t.Errorf("event %d: got %v", i, ev.Data)
						return
					}
				case <-time.After(time.Second):
					t.Errorf("timeout waiting for event %d", i)
					return
				}
				break
			}
		}
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("publish loop blocked")
	}
	select {
	case <-slow.Done:
	default:
		t.Fatal("slow not dropped")
	}
	select {
	case <-fast.Done:
		t.Fatal("fast dropped")
	default:
	}
}

func TestPeerJoinLeaveAndKick(t *testing.T) {
	h := New(16)
	obs, _, _ := h.Subscribe(Peer{ID: "obs"}, 0)
	a, _, _ := h.Subscribe(Peer{ID: "a", Name: "Win"}, 0)
	if ev := recv(t, obs.C); ev.Type != "peer.joined" {
		t.Fatal(ev)
	}
	h.Kick("a")
	if ev := recv(t, a.C); ev.Type != "session.revoked" {
		t.Fatal(ev)
	}
	waitDone(t, a.Done)
	if ev := recv(t, obs.C); ev.Type != "peer.left" {
		t.Fatal(ev)
	}
	if len(h.Peers()) != 1 {
		t.Fatal(h.Peers())
	}
}

func TestNoSelfJoinAndLastID(t *testing.T) {
	h := New(8)
	if h.LastID() != 0 {
		t.Fatal("want 0")
	}
	s, replay, _ := h.Subscribe(Peer{ID: "a"}, 0)
	if len(replay) != 0 || len(s.C) != 0 {
		t.Fatal("own join leaked")
	}
	if h.LastID() != 1 { // peer.joined consumed ID 1
		t.Fatal(h.LastID())
	}
	e := h.Publish("x", nil)
	if e.ID != 2 || h.LastID() != 2 {
		t.Fatal(e.ID)
	}
	h.Kick("a")
	if h.LastID() != 3 { // peer.left only; session.revoked consumes none
		t.Fatal(h.LastID())
	}
}

func TestUnsubscribeIdempotentAndPeerLeftOnce(t *testing.T) {
	h := New(16)
	obs, _, _ := h.Subscribe(Peer{ID: "obs"}, 0)
	a1, _, _ := h.Subscribe(Peer{ID: "a"}, 0)
	a2, _, _ := h.Subscribe(Peer{ID: "a"}, 0) // same peer, second tab
	recv(t, obs.C)                            // joined (only once)
	h.Unsubscribe(a1)
	h.Unsubscribe(a1)
	select {
	case ev := <-obs.C:
		t.Fatal("unexpected", ev)
	default:
	}
	h.Unsubscribe(a2)
	h.Unsubscribe(a2)
	if ev := recv(t, obs.C); ev.Type != "peer.left" {
		t.Fatal(ev)
	}
	select {
	case ev := <-obs.C:
		t.Fatal("double left", ev)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestSlowDropPublishesLeftAndPeersSorted(t *testing.T) {
	h := New(1024)
	obs, _, _ := h.Subscribe(Peer{ID: "obs", Name: "b"}, 0)
	slow, _, _ := h.Subscribe(Peer{ID: "slow", Name: "a"}, 0)
	left := 0
	drain := func() {
		for {
			select {
			case ev := <-obs.C:
				if ev.Type == "peer.left" {
					left++
				}
			default:
				return
			}
		}
	}
	for i := 0; i < 100; i++ {
		h.Publish("x", i)
		drain()
	}
	waitDone(t, slow.Done)
	h.Unsubscribe(slow) // no-op after drop
	drain()
	ps := h.Peers()
	if len(ps) != 1 || ps[0].ID != "obs" {
		t.Fatal(ps)
	}
	if left != 1 {
		t.Fatal("peer.left count", left)
	}
}

func TestConcurrent(t *testing.T) {
	h := New(64)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, _, _ := h.Subscribe(Peer{ID: string(rune('a' + i%3))}, 0)
			for j := 0; j < 100; j++ {
				h.Publish("x", j)
			}
			h.Kick("a")
			h.Unsubscribe(s)
		}(i)
	}
	wg.Wait()
}
