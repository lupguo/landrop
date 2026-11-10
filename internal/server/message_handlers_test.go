package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/store"
)

func stage(t *testing.T, c *http.Client, ts *httptest.Server, name, ctype string, data []byte) store.Attachment {
	t.Helper()
	body, ct := multipartBody(t, name, ctype, data)
	req, _ := http.NewRequest("POST", ts.URL+"/api/attachments", body)
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", ct)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var a store.Attachment
	if resp.StatusCode != 201 {
		t.Fatalf("stage status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatal(err)
	}
	return a
}

func msgBody(content string, ids ...string) string {
	b, _ := json.Marshal(map[string]any{"content": content, "attachments": ids})
	return string(b)
}

func TestStageThenMessage(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	sse := openSSE(t, c, ts, "x", "")
	a := stage(t, c, ts, "a.png", "image/png", []byte("aaa"))
	b := stage(t, c, ts, "b.txt", "text/plain", []byte("bb"))
	_, lb := do(t, c, ts, "GET", "/api/items", "")
	var l struct{ Items []store.Item }
	decode(t, lb, &l)
	if len(l.Items) != 0 {
		t.Fatalf("staging published items: %d", len(l.Items))
	}
	resp, body := do(t, c, ts, "POST", "/api/messages", msgBody("hi", a.ID, b.ID))
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, body)
	}
	ev := sse.next(t, 3*time.Second, isType("item.created"))
	var it store.Item
	decode(t, []byte(ev.Data), &it)
	if it.Type != store.KindMessage || len(it.Attachments) != 2 || it.Content != "hi" {
		t.Fatalf("event item %+v", it)
	}
	_, lb = do(t, c, ts, "GET", "/api/items", "")
	decode(t, lb, &l)
	if len(l.Items) != 1 {
		t.Fatalf("items %d", len(l.Items))
	}
	// no second item.created
	select {
	case m := <-sse.ch:
		if m.Event == "item.created" {
			t.Fatal("duplicate item.created")
		}
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMessageForeignStagingRejected(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	a, b := login(t, ts), login(t, ts)
	att := stage(t, a, ts, "a.txt", "", []byte("x"))
	resp, body := do(t, b, ts, "POST", "/api/messages", msgBody("", att.ID))
	wantErr(t, resp, body, 400, "bad_attachment")
}

func TestMessageValidation(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	resp, b := do(t, c, ts, "POST", "/api/messages", msgBody("hi"))
	wantErr(t, resp, b, 400, "empty")
	ids := make([]string, 21)
	for i := range ids {
		ids[i] = fmt.Sprint("x", i)
	}
	resp, b = do(t, c, ts, "POST", "/api/messages", msgBody("hi", ids...))
	wantErr(t, resp, b, 400, "too_many")
	a := stage(t, c, ts, "a.txt", "", []byte("x"))
	resp, b = do(t, c, ts, "POST", "/api/messages", msgBody(strings.Repeat("a", store.MaxText+1), a.ID))
	wantErr(t, resp, b, 413, "too_large")
}

func TestStageTooLargeAndOrigin(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) { d.MaxSize = 4 })
	c := login(t, ts)
	body, ct := multipartBody(t, "a.bin", "", []byte("12345"))
	req, _ := http.NewRequest("POST", ts.URL+"/api/attachments", body)
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", ct)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, ct = multipartBody(t, "a.bin", "", []byte("1"))
	req, _ = http.NewRequest("POST", ts.URL+"/api/attachments", body)
	req.Header.Set("Content-Type", ct)
	resp, err = c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAttachmentRaw(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	png := stage(t, c, ts, "p.png", "image/png", []byte("png"))
	html := stage(t, c, ts, "x.html", "text/html", []byte("<b>"))
	pdf := stage(t, c, ts, `报告 "a".pdf`, "application/pdf", []byte("pdf"))
	_, b := do(t, c, ts, "POST", "/api/messages", msgBody("", png.ID, html.ID, pdf.ID))
	var it store.Item
	decode(t, b, &it)
	raw := func(aid, q string) *http.Response {
		r, _ := do(t, c, ts, "GET", "/api/items/"+it.ID+"/attachments/"+aid+"/raw"+q, "")
		return r
	}
	if cd := raw(png.ID, "?inline=1").Header.Get("Content-Disposition"); cd != "inline" {
		t.Fatalf("png cd %q", cd)
	}
	if cd := raw(png.ID, "").Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Fatalf("png no inline cd %q", cd)
	}
	if cd := raw(html.ID, "?inline=1").Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") {
		t.Fatalf("html cd %q", cd)
	}
	if cd := raw(pdf.ID, "").Header.Get("Content-Disposition"); !strings.Contains(cd, "filename*=UTF-8''") {
		t.Fatalf("pdf cd %q", cd)
	}
	if r := raw("nope", ""); r.StatusCode != 404 {
		t.Fatalf("status %d", r.StatusCode)
	}
}

func TestMessageAutosaveHook(t *testing.T) {
	var mu sync.Mutex
	var got []store.Item
	ts, d := newTestServer(t, func(d *Deps) {
		d.OnFileStored = func(it store.Item) { mu.Lock(); got = append(got, it); mu.Unlock() }
	})
	c := login(t, ts)
	a := stage(t, c, ts, "a.txt", "", []byte("x"))
	b := stage(t, c, ts, "b.txt", "", []byte("y"))
	resp, body := do(t, c, ts, "POST", "/api/messages", msgBody("", a.ID, b.ID))
	if resp.StatusCode != 201 {
		t.Fatalf("status %d %s", resp.StatusCode, body)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 {
		t.Fatalf("calls %d", len(got))
	}
	for i, id := range []string{a.ID, b.ID} {
		if got[i].ID != id || got[i].Type != store.KindFile || got[i].File == nil {
			t.Fatalf("call %d: %+v", i, got[i])
		}
		if _, err := os.Stat(filepath.Join(d.Store.Dir(), id)); err != nil {
			t.Fatal(err)
		}
	}
}
