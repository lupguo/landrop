package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/store"
)

func localDo(t *testing.T, ts *httptest.Server, path, body, secret string) (*http.Response, []byte) {
	t.Helper()
	hdr := []string{"Content-Type", "application/octet-stream"}
	if secret != "" {
		hdr = append(hdr, "X-LanDrop-Secret", secret)
	}
	// The local API ignores Origin entirely, so send a foreign one.
	hdr = append(hdr, "Origin", "http://evil.example")
	return do(t, http.DefaultClient, ts, "POST", path, body, hdr...)
}

func TestLocalAPI(t *testing.T) {
	stored := make(chan store.Item, 1)
	ts, d := newTestServer(t, func(d *Deps) {
		d.LocalSecret = "topsecret"
		d.OnFileStored = func(it store.Item) { stored <- it }
	})
	viewer := login(t, ts)
	s := openSSE(t, viewer, ts, "Phone", "")

	resp, b := localDo(t, ts, "/api/local/text", "hello", "")
	wantErr(t, resp, b, 403, "forbidden")
	resp, b = localDo(t, ts, "/api/local/text", "hello", "wrong")
	wantErr(t, resp, b, 403, "forbidden")

	resp, b = localDo(t, ts, "/api/local/text", "hello", "topsecret")
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
	var it store.Item
	decode(t, b, &it)
	if it.Content != "hello" || it.From.PeerID != "cli" || it.From.Name != "testhost (CLI)" {
		t.Fatalf("item %+v", it)
	}
	m := s.next(t, time.Second, isType("item.created"))
	var ev store.Item
	decode(t, []byte(m.Data), &ev)
	if ev.ID != it.ID || !strings.HasSuffix(ev.From.Name, " (CLI)") {
		t.Fatalf("event %+v", ev)
	}

	resp, b = localDo(t, ts, "/api/local/text", "", "topsecret")
	wantErr(t, resp, b, 400, "empty")
	resp, b = localDo(t, ts, "/api/local/text", strings.Repeat("a", store.MaxText+1), "topsecret")
	wantErr(t, resp, b, 413, "too_large")

	// upload
	resp, b = localDo(t, ts, "/api/local/upload?name=x.log", "file-bytes", "topsecret")
	if resp.StatusCode != 201 {
		t.Fatalf("upload status %d %s", resp.StatusCode, b)
	}
	decode(t, b, &it)
	// The hook is called synchronously before the response, so the app can
	// register the save before shutdown can race it.
	select {
	case got := <-stored:
		stored <- got
	default:
		t.Fatal("OnFileStored not called before the response")
	}
	if it.Type != store.KindFile || it.File.Name != "x.log" || it.File.Size != 10 || it.From.Name != "testhost (CLI)" {
		t.Fatalf("upload item %+v", it)
	}
	s.next(t, time.Second, isType("item.created"))
	select {
	case got := <-stored:
		if got.ID != it.ID {
			t.Fatal("OnFileStored item mismatch")
		}
	case <-time.After(time.Second):
		t.Fatal("OnFileStored not called")
	}
	data, _ := os.ReadFile(filepath.Join(d.Store.Dir(), it.ID))
	if string(data) != "file-bytes" {
		t.Fatalf("stored %q", data)
	}
	resp, b = localDo(t, ts, "/api/local/upload?name=x.log", "file-bytes", "")
	wantErr(t, resp, b, 403, "forbidden")
}

func TestLocalAPIUploadTooLarge(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) { d.LocalSecret = "k"; d.MaxSize = 4 })
	resp, b := localDo(t, ts, "/api/local/upload?name=big.bin", "12345", "k")
	wantErr(t, resp, b, 413, "too_large")
}

func TestLocalAPIRejectsNonLoopbackAndEmptySecret(t *testing.T) {
	// Even with the right secret, a non-loopback RemoteAddr is refused.
	_, d := newTestServer(t, func(d *Deps) { d.LocalSecret = "k" })
	h := New(*d)
	req := httptest.NewRequest("POST", "/api/local/text", strings.NewReader("hi"))
	req.RemoteAddr = "192.168.1.50:5555"
	req.Header.Set("X-LanDrop-Secret", "k")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("non-loopback: %d", rec.Code)
	}
	// A server with no secret configured never accepts the local API.
	_, d2 := newTestServer(t, nil)
	req = httptest.NewRequest("POST", "/api/local/text", strings.NewReader("hi"))
	req.RemoteAddr = "127.0.0.1:5555"
	rec = httptest.NewRecorder()
	New(*d2).ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("empty secret: %d", rec.Code)
	}
}
