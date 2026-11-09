package store

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s, err := New(filepath.Join(t.TempDir(), "drop"), 24*time.Hour, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return s, &now
}

func TestTextSeqAndList(t *testing.T) {
	s, _ := newTestStore(t)
	a, _ := s.AddText(From{"p1", "Mac"}, "one")
	b, _ := s.AddText(From{"p1", "Mac"}, "two")
	if a.Seq != 1 || b.Seq != 2 || a.ExpiresAt.Sub(a.CreatedAt) != 24*time.Hour {
		t.Fatal(a, b)
	}
	if len(a.ID) != 26 {
		t.Fatal(a.ID)
	}
	l := s.List(0)
	if len(l) != 2 || l[0].ID != b.ID {
		t.Fatal(l)
	}
	if l := s.List(1); len(l) != 1 || l[0].ID != b.ID {
		t.Fatal(l)
	}
}

func TestTextLimits(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.AddText(From{}, "  \n "); !errors.Is(err, ErrEmpty) {
		t.Fatal(err)
	}
	if _, err := s.AddText(From{}, strings.Repeat("x", MaxText+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if _, err := s.AddText(From{}, strings.Repeat("x", MaxText)); err != nil {
		t.Fatal(err)
	}
}

func TestFileRoundTripAndNameSanitize(t *testing.T) {
	s, _ := newTestStore(t)
	it, err := s.AddFile(From{}, `..\..\evil/日志 "v2".log`, "", strings.NewReader("hello"), 10)
	if err != nil || it.File.Name != `日志 "v2".log` || it.File.Size != 5 {
		t.Fatal(it, err)
	}
	if it.File.MIME == "" {
		t.Fatal("mime empty")
	}
	f, _, err := s.Open(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "hello" {
		t.Fatal(b)
	}
	entries, _ := os.ReadDir(s.Dir())
	if len(entries) != 1 || entries[0].Name() != it.ID {
		t.Fatal(entries)
	}
	txt, _ := s.AddText(From{}, "x")
	if _, _, err := s.Open(txt.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, _, err := s.Open("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestSanitizeAndMIME(t *testing.T) {
	s, _ := newTestStore(t)
	for _, n := range []string{"", ".", "/", "a/"} {
		it, err := s.AddFile(From{}, n, "", strings.NewReader("x"), 10)
		if err != nil {
			t.Fatal(err)
		}
		want := "file"
		if n == "a/" {
			want = "a"
		}
		if it.File.Name != want || it.File.MIME != "application/octet-stream" {
			t.Fatalf("%q -> %+v", n, it.File)
		}
	}
	it, _ := s.AddFile(From{}, "p.png", "", strings.NewReader("x"), 10)
	if it.File.MIME != "image/png" {
		t.Fatal(it.File.MIME)
	}
}

func TestFileTooLargeLeavesNothing(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.AddFile(From{}, "a", "", strings.NewReader("123456"), 5); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(s.Dir()); len(entries) != 0 || len(s.List(0)) != 0 {
		t.Fatal(entries)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestFileReaderErrorLeavesNothing(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.AddFile(From{}, "a", "", errReader{}, 5); err == nil {
		t.Fatal("want error")
	}
	if entries, _ := os.ReadDir(s.Dir()); len(entries) != 0 {
		t.Fatal(entries)
	}
}

func TestDeleteClearSweep(t *testing.T) {
	s, now := newTestStore(t)
	f, err := s.AddFile(From{}, "a.txt", "", strings.NewReader("hi"), 10)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := s.AddText(From{}, "hello")
	p := filepath.Join(s.Dir(), f.ID)
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	if !s.Delete(f.ID) || s.Delete(f.ID) {
		t.Fatal("delete result")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("file still exists", err)
	}
	if _, ok := s.Get(f.ID); ok {
		t.Fatal("still present")
	}
	*now = now.Add(25 * time.Hour)
	got := s.Sweep()
	if len(got) != 1 || got[0] != tx.ID || len(s.List(0)) != 0 {
		t.Fatal(got)
	}
	if got := s.Clear(); len(got) != 0 {
		t.Fatal(got)
	}
	a, _ := s.AddText(From{}, "a")
	b, _ := s.AddFile(From{}, "b", "", strings.NewReader("b"), 5)
	got = s.Clear()
	if len(got) != 2 || len(s.List(0)) != 0 {
		t.Fatal(got, a, b)
	}
	if entries, _ := os.ReadDir(s.Dir()); len(entries) != 0 {
		t.Fatal(entries)
	}
}

func TestCloseRemovesDirUnlessKeep(t *testing.T) {
	s, _ := newTestStore(t)
	if err := s.Close(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Fatal("dir exists", err)
	}
	s2, _ := newTestStore(t)
	if err := s2.Close(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s2.Dir()); err != nil {
		t.Fatal(err)
	}
}
