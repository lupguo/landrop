package i18n

var en = map[string]string{
	KeyBannerTitle:       "LanDrop is running",
	KeyBannerAddrs:       "Open on another device:",
	KeyBannerRecommended: "recommended",
	KeyBannerMDNS:        "Or via name: %s",
	KeyBannerCode:        "Access code: %s",
	KeyBannerScan:        "Scan to join (no code needed):",
	KeyBannerNoAuth:      "WARNING: --no-auth is on. Anyone on this network can read and send items.",
	KeyHintAPIsolation:   "If other devices can't connect, the Wi-Fi may have client/AP isolation enabled.",
	KeyHintFirewallWin:   "If peers can't connect, allow LanDrop on Private networks, or run: %s",
	KeyHintMacLocalNet:   "If peers can't connect, enable System Settings → Privacy & Security → Local Network for your terminal.",
	KeyHintMDNSFailed:    "mDNS unavailable (%s); use the IP address above.",
	KeyErrPortExhausted:  "Ports %d–%d are all in use. Pick another with --port.",
	KeyInfoKept:          "Temp files kept at: %s",
	KeyInfoBye:           "Stopped. Temporary files removed.",
	KeySendNotRunning:    "LanDrop is not running on this machine.",
	KeySendOK:            "Sent.",
}
