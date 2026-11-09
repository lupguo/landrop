// Package config parses LanDrop command-line flags and environment.
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lupguo/landrop/internal/i18n"
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
	fs := newFlagSet(&c, &maxSize, getenv)
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

// newFlagSet defines LanDrop's flags on c and maxSize. Each flag's usage
// string is an i18n key (rendered by Usage). The --lang default honours
// LANDROP_LANG from getenv.
func newFlagSet(c *Config, maxSize *string, getenv func(string) string) *flag.FlagSet {
	fs := flag.NewFlagSet("landrop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	defLang := "en"
	if v := getenv("LANDROP_LANG"); v != "" {
		defLang = v
	}
	fs.IntVar(&c.Port, "port", 8848, i18n.KeyFlagPort)
	fs.StringVar(&c.Name, "name", "drop", i18n.KeyFlagName)
	fs.StringVar(&c.Dir, "dir", "", i18n.KeyFlagDir)
	fs.BoolVar(&c.NoAuth, "no-auth", false, i18n.KeyFlagNoAuth)
	fs.DurationVar(&c.TTL, "ttl", 24*time.Hour, i18n.KeyFlagTTL)
	fs.BoolVar(&c.Keep, "keep", false, i18n.KeyFlagKeep)
	fs.StringVar(maxSize, "max-size", "4GB", i18n.KeyFlagMaxSize)
	fs.BoolVar(&c.NoSave, "no-save", false, i18n.KeyFlagNoSave)
	fs.BoolVar(&c.NoOpen, "no-open", false, i18n.KeyFlagNoOpen)
	fs.StringVar(&c.Lang, "lang", defLang, i18n.KeyFlagLang)
	fs.BoolVar(&c.ShowVersion, "version", false, i18n.KeyFlagVersion)
	return fs
}

// Usage writes the localized help text, generated from the real flag set,
// to w. Flags are listed alphabetically (flag.VisitAll) with their defaults.
func Usage(w io.Writer, lang string) {
	var c Config
	var maxSize string
	fs := newFlagSet(&c, &maxSize, func(string) string { return "" })
	type row struct{ left, desc string }
	var rows []row
	width := len("-h, --help")
	fs.VisitAll(func(f *flag.Flag) {
		typ, _ := flag.UnquoteUsage(f)
		left := "--" + f.Name
		if typ != "" {
			left += " " + typ
		}
		desc := i18n.T(lang, f.Usage)
		if def := f.DefValue; def != "" && def != "false" {
			if typ == "string" {
				def = strconv.Quote(def)
			}
			desc += " " + i18n.T(lang, i18n.KeyUsageDefault, def)
		}
		if len(left) > width {
			width = len(left)
		}
		rows = append(rows, row{left, desc})
	})
	rows = append(rows, row{"-h, --help", i18n.T(lang, i18n.KeyUsageHelp)})
	var sb strings.Builder
	sb.WriteString(i18n.T(lang, i18n.KeyUsageHeader) + "\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "  %-*s  %s\n", width, r.left, r.desc)
	}
	io.WriteString(w, sb.String())
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

// LangFromArgs pre-scans args for --lang/-lang (as "--lang zh" or
// "--lang=zh") so help and parse errors can be localized before (or even
// when) Parse fails. Unsupported or missing values yield fallback.
func LangFromArgs(args []string, fallback string) string {
	lang := fallback
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		var v string
		switch {
		case a == "--lang" || a == "-lang":
			if i+1 >= len(args) {
				continue
			}
			i++
			v = args[i]
		case strings.HasPrefix(a, "--lang="):
			v = strings.TrimPrefix(a, "--lang=")
		case strings.HasPrefix(a, "-lang="):
			v = strings.TrimPrefix(a, "-lang=")
		default:
			continue
		}
		if slices.Contains(i18n.Langs, v) {
			lang = v
		} else {
			lang = fallback
		}
	}
	return lang
}
