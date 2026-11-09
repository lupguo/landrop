package config

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lupguo/landrop/internal/i18n"
)

func TestDefaults(t *testing.T) {
	c, err := Parse(nil, func(string) string { return "" })
	if err != nil || c.Port != 8848 || c.Name != "drop" || c.TTL != 24*time.Hour || c.MaxSize != 4<<30 || c.Lang != "en" || c.NoAuth || c.NoSave {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestLangEnvAndFlag(t *testing.T) {
	env := func(k string) string {
		if k == "LANDROP_LANG" {
			return "ja"
		}
		return ""
	}
	c, _ := Parse(nil, env)
	if c.Lang != "ja" {
		t.Fatal(c.Lang)
	}
	c, _ = Parse([]string{"--lang", "ko"}, env)
	if c.Lang != "ko" {
		t.Fatal(c.Lang)
	}
	if _, err := Parse([]string{"--lang", "fr"}, env); err == nil {
		t.Fatal("want error")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{"4GB": 4 << 30, "512mb": 512 << 20, "1KiB": 1024, "77": 77, "100KB": 100 << 10, "2GiB": 2 << 30} {
		if got, err := ParseSize(in); err != nil || got != want {
			t.Fatal(in, got, err)
		}
	}
	if _, err := ParseSize("-1GB"); err == nil {
		t.Fatal("want error")
	}
}

func TestBadPortAndDir(t *testing.T) {
	if _, err := Parse([]string{"--port", "70000"}, os.Getenv); err == nil {
		t.Fatal()
	}
	if _, err := Parse([]string{"--dir", "/no/such/dir"}, os.Getenv); err == nil {
		t.Fatal()
	}
}

func TestUsageListsEveryFlagLocalized(t *testing.T) {
	for _, lang := range []string{"en", "zh", "ja", "ko"} {
		var b strings.Builder
		Usage(&b, lang)
		s := b.String()
		fs := newFlagSet(&Config{}, new(string), func(string) string { return "" })
		fs.VisitAll(func(f *flag.Flag) {
			if !strings.Contains(s, "--"+f.Name) {
				t.Errorf("%s: --%s missing", lang, f.Name)
			}
			desc := i18n.T(lang, f.Usage)
			if desc == f.Usage {
				t.Errorf("%s: flag %s usage %q is not an i18n key", lang, f.Name, f.Usage)
			}
			if !strings.Contains(s, desc) {
				t.Errorf("%s: description for --%s missing", lang, f.Name)
			}
		})
		for _, want := range []string{"8848", `"drop"`, "24h0m0s", `"4GB"`, "-h, --help", "landrop send"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s: %q missing from usage:\n%s", lang, want, s)
			}
		}
	}
}

func TestLangFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--help", "--lang", "zh"}, "zh"},
		{[]string{"--lang", "ja", "--help"}, "ja"},
		{[]string{"-lang=ko", "-h"}, "ko"},
		{[]string{"--lang=zh"}, "zh"},
		{[]string{"-lang", "zh"}, "zh"},
		{[]string{"--lang", "xx", "--help"}, "en"}, // invalid → fallback
		{[]string{"--lang"}, "en"},
		{[]string{"--help"}, "en"},
		{[]string{"--", "--lang", "zh"}, "en"},
	}
	for _, c := range cases {
		if got := LangFromArgs(c.args, "en"); got != c.want {
			t.Errorf("%v: got %q want %q", c.args, got, c.want)
		}
	}
}
