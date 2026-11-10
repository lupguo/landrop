package instance

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstanceRoundTripPerms(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "instance.json")
	in := Info{PID: 42, Port: 9958, Secret: "s3cret"}
	if err := Write(p, in); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil || got != in {
		t.Fatal(got, err)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(p)
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("file mode %v", st.Mode())
		}
		dst, _ := os.Stat(filepath.Dir(p))
		if dst.Mode().Perm() != 0o700 {
			t.Fatalf("dir mode %v", dst.Mode())
		}
	}
	Remove(p)
	if _, err := Read(p); err == nil {
		t.Fatal("read after remove")
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got, want := DefaultPath(), filepath.Join(home, ".landrop", "instance.json"); got != want {
		t.Fatal(got, want)
	}
}
