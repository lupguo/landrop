package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/config"
	"github.com/lupguo/landrop/internal/hub"
	"github.com/lupguo/landrop/internal/sendcli"
	"github.com/lupguo/landrop/internal/share"
	"github.com/lupguo/landrop/internal/store"
	"github.com/lupguo/landrop/internal/termui"
)

func TestListenSkipsBusyPort(t *testing.T) {
	l1, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	p := l1.Addr().(*net.TCPAddr).Port
	l2, got, err := Listen(p, 3)
	if err != nil || got == p {
		t.Fatal(got, err)
	}
	if l2.Addr().(*net.TCPAddr).Port != got {
		t.Fatal("port mismatch")
	}
	l2.Close()
}

func TestListenExhausted(t *testing.T) {
	var p int
	var ls []net.Listener
	for tries := 0; tries < 20 && ls == nil; tries++ {
		l1, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			t.Fatal(err)
		}
		p = l1.Addr().(*net.TCPAddr).Port
		l2, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p+1))
		if err != nil {
			l1.Close()
			continue
		}
		ls = []net.Listener{l1, l2}
	}
	if ls == nil {
		t.Skip("could not occupy two adjacent ports")
	}
	defer ls[0].Close()
	defer ls[1].Close()
	if l, _, err := Listen(p, 2); err == nil {
		l.Close()
		t.Fatal("want error")
	}
}

