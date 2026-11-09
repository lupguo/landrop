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
	Print(&b, BannerInfo{Lang: "en", Port: 8848, NoAuth: true, GOOS: "windows", TokenURL: "http://x/?t=1", Addrs: addrs()[:1]})
	s := b.String()
	for _, want := range []string{"http://192.168.1.23:8848", "--no-auth", "netsh advfirewall", "localport=8848", "\x1b[33m", "\x1b[0m", i18n.T("en", i18n.KeyBannerRecommended)} {
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
	Print(&b, BannerInfo{Lang: "zh", Port: 8848, Code: "0427", GOOS: "darwin", MDNSHost: "drop.local", TokenURL: "http://192.168.1.23:8848/?t=abc", Addrs: addrs(), Version: "v1"})
	s := b.String()
	for _, want := range []string{i18n.T("zh", i18n.KeyBannerCode, "0427"), i18n.T("zh", i18n.KeyBannerMDNS, "drop.local"),
		i18n.T("zh", i18n.KeyHintMacLocalNet), i18n.T("zh", i18n.KeyHintAPIsolation), "http://10.0.0.5:8848", "v1"} {
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
