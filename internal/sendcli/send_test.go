package sendcli

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lupguo/landrop/internal/i18n"
	"github.com/lupguo/landrop/internal/instance"
	"github.com/lupguo/landrop/internal/store"
)

func setHome(t *testing.T) string {
	h := t.TempDir()
	t.Setenv("HOME", h)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", h)
	}
	return h
}

type got struct {
	path, name, secret string
	body               []byte
}

func fakeServer(t *testing.T, status int, resp string) (chan got, int) {
	ch := make(chan got, 4)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ch <- got{r.URL.Path, r.URL.Query().Get("name"), r.Header.Get("X-LanDrop-Secret"), b}
		w.WriteHeader(status)
		io.WriteString(w, resp)
	}))
	t.Cleanup(ts.Close)
	port := ts.Listener.Addr().(*net.TCPAddr).Port
	if err := instance.Write(instance.DefaultPath(), instance.Info{PID: 1, Port: port, Secret: "sek"}); err != nil {
		t.Fatal(err)
	}
	return ch, port
}

func TestSendTextAndFile(t *testing.T) {
	setHome(t)
	ch, _ := fakeServer(t, 201, `{"id":"x"}`)
	var out bytes.Buffer
	if err := Run([]string{"-"}, strings.NewReader("hi there"), &out, "en"); err != nil {
		t.Fatal(err)
	}
	g := <-ch
	if g.path != "/api/local/text" || g.secret != "sek" || string(g.body) != "hi there" {
		t.Fatalf("%+v", g)
	}
	if strings.TrimSpace(out.String()) != "Sent." {
		t.Fatalf("out %q", out.String())
	}

	f := filepath.Join(t.TempDir(), "report.pdf")
	os.WriteFile(f, []byte("PDF"), 0o600)
	if err := Run([]string{f}, nil, &out, "en"); err != nil {
		t.Fatal(err)
	}
	g = <-ch
	if g.path != "/api/local/upload" || g.name != "report.pdf" || string(g.body) != "PDF" {
		t.Fatalf("%+v", g)
	}
}

func TestSendBigStdinStreamsAsUpload(t *testing.T) {
	setHome(t)
	ch, _ := fakeServer(t, 201, `{}`)
	big := strings.Repeat("q", store.MaxText+1)
	if err := Run([]string{"-"}, strings.NewReader(big), io.Discard, "en"); err != nil {
		t.Fatal(err)
	}
	g := <-ch
	if g.path != "/api/local/upload" || g.name != "stdin.txt" || string(g.body) != big {
		t.Fatalf("path %s name %s len %d", g.path, g.name, len(g.body))
	}
}

func TestSendServerError(t *testing.T) {
	setHome(t)
	fakeServer(t, 403, `{"error":"forbidden"}`)
	var out bytes.Buffer
	err := Run([]string{"-"}, strings.NewReader("x"), &out, "en")
	if err == nil || !strings.Contains(out.String(), "forbidden") {
		t.Fatal(err, out.String())
	}
}

func TestSendNotRunning(t *testing.T) {
	setHome(t)
	var out bytes.Buffer
	if err := Run([]string{"-"}, strings.NewReader("x"), &out, "zh"); err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(out.String(), "LanDrop") {
		t.Fatalf("out %q", out.String())
	}

	// stale instance file pointing at a closed port
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	instance.Write(instance.DefaultPath(), instance.Info{PID: 1, Port: port, Secret: "s"})
	out.Reset()
	if err := Run([]string{"-"}, strings.NewReader("x"), &out, "en"); err == nil || !strings.Contains(out.String(), "not running") {
		t.Fatal(err, out.String())
	}
}

func TestSendUsageAndMissingFile(t *testing.T) {
	setHome(t)
	fakeServer(t, 201, `{}`)
	var out bytes.Buffer
	if err := Run(nil, nil, &out, "zh"); err == nil || !strings.Contains(out.String(), i18n.T("zh", i18n.KeySendUsage)) {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if err := Run([]string{filepath.Join(t.TempDir(), "nope")}, nil, &out, "en"); err == nil || !strings.HasPrefix(out.String(), "Send failed: ") {
		t.Fatal("want error for missing file", out.String())
	}
	out.Reset()
	d := t.TempDir()
	if err := Run([]string{d}, nil, &out, "en"); err == nil || !strings.Contains(out.String(), i18n.T("en", i18n.KeySendIsDir, d)) {
		t.Fatal(err, out.String())
	}
}
