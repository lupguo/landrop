package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/store"
)

const wantCSP = "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; frame-src blob:; style-src 'self' 'unsafe-inline'"

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

func TestTokenLoginSetsCookieAndRedirects(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := &http.Client{CheckRedirect: noRedirect}
	resp, err := c.Get(ts.URL + "/?t=" + d.Auth.Token())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/" {
		t.Fatalf("status %d location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	sc := resp.Header.Get("Set-Cookie")
	for _, want := range []string{"landrop_sid=", "HttpOnly", "SameSite=Strict", "Max-Age=2592000", "Path=/"} {
		if !strings.Contains(sc, want) {
			t.Fatalf("Set-Cookie %q missing %q", sc, want)
		}
	}

	// invalid token serves the SPA without a cookie
	resp, err = c.Get(ts.URL + "/?t=bogus")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Set-Cookie") != "" {
		t.Fatalf("bad token: status %d cookie %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
}

func TestCodeLoginAndLockout(t *testing.T) {
	ts, d := newTestServer(t, nil)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	for i := 0; i < 5; i++ {
		resp, b := do(t, c, ts, "POST", "/api/auth", `{"code":"x"}`)
		wantErr(t, resp, b, 401, "bad_code")
	}
	resp, b := do(t, c, ts, "POST", "/api/auth", fmt.Sprintf(`{"code":%q}`, d.Auth.Code()))
	wantErr(t, resp, b, 429, "locked")

	// fresh server: right code logs in
	ts2, d2 := newTestServer(t, nil)
	resp, _ = do(t, c, ts2, "POST", "/api/auth", fmt.Sprintf(`{"code":%q}`, d2.Auth.Code()))
	if resp.StatusCode != 204 || !strings.Contains(resp.Header.Get("Set-Cookie"), "landrop_sid=") {
		t.Fatalf("status %d cookie %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
	resp, _ = do(t, c, ts2, "GET", "/api/me", "")
	if resp.StatusCode != 200 {
		t.Fatalf("me after code login: %d", resp.StatusCode)
	}
	resp, b = do(t, c, ts2, "POST", "/api/auth", `not json`)
	wantErr(t, resp, b, 400, "bad_request")
}

func TestStaleCookieGets401(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	for _, p := range []string{"/api/items", "/api/me", "/api/peers", "/events"} {
		resp, b := do(t, http.DefaultClient, ts, "GET", p, "", "Cookie", "landrop_sid=deadbeef")
		wantErr(t, resp, b, 401, "unauthorized")
	}
	resp, b := do(t, http.DefaultClient, ts, "GET", "/api/items", "")
	wantErr(t, resp, b, 401, "unauthorized")
}

func TestOriginRequired(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	req, _ := http.NewRequest("POST", ts.URL+"/api/text", strings.NewReader(`{"content":"hi"}`))
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("no origin:", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", ts.URL+"/api/text", strings.NewReader(`{"content":"hi"}`))
	req.Header.Set("Origin", "http://evil.example")
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("evil origin:", resp.StatusCode)
	}
	if n := len(d.Store.List(0)); n != 0 {
		t.Fatalf("items stored despite bad origin: %d", n)
	}
	r2, _ := do(t, c, ts, "POST", "/api/text", `{"content":"hi"}`)
	if r2.StatusCode != 201 {
		t.Fatal("same origin:", r2.StatusCode)
	}
}

func TestTextEmptyAndTooLarge(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	resp, b := do(t, c, ts, "POST", "/api/text", `{"content":"   "}`)
	wantErr(t, resp, b, 400, "empty")
	big, _ := json.Marshal(map[string]string{"content": strings.Repeat("a", store.MaxText+1)})
	resp, b = do(t, c, ts, "POST", "/api/text", string(big))
	wantErr(t, resp, b, 413, "too_large")
	huge := `{"content":"` + strings.Repeat("a", store.MaxText+8192) + `"}`
	resp, b = do(t, c, ts, "POST", "/api/text", huge)
	wantErr(t, resp, b, 413, "too_large")
	resp, b = do(t, c, ts, "POST", "/api/text", `{"content":`)
	wantErr(t, resp, b, 400, "bad_request")
	if id := d.Hub.LastID(); id != 0 {
		t.Fatalf("events published: last id %d", id)
	}
	if n := len(d.Store.List(0)); n != 0 {
		t.Fatalf("items stored: %d", n)
	}
}

func TestTextBroadcastViaSSE(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	a, b := login(t, ts), login(t, ts)
	s := openSSE(t, b, ts, "Phone", "")
	if m := s.next(t, time.Second, func(m sseMsg) bool { return m.Retry != "" }); m.Retry != "500" {
		t.Fatalf("retry %q", m.Retry)
	}
	resp, body := do(t, a, ts, "POST", "/api/text", `{"content":"kubectl get pods"}`, "X-LanDrop-Name", "Mac")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, body)
	}
	var created store.Item
	decode(t, body, &created)
	if created.Content != "kubectl get pods" || created.From.Name != "Mac" || created.From.PeerID == "" {
		t.Fatalf("created %+v", created)
	}
	m := s.next(t, 500*time.Millisecond, isType("item.created"))
	if m.ID == "" {
		t.Fatal("event without id")
	}
	var it store.Item
	decode(t, []byte(m.Data), &it)
	if it.Content != "kubectl get pods" || it.From.Name != "Mac" || it.ID != created.ID {
		t.Fatalf("event item %+v", it)
	}

	// delete and clear are broadcast too
	resp, _ = do(t, a, ts, "DELETE", "/api/items/"+created.ID, "")
	if resp.StatusCode != 204 {
		t.Fatal("delete:", resp.StatusCode)
	}
	m = s.next(t, 500*time.Millisecond, isType("item.deleted"))
	if m.Data != fmt.Sprintf(`{"id":%q}`, created.ID) {
		t.Fatalf("deleted data %s", m.Data)
	}
	resp, b2 := do(t, a, ts, "DELETE", "/api/items/"+created.ID, "")
	wantErr(t, resp, b2, 404, "not_found")
	do(t, a, ts, "POST", "/api/text", `{"content":"x"}`)
	resp, _ = do(t, a, ts, "DELETE", "/api/items", "")
	if resp.StatusCode != 204 {
		t.Fatal("clear:", resp.StatusCode)
	}
	m = s.next(t, 500*time.Millisecond, isType("items.cleared"))
	if m.Data != "{}" {
		t.Fatalf("cleared data %s", m.Data)
	}
	_, lb := do(t, a, ts, "GET", "/api/items", "")
	var list struct {
		Items       []store.Item `json:"items"`
		LastEventID uint64       `json:"last_event_id"`
	}
	decode(t, lb, &list)
	if len(list.Items) != 0 || list.Items == nil {
		t.Fatalf("items after clear: %s", lb)
	}
}

func TestItemsListSinceAndDeviceName(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	do(t, c, ts, "POST", "/api/text", `{"content":"one"}`, "X-LanDrop-Name", "%E6%88%91%E7%9A%84%20Mac")
	do(t, c, ts, "POST", "/api/text", `{"content":"two"}`, "X-LanDrop-Name", strings.Repeat("n", 60))
	do(t, c, ts, "POST", "/api/text", `{"content":"three"}`)
	resp, b := do(t, c, ts, "GET", "/api/items?since=1", "")
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	var list struct {
		Items       []store.Item `json:"items"`
		LastEventID uint64       `json:"last_event_id"`
	}
	decode(t, b, &list)
	if len(list.Items) != 2 || list.Items[0].Content != "three" || list.LastEventID != d.Hub.LastID() || list.LastEventID != 3 {
		t.Fatalf("list %s", b)
	}
	if list.Items[1].From.Name != strings.Repeat("n", 40) {
		t.Fatalf("name not truncated: %q", list.Items[1].From.Name)
	}
	if list.Items[0].From.Name != "testhost" { // loopback → host → Hostname fallback
		t.Fatalf("fallback name %q", list.Items[0].From.Name)
	}
	all := d.Store.List(0)
	if all[2].From.Name != "我的 Mac" {
		t.Fatalf("decoded name %q", all[2].From.Name)
	}
	resp, b = do(t, c, ts, "GET", "/api/items?since=abc", "")
	wantErr(t, resp, b, 400, "bad_request")
}

func TestSSEReplayWithLastEventID(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	do(t, c, ts, "POST", "/api/text", `{"content":"first"}`)
	_, b := do(t, c, ts, "GET", "/api/items", "")
	var list struct {
		LastEventID uint64 `json:"last_event_id"`
	}
	decode(t, b, &list)
	do(t, c, ts, "POST", "/api/text", `{"content":"second"}`)

	s := openSSE(t, c, ts, "Mac", fmt.Sprint(list.LastEventID))
	m := s.next(t, 500*time.Millisecond, isEvent)
	if m.Event != "item.created" || m.ID != fmt.Sprint(list.LastEventID+1) {
		t.Fatalf("first replayed %+v", m)
	}
	var it store.Item
	decode(t, []byte(m.Data), &it)
	if it.Content != "second" {
		t.Fatalf("replayed content %q", it.Content)
	}

	// unknown future id → resync first
	s2 := openSSE(t, c, ts, "Mac", "999")
	if m := s2.next(t, 500*time.Millisecond, isEvent); m.Event != "resync" || m.ID == "" || m.Data == "" {
		t.Fatalf("want resync, got %+v", m)
	}
}

func TestSSEHeartbeat(t *testing.T) {
	old := heartbeatInterval
	heartbeatInterval = 30 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = old })
	ts, _ := newTestServer(t, nil)
	s := openSSE(t, login(t, ts), ts, "Mac", "")
	s.next(t, 500*time.Millisecond, func(m sseMsg) bool { return m.Comment == "ping" })
}

