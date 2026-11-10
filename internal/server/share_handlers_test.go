package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
)

func useIndex(t *testing.T) {
	t.Helper()
	old := indexFn
	indexFn = func() ([]byte, bool) { return []byte(testHTML), true }
	t.Cleanup(func() { indexFn = old })
}

func uploadShared(t *testing.T, ts *httptest.Server, c *http.Client, body string) (itemID string) {
	t.Helper()
	resp, b := doUpload(t, c, ts.URL, "a.txt", "text/plain", []byte(body))
	if resp.StatusCode != 201 {
		t.Fatalf("upload %d %s", resp.StatusCode, b)
	}
	var it struct{ ID string }
	decode(t, b, &it)
	return it.ID
}

func TestShareOnceThenGone(t *testing.T) {
	useIndex(t)
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	id := uploadShared(t, ts, c, "hello")
	resp, b := do(t, c, ts, "POST", "/api/items/"+id+"/share", "{}")
	if resp.StatusCode != 201 {
		t.Fatalf("share %d %s", resp.StatusCode, b)
	}
	var sh struct {
		ID, URL   string
		ExpiresAt string `json:"expires_at"`
		Remaining int
	}
	decode(t, b, &sh)
	if !strings.HasPrefix(sh.URL, d.PublicBase+"/s/") || sh.Remaining != 1 || sh.ExpiresAt == "" {
		t.Fatalf("share resp %+v", sh)
	}
	anon := &http.Client{}
	for i := 0; i < 2; i++ {
		resp, b = do(t, anon, ts, "GET", "/api/s/"+sh.ID, "")
		if resp.StatusCode != 200 {
			t.Fatalf("meta %d %s", resp.StatusCode, b)
		}
		var m struct {
			Type, Name, Mime string
			Size, Remaining  int
		}
		decode(t, b, &m)
		if m.Remaining != 1 || m.Name != "a.txt" || m.Type != "file" || m.Size != 5 {
			t.Fatalf("meta %+v", m)
		}
	}
	resp, b = do(t, anon, ts, "GET", "/s/"+sh.ID, "")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") ||
		resp.Header.Get("Content-Security-Policy") == "" || string(b) != testHTML {
		t.Fatalf("page %d %v", resp.StatusCode, resp.Header)
	}
	resp, b = do(t, anon, ts, "GET", "/s/"+sh.ID+"/raw", "")
	if resp.StatusCode != 200 || string(b) != "hello" ||
		!strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("raw %d %q %v", resp.StatusCode, b, resp.Header)
	}
	resp, b = do(t, anon, ts, "GET", "/s/"+sh.ID+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
	resp, _ = do(t, anon, ts, "GET", "/s/"+sh.ID, "")
	if resp.StatusCode != 410 {
		t.Fatalf("page after use: %d", resp.StatusCode)
	}
}

func TestShareDiesWithItem(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	id := uploadShared(t, ts, c, "x")
	_, b := do(t, c, ts, "POST", "/api/items/"+id+"/share", "{}")
	var sh struct{ ID string }
	decode(t, b, &sh)
	if resp, _ := do(t, c, ts, "DELETE", "/api/items/"+id, ""); resp.StatusCode != 204 {
		t.Fatal(resp.StatusCode)
	}
	resp, b := do(t, &http.Client{}, ts, "GET", "/api/s/"+sh.ID, "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareUsesPublicBaseNotHostHeader(t *testing.T) {
	ts, d := newTestServer(t, nil)
	c := login(t, ts)
	id := uploadShared(t, ts, c, "x")
	_, b := do(t, c, ts, "POST", "/api/items/"+id+"/share", `{"max_downloads":3,"ttl_seconds":60}`, "X-Forwarded-Host", "evil.example")
	var sh struct {
		URL       string
		Remaining int
	}
	decode(t, b, &sh)
	if !strings.HasPrefix(sh.URL, "http://192.168.1.23:9958/s/") || sh.Remaining != 3 || d.PublicBase == "" {
		t.Fatalf("%+v", sh)
	}
}

func TestShareMissingItemAndText(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	resp, b := do(t, c, ts, "POST", "/api/items/nope/share", "{}")
	wantErr(t, resp, b, 404, "not_found")
	resp, _ = do(t, &http.Client{}, ts, "POST", "/api/items/nope/share", "{}")
	if resp.StatusCode != 401 {
		t.Fatalf("anon share %d", resp.StatusCode)
	}
	_, b = do(t, c, ts, "POST", "/api/text", `{"content":"hi there"}`)
	var it struct{ ID string }
	decode(t, b, &it)
	_, b = do(t, c, ts, "POST", "/api/items/"+it.ID+"/share", "{}")
	var sh struct{ ID string }
	decode(t, b, &sh)
	resp, b = do(t, &http.Client{}, ts, "GET", "/s/"+sh.ID+"/raw", "")
	if resp.StatusCode != 200 || string(b) != "hi there" || resp.Header.Get("Content-Disposition") != "inline" ||
		resp.Header.Get("Content-Type") != "text/plain; charset=utf-8" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("text raw %d %q %v", resp.StatusCode, b, resp.Header)
	}
}

