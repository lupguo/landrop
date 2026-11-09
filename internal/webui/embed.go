// Package webui serves the embedded frontend.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

func distFS() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// Handler serves embedded files with SPA fallback to index.html.
func Handler() http.Handler { return handlerFor(distFS()) }

// Index returns the embedded index.html bytes, if built.
func Index() ([]byte, bool) { return indexFrom(distFS()) }

func indexFrom(fsys fs.FS) ([]byte, bool) {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, false
	}
	return b, true
}

func handlerFor(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	serveIndex := func(w http.ResponseWriter) {
		b, ok := indexFrom(fsys)
		if !ok {
			http.Error(w, `frontend not built: run "make web"`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") || p == "/events" || strings.HasPrefix(p, "/events/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+p), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(fsys, name); err == nil && !st.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		serveIndex(w)
	})
}