func TestRolesAndKick(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) {
		d.IsHost = func(r *http.Request) bool { return r.Header.Get("X-Test-Peer") != "1" }
	})
	host := login(t, ts)
	peer := login(t, ts)
	peer.Transport = headerRT{h: http.Header{"X-Test-Peer": {"1"}}, rt: http.DefaultTransport}

	var me struct {
		PeerID    string `json:"peer_id"`
		Role      string `json:"role"`
		NoAuth    bool   `json:"no_auth"`
		PublicURL string `json:"public_url"`
		MDNSURL   string `json:"mdns_url"`
		HasDir    bool   `json:"has_dir"`
		MaxSize   int64  `json:"max_size"`
	}
	_, b := do(t, peer, ts, "GET", "/api/me", "")
	decode(t, b, &me)
	if me.Role != "peer" || me.PeerID == "" || me.PublicURL != "http://192.168.1.23:9958" ||
		me.MDNSURL != "http://drop.local:9958" || me.HasDir || me.NoAuth || me.MaxSize != 4<<30 {
		t.Fatalf("peer me %s", b)
	}
	peerID := me.PeerID
	_, b = do(t, host, ts, "GET", "/api/me", "")
	decode(t, b, &me)
	if me.Role != "host" {
		t.Fatalf("host me %s", b)
	}
	hostID := me.PeerID

	s := openSSE(t, peer, ts, "Phone", "")
	s.next(t, time.Second, func(m sseMsg) bool { return m.Retry != "" })
	_, b = do(t, host, ts, "GET", "/api/peers", "")
	var peers struct {
		Peers []struct {
			PeerID string `json:"peer_id"`
			Name   string `json:"name"`
			IP     string `json:"ip"`
			Host   bool   `json:"is_host"`
		} `json:"peers"`
	}
	decode(t, b, &peers)
	if len(peers.Peers) != 1 || peers.Peers[0].PeerID != peerID || peers.Peers[0].Name != "Phone" ||
		peers.Peers[0].IP != "127.0.0.1" || peers.Peers[0].Host {
		t.Fatalf("peers %s", b)
	}

	resp, b := do(t, peer, ts, "DELETE", "/api/peers/"+hostID, "")
	wantErr(t, resp, b, 403, "forbidden")
	resp, b = do(t, host, ts, "DELETE", "/api/peers/"+hostID, "")
	wantErr(t, resp, b, 400, "self")
	resp, _ = do(t, host, ts, "DELETE", "/api/peers/"+peerID, "")
	if resp.StatusCode != 204 {
		t.Fatal("kick:", resp.StatusCode)
	}
	s.next(t, 500*time.Millisecond, isType("session.revoked"))
	resp, b = do(t, peer, ts, "GET", "/api/items", "")
	wantErr(t, resp, b, 401, "unauthorized")
	resp, _ = do(t, host, ts, "GET", "/api/items", "")
	if resp.StatusCode != 200 {
		t.Fatal("host after kick:", resp.StatusCode)
	}
}

