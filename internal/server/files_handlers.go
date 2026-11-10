package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lupguo/landrop/internal/diskfree"
	"github.com/lupguo/landrop/internal/store"
)

// freeSpace is swappable in tests. A Free error deliberately fails open.
var freeSpace = diskfree.Free

const diskReserve = 64 << 20

var inlineMIMEs = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// filePart is the shared prologue of the upload handlers: it enforces the
// Content-Length limit and disk-free check, caps the body, and returns the
// multipart part named "file". On failure it has already written the error.
func (s *server) filePart(w http.ResponseWriter, r *http.Request) (*multipart.Part, bool) {
	limit := s.MaxSize + 1<<20 // file plus multipart overhead
	if r.ContentLength > limit {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return nil, false
	}
	if r.ContentLength > 0 {
		free, err := freeSpace(s.Store.Dir())
		if err == nil && free < uint64(r.ContentLength)+diskReserve {
			writeErr(w, http.StatusInsufficientStorage, "disk_full")
			return nil, false
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no_file")
		return nil, false
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			if tooLarge(err) {
				writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
			} else {
				writeErr(w, http.StatusBadRequest, "no_file")
			}
			return nil, false
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		return part, true
	}
}

// handleUpload: POST /api/upload, multipart with a single part named "file",
// streamed straight to disk.
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	part, ok := s.filePart(w, r)
	if !ok {
		return
	}
	from := store.From{PeerID: sessionFrom(r).PeerID, Name: s.deviceName(r, r.Header.Get(nameHeader))}
	it, err := s.Store.AddFile(from, part.FileName(), part.Header.Get("Content-Type"), part, s.MaxSize)
	switch {
	case tooLarge(err):
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	case isDiskFull(err):
		writeErr(w, http.StatusInsufficientStorage, "disk_full")
		return
	case err != nil:
		// Aborted or broken body: AddFile already removed its .part file.
		writeErr(w, http.StatusBadRequest, "upload_failed")
		return
	}
	s.Hub.Publish("item.created", it)
	if s.OnFileStored != nil {
		s.OnFileStored(it)
	}
	writeJSON(w, http.StatusCreated, it)
}

func tooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.Is(err, store.ErrTooLarge) || errors.As(err, &mbe)
}

// handleRaw: GET /api/items/{id}/raw[?inline=1].
func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	it, ok := s.Store.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	s.serveItem(w, r, it, false)
}

// serveItem writes an item's bytes. public (share links) forces attachment
// for files and serves text inline as text/plain so the landing page can
// fetch it; otherwise text is an attachment and files honour ?inline=1 for
// safe image types.
func (s *server) serveItem(w http.ResponseWriter, r *http.Request, it store.Item, public bool) {
	h := w.Header()
	if it.Type == store.KindText {
		h.Set("Content-Type", "text/plain; charset=utf-8")
		if public {
			h.Set("Content-Disposition", "inline")
		} else {
			h.Set("Content-Disposition", contentDisposition("attachment", it.ID+".txt"))
		}
		h.Set("Cache-Control", "no-store")
		if public {
			h.Set("Content-Length", strconv.Itoa(len(it.Content)))
			if r.Method == http.MethodHead {
				return
			}
		}
		io.WriteString(w, it.Content)
		return
	}
	f, it, err := s.Store.Open(it.ID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	defer f.Close()
	s.serveFile(w, r, f, it.File.Name, it.File.MIME, it.CreatedAt, public, !public && r.URL.Query().Get("inline") == "1")
}

// serveFile writes a stored file. Only the inlineMIMEs whitelist is ever
// served inline, and only when the caller passes inline (share file links
// never do). Public (share) responses get no Range support: always a full 200.
func (s *server) serveFile(w http.ResponseWriter, r *http.Request, f *os.File, name, mimeType string, mod time.Time, public, inline bool) {
	h := w.Header()
	ctype := mimeType
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h.Set("Content-Type", ctype)
	kind := "attachment"
	if inline {
		base, _, _ := mime.ParseMediaType(mimeType)
		if inlineMIMEs[strings.ToLower(base)] {
			kind = "inline"
		}
	}
	h.Set("Content-Disposition", contentDisposition(kind, name))
	h.Set("Cache-Control", "no-store")
	if public {
		r.Header.Del("Range")
		r.Header.Del("If-Range")
		if fi, err := f.Stat(); err == nil {
			h.Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		}
		if r.Method != http.MethodHead {
			io.Copy(w, f)
		}
		return
	}
	http.ServeContent(w, r, "", mod, f)
}

// contentDisposition builds a Content-Disposition value. kind is "inline" or
// "attachment"; attachments carry an ASCII fallback plus RFC 5987 filename*.
func contentDisposition(kind, name string) string {
	if kind == "inline" {
		return "inline"
	}
	var b strings.Builder
	for _, r := range name {
		if r > 0x7e || r < 0x20 || r == '"' || r == '\\' || r == '%' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", b.String(), pctEncode(name))
}

// pctEncode percent-encodes every byte except RFC 5987 attr-chars.
func pctEncode(s string) string {
	const hexd = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("!#$&+.^_`|~-", c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexd[c>>4])
			b.WriteByte(hexd[c&15])
		}
	}
	return b.String()
}
