// Package termui renders the localized terminal startup banner.
package termui

import (
	"fmt"
	"io"
	"strings"

	"github.com/lupguo/landrop/internal/discovery"
	"github.com/lupguo/landrop/internal/i18n"
	"github.com/lupguo/landrop/internal/qr"
)

// BannerInfo is everything the banner shows.
type BannerInfo struct {
	Lang     string
	Addrs    []discovery.Addr
	Port     int
	MDNSHost string // e.g. "drop.local"; "" if failed. Shown as http://<host>:<port>.
	MDNSErr  error
	Code     string
	TokenURL string
	NoAuth   bool
	GOOS     string
	Version  string
}

// Print writes the banner to w.
func Print(w io.Writer, b BannerInfo) {
	t := func(k string, a ...any) string { return i18n.T(b.Lang, k, a...) }
	var sb strings.Builder

	title := t(i18n.KeyBannerTitle)
	if b.Version != "" {
		title += " " + b.Version
	}
	fmt.Fprintf(&sb, "\n%s\n\n%s\n", title, t(i18n.KeyBannerAddrs))
	for _, a := range b.Addrs {
		line := fmt.Sprintf("  http://%s:%d", a.IP, b.Port)
		if a.Recommended {
			line += "  <- " + t(i18n.KeyBannerRecommended)
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")
	switch {
	case b.MDNSErr != nil:
		sb.WriteString(t(i18n.KeyHintMDNSFailed, b.MDNSErr.Error()) + "\n")
	case b.MDNSHost != "":
		sb.WriteString(t(i18n.KeyBannerMDNS, fmt.Sprintf("http://%s:%d", b.MDNSHost, b.Port)) + "\n")
	}
	if b.NoAuth {
		sb.WriteString("\x1b[33m" + t(i18n.KeyBannerNoAuth) + "\x1b[0m\n")
	} else if b.Code != "" {
		sb.WriteString(t(i18n.KeyBannerCode, b.Code) + "\n")
	}
	if b.TokenURL != "" {
		if art, err := qr.ASCII(b.TokenURL); err == nil {
			fmt.Fprintf(&sb, "\n%s\n  %s\n%s\n", t(i18n.KeyBannerScan), b.TokenURL, art)
		}
	}
	sb.WriteString("\n" + t(i18n.KeyHintAPIsolation) + "\n")
	switch b.GOOS {
	case "windows":
		cmd := fmt.Sprintf(`netsh advfirewall firewall add rule name="LanDrop" dir=in action=allow protocol=TCP localport=%d`, b.Port)
		sb.WriteString(t(i18n.KeyHintFirewallWin, cmd) + "\n")
	case "darwin":
		sb.WriteString(t(i18n.KeyHintMacLocalNet) + "\n")
	}
	io.WriteString(w, sb.String())
}
