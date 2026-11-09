// Package qr renders QR codes as terminal half-block text and as SVG.
package qr

import (
	"bytes"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func bitmap(text string) ([][]bool, error) {
	q, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	q.DisableBorder = true
	return q.Bitmap(), nil
}

// ASCII renders text as a QR code using half-block characters "▀▄█ " (two
// module rows per text line) with a quiet zone of 2 modules.
func ASCII(text string) (string, error) {
	bm, err := bitmap(text)
	if err != nil {
		return "", err
	}
	const quiet = 2
	n := len(bm) + 2*quiet
	at := func(x, y int) bool {
		x, y = x-quiet, y-quiet
		return y >= 0 && y < len(bm) && x >= 0 && x < len(bm) && bm[y][x]
	}
	var b strings.Builder
	for y := 0; y < n; y += 2 {
		for x := 0; x < n; x++ {
			top, bot := at(x, y), at(x, y+1)
			switch {
			case top && bot:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bot:
				b.WriteString("▄")
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// SVG renders text as a QR code: black modules on white, quiet zone 4.
func SVG(text string) ([]byte, error) {
	bm, err := bitmap(text)
	if err != nil {
		return nil, err
	}
	const quiet = 4
	n := len(bm) + 2*quiet
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y, row := range bm {
		for x, on := range row {
			if on {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+quiet, y+quiet)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.Bytes(), nil
}
