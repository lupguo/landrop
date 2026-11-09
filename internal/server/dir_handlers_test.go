package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirEndpoints(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0600)
	os.MkdirAll(filepath.Join(root, "sub"), 0700)
	os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("bb"), 0600)

	ts, _ := newTestServer(t, func(d *Deps) { d.Dir = root })
	c := login(t, ts)

	resp, b := do(t, c, ts, "GET", "/api/dir", "")
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode, string(b))
	}
	var l struct {
		Path    string
		Entries []struct {
			Name  string
			IsDir bool `json:"is_dir"`
		}
	}
	decode(t, b, &l)
	if l.Path != "" || len(l.Entries) != 2 || l.Entries[0].Name != "sub" || !l.Entries[0].IsDir {
		t.Fatalf("%+v", l)
	}
	resp, b = do(t, c, ts, "GET", "/api/dir?path=sub", "")
	decode(t, b, &l)
	if resp.StatusCode != 200 || l.Path != "sub" || len(l.Entries) != 1 || l.Entries[0].Name != "b.txt" {
		t.Fatalf("%d %+v", resp.StatusCode, l)
	}
	resp, b = do(t, c, ts, "GET", "/api/dir/raw?path=a.txt", "")
	if resp.StatusCode != 200 || string(b) != "hello" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("%d %q %v", resp.StatusCode, b, resp.Header)
	}
	if resp, _ = do(t, c, ts, "GET", "/api/dir/raw?path=../x", ""); resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
	if resp, _ = do(t, c, ts, "GET", "/api/dir/raw?path=sub", ""); resp.StatusCode != 400 {
		t.Fatal(resp.StatusCode)
	}
	resp, b = do(t, c, ts, "GET", "/api/dir?path=a.txt", "")
	wantErr(t, resp, b, 400, "not_directory")
	if resp, _ = do(t, c, ts, "GET", "/api/dir?path=nope", ""); resp.StatusCode != 404 {
		t.Fatal(resp.StatusCode)
	}

	ts2, _ := newTestServer(t, nil)
	c2 := login(t, ts2)
	for _, u := range []string{"/api/dir", "/api/dir/raw?path=a.txt"} {
		if resp, _ = do(t, c2, ts2, "GET", u, ""); resp.StatusCode != 404 {
			t.Fatal(u, resp.StatusCode)
		}
	}
}

func TestDirLoopAndEscapeStatuses(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	outside, _ := filepath.EvalSymlinks(t.TempDir())
	os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0600)
	if os.Symlink(filepath.Join(root, "l2"), filepath.Join(root, "l1")) != nil {
		t.Skip("no symlinks")
	}
	os.Symlink(filepath.Join(root, "l1"), filepath.Join(root, "l2"))
	os.Symlink(outside, filepath.Join(root, "link"))
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0600)
	ts, _ := newTestServer(t, func(d *Deps) { d.Dir = root })
	c := login(t, ts)
	for u, want := range map[string]int{
		"/api/dir?path=l1/x":            400,
		"/api/dir/raw?path=l1":          400,
		"/api/dir?path=link/secret/x":   403,
		"/api/dir?path=link/nope/x":     403,
		"/api/dir/raw?path=link/secret": 403,
		"/api/dir?path=a.txt/x":         404,
		"/api/dir/raw?path=a.txt":       200,
	} {
		if resp, _ := do(t, c, ts, "GET", u, ""); resp.StatusCode != want {
			t.Fatal(u, resp.StatusCode, want)
		}
	}
}
