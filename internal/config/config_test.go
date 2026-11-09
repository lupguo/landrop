package config

import (
	"os"
	"testing"
	"time"
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
