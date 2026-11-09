//go:build !windows

package server

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestDirRawRejectsFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "fifo"), 0600); err != nil {
		t.Skip(err)
	}
	ts, _ := newTestServer(t, func(d *Deps) { d.Dir = root })
	c := login(t, ts)
	if resp, _ := do(t, c, ts, "GET", "/api/dir/raw?path=fifo", ""); resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
}
