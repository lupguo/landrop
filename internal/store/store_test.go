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

func stageN(t *testing.T, s *Store, owner string, n int) []string {
	t.Helper()
	ids := []string{}
	for i := 0; i < n; i++ {
		a, err := s.Stage(owner, "f.txt", "", strings.NewReader("data"), 100)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	return ids
}

func dirFiles(t *testing.T, s *Store) int {
	t.Helper()
	es, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	return len(es)
}

func TestStageAndAddMessage(t *testing.T) {
	s, _ := newTestStore(t)
	ids := stageN(t, s, "p1", 2)
	it, err := s.AddMessage(From{PeerID: "p1"}, "hi", ids)
	if err != nil {
		t.Fatal(err)
	}
	if it.Type != KindMessage || it.Content != "hi" || len(it.Attachments) != 2 ||
		it.Attachments[0].ID != ids[0] || it.Attachments[1].ID != ids[1] {
		t.Fatal(it)
	}
	if l := s.List(0); len(l) != 1 {
		t.Fatal(l)
	}
}

func TestAddMessageEmptyContentAllowed(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.AddMessage(From{PeerID: "p1"}, "", stageN(t, s, "p1", 2)); err != nil {
		t.Fatal(err)
	}
}

func TestAddMessageErrors(t *testing.T) {
	s, _ := newTestStore(t)
	used := stageN(t, s, "p1", 1)
	if _, err := s.AddMessage(From{PeerID: "p1"}, "", used); err != nil {
		t.Fatal(err)
	}
	valid := stageN(t, s, "p1", 2)
	other := stageN(t, s, "p2", 1)
	many := make([]string, MaxAttachments+1)
	for i := range many {
		many[i] = valid[0]
	}
	cases := []struct {
		name    string
		content string
		ids     []string
		want    error
	}{
		{"none", "x", nil, ErrEmpty},
		{"too many", "x", many, ErrTooMany},
		{"too large", strings.Repeat("a", MaxText+1), valid, ErrTooLarge},
		{"unknown", "x", []string{valid[0], "nope"}, ErrBadAttachment},
		{"other owner", "x", []string{valid[0], other[0]}, ErrBadAttachment},
		{"duplicate", "x", []string{valid[0], valid[0]}, ErrBadAttachment},
		{"used", "x", []string{valid[0], used[0]}, ErrBadAttachment},
	}
	for _, c := range cases {
		if _, err := s.AddMessage(From{PeerID: "p1"}, c.content, c.ids); !errors.Is(err, c.want) {
			t.Fatalf("%s: got %v want %v", c.name, err, c.want)
		}
	}
	if _, err := s.AddMessage(From{PeerID: "p1"}, "ok", valid); err != nil {
		t.Fatal(err)
	}
}

func TestStageSanitizesName(t *testing.T) {
	s, _ := newTestStore(t)
	for in, want := range map[string]string{"../../etc/x.png": "x.png", `a\b.png`: "b.png", "": "file"} {
		a, err := s.Stage("p1", in, "", strings.NewReader("d"), 10)
		if err != nil {
			t.Fatal(err)
		}
		if a.Name != want {
			t.Fatalf("%q: got %q want %q", in, a.Name, want)
		}
		if in == "../../etc/x.png" && a.MIME != "image/png" {
			t.Fatal(a.MIME)
		}
		if in == "" && a.MIME != "application/octet-stream" {
			t.Fatal(a.MIME)
		}
	}
}

func TestStageTooLarge(t *testing.T) {
	s, _ := newTestStore(t)
	if _, err := s.Stage("p1", "a", "", strings.NewReader("abcd"), 3); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	if n := dirFiles(t, s); n != 0 {
		t.Fatal(n)
	}
}

func TestOpenAttachment(t *testing.T) {
	s, _ := newTestStore(t)
	a, _ := s.Stage("p1", "a.txt", "", strings.NewReader("hello"), 100)
	it, err := s.AddMessage(From{PeerID: "p1"}, "", []string{a.ID})
	if err != nil {
		t.Fatal(err)
	}
	f, att, got, err := s.OpenAttachment(it.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if string(b) != "hello" || att.ID != a.ID || got.ID != it.ID {
		t.Fatal(string(b), att, got)
	}
	if _, _, _, err := s.OpenAttachment(it.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	txt, _ := s.AddText(From{}, "t")
	if _, _, _, err := s.OpenAttachment(txt.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestMessageDeleteRemovesFiles(t *testing.T) {
	mk := func(s *Store) Item {
		it, err := s.AddMessage(From{PeerID: "p1"}, "", stageN(t, s, "p1", 2))
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	s, now := newTestStore(t)
	it := mk(s)
	if dirFiles(t, s) != 2 {
		t.Fatal("expected 2 files")
	}
	s.Delete(it.ID)
	if n := dirFiles(t, s); n != 0 {
		t.Fatal("delete", n)
	}
	mk(s)
	s.Clear()
	if n := dirFiles(t, s); n != 0 {
		t.Fatal("clear", n)
	}
	mk(s)
	*now = now.Add(25 * time.Hour)
	s.Sweep()
	if n := dirFiles(t, s); n != 0 {
		t.Fatal("sweep", n)
	}
}

func TestSweepDropsStaleStaging(t *testing.T) {
	s, now := newTestStore(t)
	stageN(t, s, "p1", 1)
	*now = now.Add(29 * time.Minute)
	if ids := s.Sweep(); len(ids) != 0 || dirFiles(t, s) != 1 {
		t.Fatal(ids, dirFiles(t, s))
	}
	*now = now.Add(2 * time.Minute)
	if ids := s.Sweep(); len(ids) != 0 || dirFiles(t, s) != 0 {
		t.Fatal(ids, dirFiles(t, s))
	}
}

// gatedReader signals on its first Read, then blocks until release is closed.
type gatedReader struct {
	started chan struct{}
	release chan struct{}
	once    bool
	data    io.Reader
}

func (g *gatedReader) Read(p []byte) (int, error) {
	if !g.once {
		g.once = true
		close(g.started)
		<-g.release
	}
	return g.data.Read(p)
}

func TestSweepKeepsStagingWhileOwnerUploads(t *testing.T) {
	s, now := newTestStore(t)
	a := stageN(t, s, "p1", 1)[0]
	other := stageN(t, s, "p2", 1)[0]
	g := &gatedReader{started: make(chan struct{}), release: make(chan struct{}), data: strings.NewReader("big")}
	type res struct {
		att Attachment
		err error
	}
	done := make(chan res)
	go func() {
		att, err := s.Stage("p1", "big.bin", "", g, 100)
		done <- res{att, err}
	}()
	<-g.started
	*now = now.Add(45 * time.Minute)
	s.Sweep()
	if _, err := os.Stat(filepath.Join(s.Dir(), a)); err != nil {
		t.Fatal("A swept while owner upload in flight:", err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), other)); !os.IsNotExist(err) {
		t.Fatal("other owner's stale staging should be swept", err)
	}
	close(g.release)
	r := <-done
	if r.err != nil {
		t.Fatal(r.err)
	}
	// Completion refreshed A: still alive just under StageTTL later.
	*now = now.Add(29 * time.Minute)
	s.Sweep()
	it, err := s.AddMessage(From{PeerID: "p1"}, "", []string{a, r.att.ID})
	if err != nil || len(it.Attachments) != 2 {
		t.Fatal(it, err)
	}

	// Nothing in flight: staging expires after StageTTL as before.
	ids := stageN(t, s, "p1", 2)
	*now = now.Add(StageTTL)
	s.Sweep()
	if _, err := s.AddMessage(From{PeerID: "p1"}, "", ids); !errors.Is(err, ErrBadAttachment) {
		t.Fatal("expected expiry", err)
	}
}