func TestNoAuthIssuesSession(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) { d.NoAuth = true })
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	resp, b := do(t, c, ts, "GET", "/api/me", "", "Cookie", "landrop_sid=deadbeef")
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Set-Cookie"), "landrop_sid=") {
		t.Fatalf("status %d cookie %q", resp.StatusCode, resp.Header.Get("Set-Cookie"))
	}
	var me struct {
		PeerID string `json:"peer_id"`
		NoAuth bool   `json:"no_auth"`
	}
	decode(t, b, &me)
	if !me.NoAuth || me.PeerID == "" {
		t.Fatalf("me %s", b)
	}
	resp, _ = do(t, c, ts, "GET", "/api/me", "")
	if resp.Header.Get("Set-Cookie") != "" {
		t.Fatal("second request re-issued a cookie")
	}
	var me2 struct {
		PeerID string `json:"peer_id"`
	}
	_, b = do(t, c, ts, "GET", "/api/me", "")
	decode(t, b, &me2)
	if me2.PeerID != me.PeerID {
		t.Fatalf("peer id changed %s → %s", me.PeerID, me2.PeerID)
	}
	resp, _ = do(t, c, ts, "POST", "/api/auth", `{"code":"nope"}`)
	if resp.StatusCode != 204 {
		t.Fatal("auth in no-auth:", resp.StatusCode)
	}
	openSSE(t, &http.Client{}, ts, "Mac", "") // no cookie at all → still 200
}

