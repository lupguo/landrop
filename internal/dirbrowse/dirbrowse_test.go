package dirbrowse

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkRoot(t *testing.T) (root, outside string) {
	t.Helper()
	root, _ = filepath.EvalSymlinks(t.TempDir())
	outside, _ = filepath.EvalSymlinks(t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	return root, outside
}

func TestResolveRejectsTraversal(t *testing.T) {
	root, _ := mkRoot(t)
	for _, rel := range []string{"../", "../../etc/passwd", "a/../../x", "link", "link/x"} {
		if p, err := Resolve(root, rel); !errors.Is(err, ErrOutside) {
			t.Fatal(rel, p, err)
		}
	}
	// A leading slash is treated as relative to root: never leaves root.
	if p, err := Resolve(root, "/etc/passwd"); err == nil || (!errors.Is(err, ErrOutside) && !errors.Is(err, os.ErrNotExist)) {
		t.Fatal(p, err)
	}
	if p, err := Resolve(root, "a.txt"); err != nil || !strings.HasPrefix(p, root) {
		t.Fatal(p, err)
	}
	if p, err := Resolve(root, ""); err != nil || p != root {
		t.Fatal(p, err)
	}
}

func TestResolveSymlinkedRoot(t *testing.T) {
	root, _ := mkRoot(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skip(err)
	}
	if _, err := Resolve(alias, "a.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestListHidesEscapingSymlink(t *testing.T) {
	root, _ := mkRoot(t)
	os.Mkdir(filepath.Join(root, "sub"), 0700)
	es, err := List(root, "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name)
	}
	if strings.Join(names, ",") != "sub,a.txt" {
		t.Fatal(names)
	}
	if !es[0].IsDir || es[1].Size != 1 {
		t.Fatal(es)
	}
}

func TestResolveNoExistenceOracle(t *testing.T) {
	root, outside := mkRoot(t)
	os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0600)
	os.WriteFile(filepath.Join(outside, "realfile"), []byte("s"), 0600)
	for _, rel := range []string{"link/secret/x", "link/nope/x", "link/realfile", "link/secret", "..\\x", "a\x00b/../../.."} {
		if p, err := Resolve(root, rel); !errors.Is(err, ErrOutside) && !errors.Is(err, ErrInvalid) {
			t.Fatal(rel, p, err)
		}
	}
	for _, rel := range []string{"link/secret/x", "link/nope/x", "link/realfile", "..\\x"} {
		if _, err := Resolve(root, rel); !errors.Is(err, ErrOutside) {
			t.Fatal(rel, err)
		}
	}
	// ENOTDIR inside the root is plain not-found.
	if _, err := Resolve(root, "a.txt/x"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestResolveSiblingPrefix(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	root := filepath.Join(base, "r")
	sib := filepath.Join(base, "rX")
	os.Mkdir(root, 0700)
	os.Mkdir(sib, 0700)
	if err := os.Symlink(sib, filepath.Join(root, "s")); err != nil {
		t.Skip(err)
	}
	if _, err := Resolve(root, "s"); !errors.Is(err, ErrOutside) {
		t.Fatal(err)
	}
}

func TestResolveLoop(t *testing.T) {
	root, _ := mkRoot(t)
	if err := os.Symlink(filepath.Join(root, "l2"), filepath.Join(root, "l1")); err != nil {
		t.Skip(err)
	}
	os.Symlink(filepath.Join(root, "l1"), filepath.Join(root, "l2"))
	if _, err := Resolve(root, "l1/x"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestListRegularFileIsNotDir(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0600)
	if _, err := List(root, "f"); !errors.Is(err, ErrNotDir) {
		t.Fatalf("err %v", err)
	}
}