func TestSweepOnce(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	st, err := store.New(t.TempDir(), time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	h := hub.New(16)
	sh := share.New(clock)
	it, _ := st.AddText(store.From{}, "x")
	l := sh.Create(it.ID, 1, time.Hour)
	sub, _, _ := h.Subscribe(hub.Peer{ID: "p"}, 0)
	defer h.Unsubscribe(sub)

	now = now.Add(2 * time.Minute)
	sweepOnce(st, sh, h)
	if _, ok := st.Get(it.ID); ok {
		t.Fatal("item not swept")
	}
	if _, err := sh.Peek(l.ID); err == nil {
		t.Fatal("share link survived its item")
	}
	select {
	case ev := <-sub.C:
		if ev.Type != "item.deleted" || ev.Data.(map[string]string)["id"] != it.ID {
			t.Fatalf("event %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no item.deleted")
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func poll(t *testing.T, d time.Duration, what string, f func() bool) {
	t.Helper()
	end := time.Now().Add(d)
	for !f() {
		if time.Now().After(end) {
			t.Fatal("timeout waiting for " + what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestRunEndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
	// Hermetic: never touch real mDNS.
	oldAnnounce := announce
	announce = func(string, int, []net.IP) (func(), error) { return nil, errors.New("mdns disabled in test") }
	t.Cleanup(func() { announce = oldAnnounce })
	banners := make(chan termui.BannerInfo, 1)
	oldPrint := printBanner
	printBanner = func(w io.Writer, b termui.BannerInfo) { banners <- b; oldPrint(w, b) }
	t.Cleanup(func() { printBanner = oldPrint })

	// A private temp root so the store dir can be found (and never leaks).
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if runtime.GOOS == "windows" {
		t.Setenv("TMP", tmp)
		t.Setenv("TEMP", tmp)
	}

	cfg := config.Config{Port: freePort(t), Name: "drop", TTL: time.Hour, MaxSize: 1 << 30, NoOpen: true, Lang: "en"}
	out := &syncBuf{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, out) }()
	t.Cleanup(func() {
		// On early failure still stop Run (t.TempDir removes the store dir).
		cancel()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
		}
	})

	instPath := filepath.Join(home, ".landrop", "instance.json")
	poll(t, 3*time.Second, "instance file", func() bool { return exists(instPath) })
	// The store dir is unpredictable (os.MkdirTemp), not landrop-<pid>.
	dirs, _ := filepath.Glob(filepath.Join(tmp, "landrop-*"))
	if len(dirs) != 1 || filepath.Base(dirs[0]) == fmt.Sprintf("landrop-%d", os.Getpid()) {
		t.Fatalf("store dirs %v", dirs)
	}
	storeDir := dirs[0]

	// banner: code, LAN token URL, version; mDNS failure is reported, not fatal
	var b termui.BannerInfo
	select {
	case b = <-banners:
	case <-time.After(3 * time.Second):
		t.Fatal("no banner")
	}
	m := regexp.MustCompile(`^http://[0-9.]+:(\d+)/\?t=([0-9a-f]{32})$`).FindStringSubmatch(b.TokenURL)
	// freePort's port may be taken by a concurrent test process before Run
	// listens; Listen then moves up, so the bound port is the source of truth.
	if m == nil || m[1] != fmt.Sprint(b.Port) || len(b.Code) != 4 || b.MDNSErr == nil || b.MDNSHost != "" || b.Version == "" || b.Port < cfg.Port || b.Port >= cfg.Port+10 {
		t.Fatalf("banner %+v (requested port %d)", b, cfg.Port)
	}
	if !strings.Contains(out.String(), "Access code: "+b.Code) {
		t.Fatalf("banner output:\n%s", out.String())
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/?t=%s", b.Port, m[2]))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatal("token login status", resp.StatusCode)
	}
	// Hold an SSE stream open across shutdown: it must not stall Run.
	evReq, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/events", b.Port), nil)
	for _, ck := range resp.Cookies() {
		evReq.AddCookie(ck)
	}
	evResp, err := http.DefaultClient.Do(evReq)
	if err != nil || evResp.StatusCode != 200 {
		t.Fatal("events", err)
	}
	defer evResp.Body.Close()

	var so bytes.Buffer
	if err := sendcli.Run([]string{"-"}, strings.NewReader("hello"), &so, "en"); err != nil {
		t.Fatal(err, so.String())
	}
	if !strings.Contains(so.String(), "Sent.") {
		t.Fatalf("send output %q", so.String())
	}

	src := filepath.Join(t.TempDir(), "x.log")
	os.WriteFile(src, []byte("log line\n"), 0o600)
	so.Reset()
	if err := sendcli.Run([]string{src}, nil, &so, "en"); err != nil {
		t.Fatal(err, so.String())
	}
	// stdin beyond the text limit becomes a stdin.txt upload
	big := strings.Repeat("z", store.MaxText+10)
	if err := sendcli.Run([]string{"-"}, strings.NewReader(big), &so, "en"); err != nil {
		t.Fatal(err, so.String())
	}

	saveDir := filepath.Join(home, "Downloads", "LanDrop")
	poll(t, 2*time.Second, "autosaved x.log", func() bool {
		b, err := os.ReadFile(filepath.Join(saveDir, "x.log"))
		return err == nil && string(b) == "log line\n"
	})
	poll(t, 2*time.Second, "autosaved stdin.txt", func() bool {
		st, err := os.Stat(filepath.Join(saveDir, "stdin.txt"))
		return err == nil && st.Size() == int64(len(big))
	})

	if !exists(storeDir) {
		t.Fatal("store dir missing while running")
	}
	cancel()
	select {
	case err := <-done:
		done <- nil // let the cleanup's receive succeed
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return")
	}
	if exists(storeDir) {
		t.Fatal("store dir not removed")
	}
	if exists(instPath) {
		t.Fatal("instance file not removed")
	}
	if !exists(filepath.Join(saveDir, "x.log")) {
		t.Fatal("exit cleanup touched autosave dir")
	}
	if !strings.Contains(out.String(), "Stopped. Temporary files removed.") {
		t.Fatalf("no bye line:\n%s", out.String())
	}
	so.Reset()
	if err := sendcli.Run([]string{"-"}, strings.NewReader("x"), &so, "en"); err == nil || !strings.Contains(so.String(), "not running") {
		t.Fatalf("after exit: %v %q", err, so.String())
	}
}

func TestRunPortExhausted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	p := l.Addr().(*net.TCPAddr).Port
	old := listenTries
	listenTries = 1
	defer func() { listenTries = old }()
	err = Run(context.Background(), config.Config{Port: p, Name: "drop", TTL: time.Hour, NoOpen: true, Lang: "en"}, &syncBuf{})
	if err == nil || !strings.Contains(err.Error(), "--port") {
		t.Fatal(err)
	}
}

// An IPv4-only listener from another program must count as "busy"; a
// dual-stack [::] socket would silently coexist with it on macOS while IPv4
// clients keep reaching the other program.
func TestListenSkipsPortHeldByIPv4Socket(t *testing.T) {
	l1, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	p := l1.Addr().(*net.TCPAddr).Port
	l2, got, err := Listen(p, 3)
	if err != nil || got == p {
		t.Fatal(got, err)
	}
	l2.Close()
}

// Another program holding only 127.0.0.1:p must make p busy: otherwise the
// browser launch (?t=<token>) and `landrop send` (secret header), which both
// dial 127.0.0.1, would reach that program instead of LanDrop.
func TestListenSkipsPortHeldOnLoopback(t *testing.T) {
	l1, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	p := l1.Addr().(*net.TCPAddr).Port
	l2, got, err := Listen(p, 3)
	if err != nil || got == p {
		t.Fatal(got, err)
	}
	l2.Close()
}

func TestHostnameFallback(t *testing.T) {
	old := osHostname
	defer func() { osHostname = old }()
	osHostname = func() (string, error) { return "", errors.New("boom") }
	if h := hostname(); h != "localhost" {
		t.Fatal(h)
	}
	osHostname = func() (string, error) { return "mac", nil }
	if h := hostname(); h != "mac" {
		t.Fatal(h)
	}
}
