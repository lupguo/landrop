package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

// get sends a GET with an explicit Host header.
func getWithHost(t *testing.T, c *http.Client, url, host string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Host = host
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestNoAuthRejectsForeignHost(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) { d.NoAuth = true })
	for _, p := range []string{"/api/items", "/", "/events", "/api/me", "/s/abc"} {
		resp, b := getWithHost(t, &http.Client{}, ts.URL+p, "attacker.example:9958")
		wantErr(t, resp, b, http.StatusMisdirectedRequest, "bad_host")
		if resp.Header.Get("Set-Cookie") != "" {
			t.Fatalf("%s: cookie issued for foreign host", p)
		}
	}
}

func TestNoAuthAllowsKnownHosts(t *testing.T) {
	ts, _ := newTestServer(t, func(d *Deps) {
		d.NoAuth = true
		d.Hostname = "MacBook-Pro"
		d.MDNSURL = "http://drop.local:9958"
	})
	for _, h := range []string{
		"127.0.0.1:9958", "192.168.1.23", "[::1]:9958", "[fe80::1%25en0]:9958",
		"localhost:9958", "LOCALHOST", "drop.local:9958", "DROP.LOCAL",
		"macbook-pro", "MacBook-Pro.local:9958",
	} {
		resp, b := getWithHost(t, &http.Client{}, ts.URL+"/api/items", h)
		if resp.StatusCode != 200 {
			t.Fatalf("host %q: status %d %s", h, resp.StatusCode, b)
		}
	}
	for _, h := range []string{"drop.local.evil.example", "evil.local", "macbook-pro.evil", "localhost.evil"} {
		resp, b := getWithHost(t, &http.Client{}, ts.URL+"/api/items", h)
		wantErr(t, resp, b, http.StatusMisdirectedRequest, "bad_host")
	}
}

func TestCodeModeIgnoresHost(t *testing.T) {
	ts, _ := newTestServer(t, nil)
	c := login(t, ts)
	u, _ := url.Parse(ts.URL)
	cookies := c.Jar.Cookies(u)
	if len(cookies) == 0 {
		t.Fatal("no session cookie")
	}
	req, _ := http.NewRequest("GET", ts.URL+"/api/items", nil)
	req.Host = "attacker.example:9958"
	req.AddCookie(cookies[0])
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d %s", resp.StatusCode, b)
	}
}

// Every write route rejects a missing or foreign Origin with 403, and (in
// NoAuth mode) does so before a session is created.
func TestWriteRoutesRequireOrigin(t *testing.T) {
	routes := []struct{ method, path, body string }{
		{"POST", "/api/text", `{"content":"hi"}`},
		{"POST", "/api/upload", ""},
		{"POST", "/api/items/x/share", `{}`},
		{"POST", "/api/auth", `{"code":"0000"}`},
		{"DELETE", "/api/items/x", ""},
		{"DELETE", "/api/items", ""},
		{"DELETE", "/api/peers/x", ""},
	}
	for _, noAuth := range []bool{false, true} {
		ts, _ := newTestServer(t, func(d *Deps) { d.NoAuth = noAuth })
		var c *http.Client
		if noAuth {
			jar, _ := cookiejar.New(nil)
			c = &http.Client{Jar: jar}
		} else {
			c = login(t, ts)
		}
		for _, rt := range routes {
			for _, origin := range []string{"", "http://evil.example", "null"} {
				var rd io.Reader
				if rt.body != "" {
					rd = strings.NewReader(rt.body)
				}
				req, _ := http.NewRequest(rt.method, ts.URL+rt.path, rd)
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				resp, err := c.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode != 403 {
					t.Fatalf("noAuth=%v %s %s origin %q: status %d %s", noAuth, rt.method, rt.path, origin, resp.StatusCode, b)
				}
				if resp.Header.Get("Set-Cookie") != "" {
					t.Fatalf("noAuth=%v %s %s: session created by cross-origin write", noAuth, rt.method, rt.path)
				}
			}
		}
	}
}
