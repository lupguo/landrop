package i18n

var zh = map[string]string{
	KeyBannerTitle:       "LanDrop 正在运行",
	KeyBannerAddrs:       "在其他设备上打开：",
	KeyBannerRecommended: "推荐",
	KeyBannerMDNS:        "或通过名称访问：%s",
	KeyBannerCode:        "访问码：%s",
	KeyBannerScan:        "扫码加入（无需访问码）：",
	KeyBannerNoAuth:      "警告：已启用 --no-auth。同一网络中的任何人都可以查看和发送内容。",
	KeyHintAPIsolation:   "如果其他设备无法连接，可能是 Wi-Fi 开启了客户端/AP 隔离。",
	KeyHintFirewallWin:   "如果其他设备无法连接，请在专用网络中允许 LanDrop，或运行：%s",
	KeyHintMacLocalNet:   "如果其他设备无法连接，请在“系统设置 → 隐私与安全性 → 本地网络”中为你的终端开启权限。",
	KeyHintMDNSFailed:    "mDNS 不可用（%s）；请使用上方的 IP 地址。",
	KeyErrPortExhausted:  "端口 %d–%d 均已被占用。请用 --port 指定其他端口。",
	KeyInfoKept:          "临时文件保留在：%s",
	KeyInfoBye:           "已停止，临时文件已清除。",
	KeySendNotRunning:    "本机上的 LanDrop 未在运行。",
	KeySendOK:            "已发送。",
}