func TestQRHandler(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	resp, b := do(t, c, ts, "GET", "/api/qr?data=hello", "")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/svg+xml" || !strings.HasPrefix(string(b), "<svg") {
		t.Fatalf("qr %d %v", resp.StatusCode, resp.Header)
	}
	resp, b = do(t, c, ts, "GET", "/api/qr", "")
	wantErr(t, resp, b, 400, "bad_request")
	resp, b = do(t, c, ts, "GET", "/api/qr?data="+strings.Repeat("a", 1025), "")
	wantErr(t, resp, b, 400, "bad_request")
	resp, _ = do(t, &http.Client{}, ts, "GET", "/api/qr?data=x", "")
	if resp.StatusCode != 401 {
		t.Fatalf("anon qr %d", resp.StatusCode)
	}
}

func newShare(t *testing.T, ts *httptest.Server, c *http.Client, id string) string {
	t.Helper()
	_, b := do(t, c, ts, "POST", "/api/items/"+id+"/share", "{}")
	var sh struct{ ID string }
	decode(t, b, &sh)
	return sh.ID
}

func TestShareHeadDoesNotConsume(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	sid := newShare(t, ts, c, uploadShared(t, ts, c, "hello"))
	anon := &http.Client{}
	for i := 0; i < 2; i++ {
		resp, b := do(t, anon, ts, "HEAD", "/s/"+sid+"/raw", "")
		if resp.StatusCode != 200 || len(b) != 0 || resp.Header.Get("Content-Length") != "5" ||
			!strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
			t.Fatalf("head %d %v", resp.StatusCode, resp.Header)
		}
	}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/raw", "")
	if resp.StatusCode != 200 || string(b) != "hello" {
		t.Fatalf("get %d %q", resp.StatusCode, b)
	}
	resp, _ = do(t, anon, ts, "HEAD", "/s/"+sid+"/raw", "")
	if resp.StatusCode != 410 {
		t.Fatalf("head after use %d", resp.StatusCode)
	}
}

