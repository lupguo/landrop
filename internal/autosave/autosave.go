// Package autosave copies received files into the user's download folder.
package autosave

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UniquePath returns dir/name, or dir/"stem (n)ext" for the first n ≥ 1 that
// does not exist yet.
func UniquePath(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Lstat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p
		}
	}
}

// safeName reduces name to a base name that is valid on every platform
// (Windows being the strictest): <>:"/\|?* and control characters become
// "_", trailing dots and spaces are dropped, and reserved device names (CON,
// PRN, AUX, NUL, COM1-9, LPT1-9, with or without an extension) get a "_"
// prefix. An empty result is "file".
func safeName(name string) string {
	n := name
	if i := strings.LastIndexAny(n, `/\`); i >= 0 {
		n = n[i+1:]
	}
	n = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, n)
	n = strings.TrimRight(n, ". ")
	if n == "" {
		return "file"
	}
	stem, _, _ := strings.Cut(n, ".")
	if reserved[strings.ToUpper(strings.TrimRight(stem, " "))] {
		n = "_" + n
	}
	return n
}

var reserved = func() map[string]bool {
	m := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true}
	for i := '1'; i <= '9'; i++ {
		m["COM"+string(i)] = true
		m["LPT"+string(i)] = true
	}
	return m
}()

// Save copies srcPath into dir under a unique variant of name and returns the
// destination path. A failed copy leaves no partial file behind.
func Save(srcPath, name, dir string) (string, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name = safeName(name)
	var dst *os.File
	var p string
	// O_EXCL guards against a racing writer taking the same name.
	for i := 0; i < 100; i++ {
		p = UniquePath(dir, name)
		dst, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if !os.IsExist(err) {
			break
		}
	}
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(p)
		return "", err
	}
	if err := dst.Close(); err != nil {
		os.Remove(p)
		return "", err
	}
	return p, nil
}
