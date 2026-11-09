package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if !strings.HasPrefix(sh.URL, "http://192.168.1.23:8848/s/") || sh.Remaining != 3 || d.PublicBase == "" {
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