func TestSecurityHeaders(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	resp, _ := do(t, c, ts, "GET", "/api/me", "")
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers %v", resp.Header)
	}
	if resp.Header.Get("Content-Security-Policy") != "" {
		t.Fatal("CSP on JSON")
	}
	resp, b := do(t, c, ts, "GET", "/", "")
	if string(b) != testHTML {
		t.Fatalf("body %q", b)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("CSP %q", got)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers %v", resp.Header)
	}
}

// A kick that lands between the session check and Subscribe must not leave
// a live subscription behind: handleEvents re-checks the session.
func TestEventsKickRaceRechecksSession(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	_, b := do(t, c, ts, "GET", "/api/me", "")
	var me struct {
		PeerID string `json:"peer_id"`
	}
	decode(t, b, &me)
	// Simulates a kick that completed just before Subscribe: the session is
	// revoked but Hub.Kick found no subscriber to close.
	afterSubscribe = func() { d.Auth.RevokePeer(me.PeerID) }
	defer func() { afterSubscribe = nil }()
	resp, b := do(t, c, ts, "GET", "/events?name=x", "")
	wantErr(t, resp, b, 401, "unauthorized")
	if n := len(d.Hub.Peers()); n != 0 {
		t.Fatalf("%d peers still subscribed", n)
	}
}

// Without a supplied name, non-host devices get an empty name (the UI
// renders a localized "unknown"); the host still falls back to Hostname.
func TestEmptyDeviceNameForPeers(t *testing.T) {
	ts, d := newTestServer(t, func(d *Deps) {
		d.IsHost = func(r *http.Request) bool { return r.Header.Get("X-Test-Peer") != "1" }
	})
	c := login(t, ts)
	do(t, c, ts, "POST", "/api/text", `{"content":"peer"}`, "X-Test-Peer", "1")
	do(t, c, ts, "POST", "/api/text", `{"content":"host"}`)
	names := map[string]string{}
	for _, it := range d.Store.List(0) {
		names[it.Content] = it.From.Name
	}
	if n, ok := names["peer"]; !ok || n != "" || names["host"] != "testhost" {
		t.Fatalf("names %q", names)
	}
	peer := login(t, ts)
	peer.Transport = headerRT{h: http.Header{"X-Test-Peer": {"1"}}, rt: http.DefaultTransport}
	s := openSSE(t, peer, ts, "", "")
	s.next(t, time.Second, func(m sseMsg) bool { return m.Retry != "" })
	ps := d.Hub.Peers()
	if len(ps) != 1 || ps[0].Name != "" {
		t.Fatalf("peers %+v", ps)
	}
}

func TestMeInviteInfo(t *testing.T) {
	type invite struct {
		Code    string `json:"code"`
		JoinURL string `json:"join_url"`
	}
	ts, d := newTestServer(t, nil)
	var me invite
	_, b := do(t, login(t, ts), ts, "GET", "/api/me", "")
	decode(t, b, &me)
	if me.Code != d.Auth.Code() || me.JoinURL != "http://192.168.1.23:9958/?t="+d.Auth.Token() {
		t.Fatalf("me %s", b)
	}

	// NoAuth: no code to hand out; the plain address is the join link.
	ts2, _ := newTestServer(t, func(d *Deps) { d.NoAuth = true })
	jar, _ := cookiejar.New(nil)
	_, b = do(t, &http.Client{Jar: jar}, ts2, "GET", "/api/me", "")
	me = invite{}
	decode(t, b, &me)
	if me.Code != "" || me.JoinURL != "http://192.168.1.23:9958" {
		t.Fatalf("noauth me %s", b)
	}
}
