package qr

import (
	"strings"
	"testing"
)

func TestQRASCIIandSVG(t *testing.T) {
	a, err := ASCII("http://192.168.1.23:9958/?t=abc")
	if err != nil || a == "" || !strings.Contains(a, "█") {
		t.Fatalf("ascii: err=%v out=%q", err, a)
	}
	s, err := SVG("http://192.168.1.23:9958/?t=abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(s), "<svg") || !strings.Contains(string(s), "viewBox") ||
		!strings.Contains(string(s), "crispEdges") {
		t.Fatalf("svg: %.200s", s)
	}
}
