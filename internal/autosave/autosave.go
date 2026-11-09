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

func safeName(name string) string {
	n := filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if n == "" || n == "." || n == ".." || n == "/" {
		return "file"
	}
	return n
}

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
