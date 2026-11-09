// Package i18n provides terminal string catalogs.
package i18n

import "fmt"

// Langs lists supported languages; "en" is the base.
var Langs = []string{"en", "zh", "ja", "ko"}

const (
	KeyBannerTitle       = "banner.title"
	KeyBannerAddrs       = "banner.addrs"
	KeyBannerRecommended = "banner.recommended"
	KeyBannerMDNS        = "banner.mdns"
	KeyBannerCode        = "banner.code"
	KeyBannerScan        = "banner.scan"
	KeyBannerNoAuth      = "banner.noauth"
	KeyHintAPIsolation   = "hint.ap_isolation"
	KeyHintFirewallWin   = "hint.firewall_windows"
	KeyHintMacLocalNet   = "hint.macos_localnet"
	KeyHintMDNSFailed    = "hint.mdns_failed"
	KeyErrPortExhausted  = "err.port_exhausted"
	KeyInfoKept          = "info.kept"
	KeyInfoBye           = "info.bye"
	KeySendNotRunning    = "send.not_running"
	KeySendOK            = "send.ok"
	KeySendUsage         = "send.usage"
	KeySendIsDir         = "send.is_dir"
	KeySendFailed        = "send.failed"

	KeyWarnInstanceFile = "warn.instance_file"
	KeyWarnAutosave     = "warn.autosave"
	KeyWarnAutosaveSkip = "warn.autosave_skipped"
	KeyWarnOpenBrowser  = "warn.open_browser"
	KeyCLIError         = "cli.error"
	KeyUsageHeader      = "usage.header"
	KeyUsageDefault     = "usage.default"
	KeyUsageHelp        = "usage.help"
	KeyFlagPort         = "flag.port"
	KeyFlagName         = "flag.name"
	KeyFlagDir          = "flag.dir"
	KeyFlagNoAuth       = "flag.no_auth"
	KeyFlagTTL          = "flag.ttl"
	KeyFlagKeep         = "flag.keep"
	KeyFlagMaxSize      = "flag.max_size"
	KeyFlagNoSave       = "flag.no_save"
	KeyFlagNoOpen       = "flag.no_open"
	KeyFlagLang         = "flag.lang"
	KeyFlagVersion      = "flag.version"
)

var catalogs = map[string]map[string]string{
	"en": en,
	"zh": zh,
	"ja": ja,
	"ko": ko,
}

// T returns the message for key in lang, falling back to en, then to key.
// args are applied with fmt.Sprintf when present.
func T(lang, key string, args ...any) string {
	msg, ok := catalogs[lang][key]
	if !ok {
		msg, ok = catalogs["en"][key]
	}
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}
