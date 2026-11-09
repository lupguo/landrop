package server

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/lupguo/landrop/internal/dirbrowse"
)

// dirErr maps dirbrowse errors to HTTP statuses.
func dirErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dirbrowse.ErrOutside):
		writeErr(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, dirbrowse.ErrInvalid):
		writeErr(w, http.StatusBadRequest, "bad_request")
	case errors.Is(err, os.ErrNotExist):
		writeErr(w, http.StatusNotFound, "not_found")
	default:
		writeErr(w, http.StatusInternalServerError, "internal")
	}
}

func cleanRel(p string) string {
	c := path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))
	return strings.TrimPrefix(c, "/")
}

func (s *server) handleDirList(w http.ResponseWriter, r *http.Request) {
	if s.Dir == "" {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	rel := r.URL.Query().Get("path")
	entries, err := dirbrowse.List(s.Dir, rel)
	if err != nil {
		dirErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": cleanRel(rel), "entries": entries})
}

func (s *server) handleDirRaw(w http.ResponseWriter, r *http.Request) {
	if s.Dir == "" {
		writeErr(w, http.StatusNotFound, "not_found")
		return
	}
	p, err := dirbrowse.Resolve(s.Dir, r.URL.Query().Get("path"))
	if err != nil {
		dirErr(w, err)
		return
	}
	fi, err := os.Stat(p)
	if err != nil {
		dirErr(w, err)
		return
	}
	if fi.IsDir() {
		writeErr(w, http.StatusBadRequest, "is_directory")
		return
	}
	if !fi.Mode().IsRegular() {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	f, err := os.Open(p)
	if err != nil {
		dirErr(w, err)
		return
	}
	defer f.Close()
	name := path.Base(cleanRel(r.URL.Query().Get("path")))
	w.Header().Set("Content-Disposition", contentDisposition("attachment", name))
	http.ServeContent(w, r, name, fi.ModTime(), f)
}
