package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/lupguo/landrop/internal/diskfree"
	"github.com/lupguo/landrop/internal/store"
)

const diskReserve = 64 << 20

var inlineMIMEs = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// handleUpload: POST /api/upload, multipart with a single part named "file",
// streamed straight to disk.
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 0 {
		free, err := diskfree.Free(s.Store.Dir())
		if err == nil && free < uint64(r.ContentLength)+diskReserve {
			writeErr(w, http.StatusInsufficientStorage, "disk_full")
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxSize+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no_file")
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			if tooLarge(err) {
				writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
			} else {
				writeErr(w, http.StatusBadRequest, "no_file")
			}
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		from := store.From{PeerID: sessionFrom(r).PeerID, Name: s.deviceName(r, r.Header.Get(nameHeader))}
		it, err := s.Store.AddFile(from, part.FileName(), part.Header.Get("Content-Type"), part, s.MaxSize)
		switch {
		case tooLarge(err):
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		case err != nil:
			// Aborted or broken body: AddFile already removed its .part file.
			writeErr(w, http.StatusBadRequest, "upload_failed")
			return
		}
		s.Hub.Publish("item.created", it)
		if s.OnFileStored != nil {
			go s.OnFileStored(it)
		}
		writeJSON(w, http.StatusCreated, it)
		return
	}
}

func tooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.Is(err, store.ErrTooLarge) || errors.As(err, &mbe)
}

// handleRaw: GET /api/items/{id}/raw[?inline=1].
func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	it, ok := s.Store.Get(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	h := w.Header()
	if it.Type == store.KindText {
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Disposition", contentDisposition("attachment", id+".txt"))
		h.Set("Cache-Control", "no-store")
		io.WriteString(w, it.Content)
		return
	}
	f, it, err := s.Store.Open(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	defer f.Close()
	h.Set("Content-Type", it.File.MIME)
	kind := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		base, _, _ := mime.ParseMediaType(it.File.MIME)
		if inlineMIMEs[strings.ToLower(base)] {
			kind = "inline"
		}
	}
	h.Set("Content-Disposition", contentDisposition(kind, it.File.Name))
	h.Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", it.CreatedAt, f)
}

// contentDisposition builds a Content-Disposition value. kind is "inline" or
// "attachment"; attachments carry an ASCII fallback plus RFC 5987 filename*.
func contentDisposition(kind, name string) string {
	if kind == "inline" {
		return "inline"
	}
	var b strings.Builder
	for _, r := range name {
		if r > 0x7e || r < 0x20 || r == '"' || r == '\\' {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	return fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", b.String(), url.PathEscape(name))
}
