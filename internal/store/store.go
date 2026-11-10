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
	stage map[string]staged
}

type staged struct {
	att   Attachment
	owner string
	at    time.Time
}

func New(dir string, ttl time.Duration, now func() time.Time) (*Store, error) {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, ttl: ttl, now: now, items: map[string]*Item{}, stage: map[string]staged{}}, nil
}

func (s *Store) Dir() string { return s.dir }

// insert must be called without the lock held; it assigns Seq and timestamps.
func (s *Store) insert(it Item) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.insertLocked(it)
}

// insertLocked must be called with the write lock held.
func (s *Store) insertLocked(it Item) Item {
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

// writeBlob streams r to Dir()/id via a .part file, enforcing max bytes.
func (s *Store) writeBlob(id string, r io.Reader, max int64) (int64, error) {
	final := filepath.Join(s.dir, id)
	part := final + ".part"
	f, err := os.OpenFile(part, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
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
		return 0, err
	}
	return n, nil
}

func fileMIME(name, mimeType string) string {
	if mimeType != "" {
		return mimeType
	}
	if m := mime.TypeByExtension(filepath.Ext(name)); m != "" {
		return m
	}
	return "application/octet-stream"
}

func (s *Store) AddFile(from From, name, mimeType string, r io.Reader, max int64) (Item, error) {
	id, err := newID(s.now())
	if err != nil {
		return Item{}, err
	}
	name = sanitizeName(name)
	mimeType = fileMIME(name, mimeType)
	n, err := s.writeBlob(id, r, max)
	if err != nil {
		return Item{}, err
	}
	return s.insert(Item{ID: id, Type: KindFile, File: &FileMeta{Name: name, Size: n, MIME: mimeType}, From: from}), nil
}

// Stage stores an upload on disk, unattached, until AddMessage claims it or it goes stale.
func (s *Store) Stage(owner, name, mimeType string, r io.Reader, max int64) (Attachment, error) {
	id, err := newID(s.now())
	if err != nil {
		return Attachment{}, err
	}
	name = sanitizeName(name)
	n, err := s.writeBlob(id, r, max)
	if err != nil {
		return Attachment{}, err
	}
	att := Attachment{ID: id, Name: name, Size: n, MIME: fileMIME(name, mimeType)}
	s.mu.Lock()
	s.stage[id] = staged{att: att, owner: owner, at: s.now()}
	s.mu.Unlock()
	return att, nil
}

// AddMessage creates a message item from content and the owner's staged attachments.
func (s *Store) AddMessage(from From, content string, ids []string) (Item, error) {
	if len(ids) == 0 {
		return Item{}, ErrEmpty
	}
	if len(ids) > MaxAttachments {
		return Item{}, ErrTooMany
	}
	if len(content) > MaxText {
		return Item{}, ErrTooLarge
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	atts := make([]Attachment, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		st, ok := s.stage[id]
		if !ok || st.owner != from.PeerID || seen[id] {
			return Item{}, ErrBadAttachment
		}
		seen[id] = true
		atts = append(atts, st.att)
	}
	id, err := newID(s.now())
	if err != nil {
		return Item{}, err
	}
	for _, id := range ids {
		delete(s.stage, id)
	}
	return s.insertLocked(Item{ID: id, Type: KindMessage, Content: content, Attachments: atts, From: from}), nil
}

// OpenAttachment opens one attachment of a message item.
func (s *Store) OpenAttachment(itemID, attID string) (*os.File, Attachment, Item, error) {
	it, ok := s.Get(itemID)
	if !ok || it.Type != KindMessage {
		return nil, Attachment{}, Item{}, ErrNotFound
	}
	for _, a := range it.Attachments {
		if a.ID != attID {
			continue
		}
		f, err := os.Open(filepath.Join(s.dir, a.ID))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, Attachment{}, Item{}, ErrNotFound
			}
			return nil, Attachment{}, Item{}, err
		}
		return f, a, it, nil
	}
	return nil, Attachment{}, Item{}, ErrNotFound
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
	for _, a := range it.Attachments {
		os.Remove(filepath.Join(s.dir, a.ID))
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
	s.mu.Lock()
	for id, st := range s.stage {
		if now.Sub(st.at) >= StageTTL {
			delete(s.stage, id)
			os.Remove(filepath.Join(s.dir, id))
		}
	}
	s.mu.Unlock()
	return s.removeWhere(func(it *Item) bool { return !now.Before(it.ExpiresAt) })
}

func (s *Store) Close(keep bool) error {
	if keep {
		return nil
	}
	return os.RemoveAll(s.dir)
}
