// Package dirbrowse provides read-only directory listing confined to a root.
package dirbrowse

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// ErrOutside is returned when a path resolves outside the root.
var ErrOutside = errors.New("path outside root")

// ErrInvalid is returned for malformed paths and symlink loops.
var ErrInvalid = errors.New("invalid path")

// Entry is one directory entry.
type Entry struct {
	Name    string    `json:"name"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Resolve maps rel ("/" separators) to a real path under root. Symlinks are
// followed and the result must be root itself or inside it, else ErrOutside.
// A missing target yields an error wrapping os.ErrNotExist.
func Resolve(root, rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", ErrInvalid
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	c := path.Clean(rel)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", ErrOutside
	}
	c = strings.TrimLeft(c, "/")
	if c == "." {
		c = ""
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(realRoot, filepath.FromSlash(c))
	p, err := filepath.EvalSymlinks(full)
	if err != nil {
		// Whatever the failure, a symlink on the way that escapes the root must
		// win, so the error never reveals what exists outside.
		for anc := filepath.Dir(full); ; anc = filepath.Dir(anc) {
			if ap, aerr := filepath.EvalSymlinks(anc); aerr == nil {
				if !inside(realRoot, ap) {
					return "", ErrOutside
				}
				break
			}
			if anc == filepath.Dir(anc) {
				break
			}
		}
		switch {
		case errors.Is(err, os.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
			return "", fmt.Errorf("%w: %s", os.ErrNotExist, rel)
		case errors.Is(err, syscall.ELOOP), errors.Is(err, syscall.EINVAL),
			strings.Contains(err.Error(), "too many links"): // Go's own loop error has no errno
			return "", ErrInvalid
		}
		return "", err
	}
	if !inside(realRoot, p) {
		return "", ErrOutside
	}
	return p, nil
}

func inside(root, p string) bool {
	sep := string(filepath.Separator)
	return p == root || strings.HasPrefix(p, strings.TrimRight(root, sep)+sep)
}

// List returns the entries of the directory rel, directories first then by
// name. Entries that resolve outside root (escaping symlinks) are skipped.
func List(root, rel string) ([]Entry, error) {
	dir, err := Resolve(root, rel)
	if err != nil {
		return nil, err
	}
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		p, err := Resolve(root, path.Join(rel, de.Name()))
		if err != nil {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		out = append(out, Entry{Name: de.Name(), IsDir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
