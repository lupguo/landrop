package i18n

import (
	"regexp"
	"testing"
)

func TestParity(t *testing.T) {
	for _, l := range Langs {
		for k := range catalogs["en"] {
			if _, ok := catalogs[l][k]; !ok {
				t.Errorf("%s missing %s", l, k)
			}
		}
	}
}

var verbRe = regexp.MustCompile(`%[sd]`)

func TestVerbsMatch(t *testing.T) {
	for _, l := range Langs {
		for k, en := range catalogs["en"] {
			a, b := verbRe.FindAllString(en, -1), verbRe.FindAllString(catalogs[l][k], -1)
			if len(a) != len(b) {
				t.Errorf("%s %s verbs %v vs %v", l, k, a, b)
				continue
			}
			for i := range a {
				if a[i] != b[i] {
					t.Errorf("%s %s verb order %v vs %v", l, k, a, b)
				}
			}
		}
	}
}

func TestFallback(t *testing.T) {
	if T("fr", "banner.code", "123456") != "Access code: 123456" {
		t.Fatal()
	}
	if T("en", "no.such") != "no.such" {
		t.Fatal()
	}
}