func TestShareRangeIgnored(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	sid := newShare(t, ts, c, uploadShared(t, ts, c, "0123456789abcdef"))
	anon := &http.Client{}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/raw", "", "Range", "bytes=0-9")
	if resp.StatusCode != 200 || string(b) != "0123456789abcdef" || resp.Header.Get("Accept-Ranges") == "bytes" {
		t.Fatalf("range %d %q %v", resp.StatusCode, b, resp.Header)
	}
	resp, b = do(t, anon, ts, "GET", "/s/"+sid+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareMetaText(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	_, b := do(t, c, ts, "POST", "/api/text", `{"content":"hi there"}`)
	var it struct{ ID string }
	decode(t, b, &it)
	sid := newShare(t, ts, c, it.ID)
	resp, b := do(t, &http.Client{}, ts, "GET", "/api/s/"+sid, "")
	var m struct {
		Type, Name, Mime string
		Size, Remaining  int
	}
	decode(t, b, &m)
	if resp.StatusCode != 200 || m.Type != "text" || m.Size != 8 || m.Mime != "text/plain" || m.Remaining != 1 {
		t.Fatalf("meta %d %+v", resp.StatusCode, m)
	}
}

// newMessageShare stages a png and a txt, posts a message and shares it.
func newMessageShare(t *testing.T, ts *httptest.Server, c *http.Client, body string) (itemID, shareID string, atts []store.Attachment) {
	t.Helper()
	a := stage(t, c, ts, "a.png", "image/png", []byte("pngdata"))
	b := stage(t, c, ts, "b.html", "text/html", []byte("<b>x</b>"))
	resp, rb := do(t, c, ts, "POST", "/api/messages", msgBody("hello msg", a.ID, b.ID))
	if resp.StatusCode != 201 {
		t.Fatalf("message %d %s", resp.StatusCode, rb)
	}
	var it store.Item
	decode(t, rb, &it)
	resp, rb = do(t, c, ts, "POST", "/api/items/"+it.ID+"/share", body)
	if resp.StatusCode != 201 {
		t.Fatalf("share %d %s", resp.StatusCode, rb)
	}
	var sh struct {
		ID        string
		Remaining int
	}
	decode(t, rb, &sh)
	if sh.Remaining != -1 {
		t.Fatalf("remaining %d", sh.Remaining)
	}
	return it.ID, sh.ID, it.Attachments
}

func TestShareMessage(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	_, sid, atts := newMessageShare(t, ts, c, `{"max_downloads":1}`)
	anon := &http.Client{}
	for i := 0; i < 3; i++ {
		resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/attachments/"+atts[0].ID+"/raw", "")
		if resp.StatusCode != 200 || string(b) != "pngdata" {
			t.Fatalf("raw %d %q", resp.StatusCode, b)
		}
		if !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
			t.Fatal("expected attachment", resp.Header)
		}
	}
	resp, _ := do(t, anon, ts, "GET", "/s/"+sid+"/attachments/"+atts[0].ID+"/raw?inline=1", "")
	if resp.Header.Get("Content-Disposition") != "inline" {
		t.Fatal("png should be inline", resp.Header)
	}
	resp, _ = do(t, anon, ts, "GET", "/s/"+sid+"/attachments/"+atts[1].ID+"/raw?inline=1", "")
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("html must not be inline", resp.Header)
	}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/attachments/nope/raw", "")
	wantErr(t, resp, b, 410, "gone")
	resp, b = do(t, anon, ts, "GET", "/s/"+sid+"/raw", "")
	wantErr(t, resp, b, 410, "gone")

	resp, b = do(t, anon, ts, "GET", "/api/s/"+sid, "")
	var m struct {
		Type        string
		Content     string
		Attachments []store.Attachment
		Remaining   int
		ExpiresAt   string `json:"expires_at"`
	}
	decode(t, b, &m)
	if resp.StatusCode != 200 || m.Type != "message" || m.Content != "hello msg" || len(m.Attachments) != 2 || m.Remaining != -1 || m.ExpiresAt == "" {
		t.Fatalf("meta %d %s", resp.StatusCode, b)
	}
}

func TestShareMessageGoneAfterDelete(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	id, sid, atts := newMessageShare(t, ts, c, "{}")
	if resp, _ := do(t, c, ts, "DELETE", "/api/items/"+id, ""); resp.StatusCode != 204 {
		t.Fatal(resp.StatusCode)
	}
	anon := &http.Client{}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/attachments/"+atts[0].ID+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
	resp, b = do(t, anon, ts, "GET", "/api/s/"+sid, "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareMessageExpired(t *testing.T) {
	var mu sync.Mutex
	now := time.Now()
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	ts, _ := newTestServer(t, func(d *Deps) { d.Share = share.New(clock) })
	c := login(t, ts)
	_, sid, atts := newMessageShare(t, ts, c, `{"ttl_seconds":60}`)
	mu.Lock()
	now = now.Add(2 * time.Minute)
	mu.Unlock()
	anon := &http.Client{}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/attachments/"+atts[0].ID+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
	resp, b = do(t, anon, ts, "GET", "/api/s/"+sid, "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareFileStillCounts(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	sid := newShare(t, ts, c, uploadShared(t, ts, c, "hello"))
	anon := &http.Client{}
	if resp, _ := do(t, anon, ts, "GET", "/s/"+sid+"/raw", ""); resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	resp, b := do(t, anon, ts, "GET", "/s/"+sid+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareFileUnlimitedClamped(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	id := uploadShared(t, ts, c, "hello")
	resp, b := do(t, c, ts, "POST", "/api/items/"+id+"/share", `{"max_downloads":-1}`)
	var sh struct {
		ID        string
		Remaining int
	}
	decode(t, b, &sh)
	if resp.StatusCode != 201 || sh.Remaining != 1 {
		t.Fatalf("share %d %s", resp.StatusCode, b)
	}
	anon := &http.Client{}
	if resp, _ := do(t, anon, ts, "GET", "/s/"+sh.ID+"/raw", ""); resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	resp, b = do(t, anon, ts, "GET", "/s/"+sh.ID+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
}

func TestShareMessageCannotReachOtherMessageAttachment(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	_, sidA, _ := newMessageShare(t, ts, c, "{}")
	_, _, attsB := newMessageShare(t, ts, c, "{}")
	resp, b := do(t, &http.Client{}, ts, "GET", "/s/"+sidA+"/attachments/"+attsB[0].ID+"/raw", "")
	wantErr(t, resp, b, 410, "gone")
}
