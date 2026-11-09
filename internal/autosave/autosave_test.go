package autosave

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniquePath(t *testing.T) {
	d := t.TempDir()
	if got := UniquePath(d, "a.log"); filepath.Base(got) != "a.log" {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(d, "a.log"), nil, 0600)
	if got := UniquePath(d, "a.log"); filepath.Base(got) != "a (1).log" {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(d, "a (1).log"), nil, 0600)
	if got := UniquePath(d, "a.log"); filepath.Base(got) != "a (2).log" {
		t.Fatal(got)
	}
	os.WriteFile(filepath.Join(d, "README"), nil, 0600)
	if got := UniquePath(d, "README"); filepath.Base(got) != "README (1)" {
		t.Fatal(got)
	}
}

func TestSave(t *testing.T) {
	src := filepath.Join(t.TempDir(), "blob")
	os.WriteFile(src, []byte("payload"), 0600)
	dir := filepath.Join(t.TempDir(), "Downloads", "LanDrop")
	p1, err := Save(src, "x.log", dir)
	if err != nil || filepath.Base(p1) != "x.log" {
		t.Fatal(p1, err)
	}
	p2, err := Save(src, "../x.log", dir)
	if err != nil || filepath.Base(p2) != "x (1).log" || filepath.Dir(p2) != dir {
		t.Fatal(p2, err)
	}
	b, _ := os.ReadFile(p2)
	if string(b) != "payload" {
		t.Fatalf("%q", b)
	}
	if _, err := Save(filepath.Join(t.TempDir(), "missing"), "y", dir); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(filepath.Join(dir, "y")); !os.IsNotExist(err) {
		t.Fatal("partial dest left behind")
	}
}
