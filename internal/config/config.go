// Package config parses LanDrop command-line flags and environment.
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        int
	Name        string
	Dir         string
	NoAuth      bool
	TTL         time.Duration
	Keep        bool
	MaxSize     int64
	NoSave      bool
	NoOpen      bool
	Lang        string
	ShowVersion bool
}

var allowedLangs = []string{"en", "zh", "ja", "ko"}

// Parse parses args (without program name). Flag beats env beats default.
func Parse(args []string, getenv func(string) string) (Config, error) {
	var c Config
	var maxSize string
	fs := flag.NewFlagSet("landrop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	defLang := "en"
	if v := getenv("LANDROP_LANG"); v != "" {
		defLang = v
	}
	fs.IntVar(&c.Port, "port", 8848, "listen port")
	fs.StringVar(&c.Name, "name", "drop", "mDNS name")
	fs.StringVar(&c.Dir, "dir", "", "temp/storage directory")
	fs.BoolVar(&c.NoAuth, "no-auth", false, "disable access code")
	fs.DurationVar(&c.TTL, "ttl", 24*time.Hour, "item time to live")
	fs.BoolVar(&c.Keep, "keep", false, "keep temp files on exit")
	fs.StringVar(&maxSize, "max-size", "4GB", "max file size")
	fs.BoolVar(&c.NoSave, "no-save", false, "disable autosave")
	fs.BoolVar(&c.NoOpen, "no-open", false, "do not open browser")
	fs.StringVar(&c.Lang, "lang", defLang, "language (en|zh|ja|ko)")
	fs.BoolVar(&c.ShowVersion, "version", false, "print version")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if c.Port < 1 || c.Port > 65535 {
		return Config{}, fmt.Errorf("invalid --port %d", c.Port)
	}
	if c.TTL <= 0 {
		return Config{}, fmt.Errorf("invalid --ttl %s", c.TTL)
	}
	ok := false
	for _, l := range allowedLangs {
		if c.Lang == l {
			ok = true
		}
	}
	if !ok {
		return Config{}, fmt.Errorf("invalid --lang %q (want en|zh|ja|ko)", c.Lang)
	}
	n, err := ParseSize(maxSize)
	if err != nil {
		return Config{}, fmt.Errorf("invalid --max-size: %w", err)
	}
	c.MaxSize = n
	if c.Dir != "" {
		abs, err := filepath.Abs(c.Dir)
		if err != nil {
			return Config{}, err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return Config{}, fmt.Errorf("invalid --dir: %w", err)
		}
		if !st.IsDir() {
			return Config{}, fmt.Errorf("invalid --dir: %s is not a directory", abs)
		}
		c.Dir = abs
	}
	return c, nil
}

// ParseSize parses sizes like "4GB", "512mb", "1KiB", "123" (bytes).
func ParseSize(s string) (int64, error) {
	u := strings.ToUpper(strings.TrimSpace(s))
	mult := int64(1)
	for _, p := range []struct {
		suf string
		m   int64
	}{{"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}} {
		if strings.HasSuffix(u, p.suf) {
			u = strings.TrimSpace(strings.TrimSuffix(u, p.suf))
			mult = p.m
			break
		}
	}
	n, err := strconv.ParseInt(u, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	if n > (1<<63-1)/mult {
		return 0, fmt.Errorf("size %q too large", s)
	}
	return n * mult, nil
}
