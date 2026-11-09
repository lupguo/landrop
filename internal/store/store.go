package store

import (
	"errors"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store is a concurrency-safe item index. A file item's bytes live at filepath.Join(Dir(), item.ID).
type Store struct {
	dir string
	ttl time.Duration
	now func() time.Time

	mu    sync.RWMutex
	items map[string]*Item
	order []*Item // ascending by Seq
	seq   uint64
}

func New(dir string, ttl time.Duration, now func() time.Time) (*Store, error) {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, ttl: ttl, now: now, items: map[string]*Item{}}, nil
}

func (s *Store) Dir() string { return s.dir }

// insert must be called without the lock held; it assigns Seq and timestamps.
func (s *Store) insert(it Item) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	it.Seq = s.seq
	it.CreatedAt = s.now()
	it.ExpiresAt = it.CreatedAt.Add(s.ttl)
	p := &it
	s.items[it.ID] = p
	s.order = append(s.order, p)
	return it
}

func (s *Store) AddText(from From, content string) (Item, error) {
	if strings.TrimSpace(content) == "" {
		return Item{}, ErrEmpty
	}
	if len(content) > MaxText {
		return Item{}, ErrTooLarge
	}
	id, err := newID(s.now())
	if err != nil {
		return Item{}, err
	}
	return s.insert(Item{ID: id, Type: KindText, Content: content, From: from}), nil
}

func sanitizeName(name string) string {
	n := filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if n == "" || n == "." || n == "/" {
		return "file"
	}
	return n
}

func (s *Store) AddFile(from From, name, mimeType string, r io.Reader, max int64) (Item, error) {
	id, err := newID(s.now())
	if err != nil {
		return Item{}, err
	}
	name = sanitizeName(name)
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}
	final := filepath.Join(s.dir, id)
	part := final + ".part"
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return Item{}, err
	}
	n, err := io.Copy(f, io.LimitReader(r, max+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > max {
		err = ErrTooLarge
	}
	if err == nil {
		err = os.Rename(part, final)
	}
	if err != nil {
		os.Remove(part)
		return Item{}, err
	}
	return s.insert(Item{ID: id, Type: KindFile, File: &FileMeta{Name: name, Size: n, MIME: mimeType}, From: from}), nil
}

func (s *Store) Get(id string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok {
		return Item{}, false
	}
	return *it, true
}

func (s *Store) Open(id string) (*os.File, Item, error) {
	it, ok := s.Get(id)
	if !ok || it.Type != KindFile {
		return nil, Item{}, ErrNotFound
	}
	f, err := os.Open(filepath.Join(s.dir, id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, Item{}, ErrNotFound
		}
		return nil, Item{}, err
	}
	return f, it, nil
}

// List returns items with Seq > since, newest first.
func (s *Store) List(since uint64) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Item{}
	for i := len(s.order) - 1; i >= 0; i-- {
		if s.order[i].Seq <= since {
			break
		}
		out = append(out, *s.order[i])
	}
	return out
}

func (s *Store) removeFile(it *Item) {
	if it.Type == KindFile {
		os.Remove(filepath.Join(s.dir, it.ID))
	}
}

func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return false
	}
	delete(s.items, id)
	for i, o := range s.order {
		if o == it {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	s.removeFile(it)
	return true
}

// removeWhere removes matching items; caller must not hold the lock.
func (s *Store) removeWhere(pred func(*Item) bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	keep := s.order[:0]
	for _, it := range s.order {
		if pred(it) {
			ids = append(ids, it.ID)
			delete(s.items, it.ID)
			s.removeFile(it)
		} else {
			keep = append(keep, it)
		}
	}
	for i := len(keep); i < len(s.order); i++ {
		s.order[i] = nil
	}
	s.order = keep
	return ids
}

func (s *Store) Clear() []string { return s.removeWhere(func(*Item) bool { return true }) }

func (s *Store) Sweep() []string {
	now := s.now()
	return s.removeWhere(func(it *Item) bool { return !now.Before(it.ExpiresAt) })
}

func (s *Store) Close(keep bool) error {
	if keep {
		return nil
	}
	return os.RemoveAll(s.dir)
}
