package termui

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/lupguo/landrop/internal/discovery"
	"github.com/lupguo/landrop/internal/i18n"
)

func addrs() []discovery.Addr {
	return []discovery.Addr{
		{IP: net.ParseIP("192.168.1.23"), Iface: "en0", Recommended: true},
		{IP: net.ParseIP("10.0.0.5"), Iface: "en7"},
	}
}

func TestBannerWindowsAndNoAuth(t *testing.T) {
	var b bytes.Buffer
	Print(&b, BannerInfo{Lang: "en", Port: 9958, NoAuth: true, GOOS: "windows", TokenURL: "http://x/?t=1", Addrs: addrs()[:1]})
	s := b.String()
	for _, want := range []string{"http://192.168.1.23:9958", "--no-auth", "netsh advfirewall", "localport=9958", "\x1b[33m", "\x1b[0m", i18n.T("en", i18n.KeyBannerRecommended)} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	if strings.Contains(s, "Access code") || strings.Contains(s, "macOS") {
		t.Fatal("unexpected content", s)
	}
}

func TestBannerZh(t *testing.T) {
	var b bytes.Buffer
	Print(&b, BannerInfo{Lang: "zh", Port: 9958, Code: "0427", GOOS: "darwin", MDNSHost: "drop.local", TokenURL: "http://192.168.1.23:9958/?t=abc", Addrs: addrs(), Version: "v1"})
	s := b.String()
	for _, want := range []string{i18n.T("zh", i18n.KeyBannerCode, "0427"), i18n.T("zh", i18n.KeyBannerMDNS, "http://drop.local:9958"),
		i18n.T("zh", i18n.KeyHintMacLocalNet), i18n.T("zh", i18n.KeyHintAPIsolation), "http://10.0.0.5:9958", "v1"} {
		if !strings.Contains(s, want) {
			t.Fatal(want, s)
		}
	}
	if strings.Contains(s, "netsh") || strings.Contains(s, "\x1b[33m") {
		t.Fatal(s)
	}
}

func TestBannerMDNSFailed(t *testing.T) {
	var b bytes.Buffer
	Print(&b, BannerInfo{Lang: "en", Port: 1, MDNSErr: net.ErrClosed, Code: "1234", GOOS: "linux", TokenURL: "http://x/", Addrs: addrs()})
	s := b.String()
	if !strings.Contains(s, i18n.T("en", i18n.KeyHintMDNSFailed, net.ErrClosed.Error())) || strings.Contains(s, "Or via name") {
		t.Fatal(s)
	}
}

// Spec §6.1: the token link is printed as text under the scan heading, above
// the QR; the mDNS line is a full URL.
func TestBannerTokenLinkTextAndMDNSURL(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		var b bytes.Buffer
		tok := "http://192.168.1.23:9958/?t=0123456789abcdef0123456789abcdef"
		Print(&b, BannerInfo{Lang: lang, Port: 9958, Code: "0427", GOOS: "linux", MDNSHost: "drop.local", TokenURL: tok, Addrs: addrs()})
		lines := strings.Split(b.String(), "\n")
		idx := func(s string) int {
			for i, l := range lines {
				if strings.TrimSpace(l) == s {
					return i
				}
			}
			return -1
		}
		scan, link := idx(i18n.T(lang, i18n.KeyBannerScan)), idx(tok)
		if scan < 0 || link != scan+1 {
			t.Fatalf("%s: scan heading %d, token link %d:\n%s", lang, scan, link, b.String())
		}
		if !strings.ContainsAny(strings.Join(lines[link+1:], ""), "█▀▄") {
			t.Fatalf("%s: QR not below token link", lang)
		}
		if !strings.Contains(b.String(), i18n.T(lang, i18n.KeyBannerMDNS, "http://drop.local:9958")) {
			t.Fatalf("%s: mDNS URL missing:\n%s", lang, b.String())
		}
	}
}
