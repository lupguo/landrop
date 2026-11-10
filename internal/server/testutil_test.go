package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/auth"
	"github.com/lupguo/landrop/internal/hub"
	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
)

const testHTML = "<!doctype html><html><body>spa</body></html>"

var testDeps sync.Map // *httptest.Server -> *Deps

func newTestServer(t *testing.T, mut func(*Deps)) (*httptest.Server, *Deps) {
	t.Helper()
	st, err := store.New(t.TempDir(), 24*time.Hour, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	d := &Deps{
		Store:      st,
		Hub:        hub.New(256),
		Auth:       auth.New(time.Now),
		Share:      share.New(time.Now),
		MaxSize:    4 << 30,
		PublicBase: "http://192.168.1.23:9958",
		MDNSURL:    "http://drop.local:9958",
		Hostname:   "testhost",
		Now:        time.Now,
		Web: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, testHTML)
		}),
	}
	if mut != nil {
		mut(d)
	}
	ts := httptest.NewServer(New(*d))
	testDeps.Store(ts, d)
	t.Cleanup(func() {
		ts.CloseClientConnections()
		ts.Close()
		testDeps.Delete(ts)
		st.Close(false)
	})
	return ts, d
}

func depsOf(t *testing.T, ts *httptest.Server) *Deps {
	t.Helper()
	v, ok := testDeps.Load(ts)
	if !ok {
		t.Fatal("unknown test server")
	}
	return v.(*Deps)
}

// login returns a cookie-jar client logged in via the link token.
func login(t *testing.T, ts *httptest.Server) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	resp, err := c.Get(ts.URL + "/?t=" + depsOf(t, ts).Auth.Token())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login: status %d", resp.StatusCode)
	}
	return c
}

// headerRT adds fixed headers to every request.
type headerRT struct {
	h  http.Header
	rt http.RoundTripper
}

func (h headerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h.h {
		r.Header[k] = v
	}
	return h.rt.RoundTrip(r)
}

// do sends a request with Origin set to ts.URL and returns status and body.
func do(t *testing.T, c *http.Client, ts *httptest.Server, method, path, body string, hdr ...string) (*http.Response, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", ts.URL)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func decode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode %q: %v", b, err)
	}
}

func wantErr(t *testing.T, resp *http.Response, b []byte, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status %d, want %d (body %s)", resp.StatusCode, status, b)
	}
	var e struct{ Error string }
	decode(t, b, &e)
	if e.Error != code {
		t.Fatalf("error %q, want %q", e.Error, code)
	}
}

type sseMsg struct {
	ID, Event, Data, Comment, Retry string
}

type sseStream struct {
	resp *http.Response
	ch   chan sseMsg
}

// openSSE connects to /events. lastID may be "".
func openSSE(t *testing.T, c *http.Client, ts *httptest.Server, name, lastID string) *sseStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/events?name="+name, nil)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	cl := *c
	cl.Timeout = 0
	resp, err := cl.Do(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("events: status %d", resp.StatusCode)
	}
	s := &sseStream{resp: resp, ch: make(chan sseMsg, 64)}
	go func() {
		defer close(s.ch)
		br := bufio.NewReader(resp.Body)
		var m sseMsg
		has := false
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\n")
			switch {
			case line == "":
				if has {
					s.ch <- m
				}
				m, has = sseMsg{}, false
			case strings.HasPrefix(line, ":"):
				s.ch <- sseMsg{Comment: strings.TrimSpace(line[1:])}
			default:
				k, v, _ := strings.Cut(line, ":")
				v = strings.TrimPrefix(v, " ")
				has = true
				switch k {
				case "id":
					m.ID = v
				case "event":
					m.Event = v
				case "data":
					m.Data = v
				case "retry":
					m.Retry = v
				}
			}
		}
	}()
	return s
}

// next returns the next message matching keep within d.
func (s *sseStream) next(t *testing.T, d time.Duration, keep func(sseMsg) bool) sseMsg {
	t.Helper()
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case m, ok := <-s.ch:
			if !ok {
				t.Fatal("sse stream closed")
			}
			if keep(m) {
				return m
			}
		case <-timer.C:
			t.Fatal("sse timeout")
		}
	}
}

func isEvent(m sseMsg) bool { return m.Event != "" }

func isType(typ string) func(sseMsg) bool { return func(m sseMsg) bool { return m.Event == typ } }
