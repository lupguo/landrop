package server

import (
	"os"
	"path/filepath"
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
	if resp.StatusCode != 200 || string(b) != "hello" || resp.Header.Get("Content-Disposition") == "" {
		t.Fatalf("%d %q %v", resp.StatusCode, b, resp.Header)
	}
	if resp, _ = do(t, c, ts, "GET", "/api/dir/raw?path=../x", ""); resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
	if resp, _ = do(t, c, ts, "GET", "/api/dir/raw?path=sub", ""); resp.StatusCode != 400 {
		t.Fatal(resp.StatusCode)
	}
	if resp, _ = do(t, c, ts, "GET", "/api/dir?path=nope", ""); resp.StatusCode != 404 {
		t.Fatal(resp.StatusCode)
	}

	ts2, _ := newTestServer(t, nil)
	c2 := login(t, ts2)
	if resp, _ = do(t, c2, ts2, "GET", "/api/dir", ""); resp.StatusCode != 404 {
		t.Fatal(resp.StatusCode)
	}
}
