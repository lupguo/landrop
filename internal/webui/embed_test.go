package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandlerMissingIndex(t *testing.T) {
	h := handlerFor(fstest.MapFS{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "make web") {
		t.Fatal(rec.Code, rec.Body)
	}
}

func TestHandlerSPAFallback(t *testing.T) {
	h := handlerFor(fstest.MapFS{"index.html": {Data: []byte("<html>app")}, "assets/a.js": {Data: []byte("js")}})
	for path, want := range map[string]string{"/": "<html>app", "/s/abc": "<html>app", "/assets/a.js": "js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || rec.Body.String() != want {
			t.Fatal(path, rec.Code, rec.Body)
		}
	}
}

func TestHandlerAPINotFallback(t *testing.T) {
	h := handlerFor(fstest.MapFS{"index.html": {Data: []byte("<html>app")}})
	for _, p := range []string{"/api/x", "/events"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != 404 {
			t.Fatal(p, rec.Code)
		}
	}
}
