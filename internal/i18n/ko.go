package i18n

var ko = map[string]string{
	KeyBannerTitle:       "LanDrop 실행 중",
	KeyBannerAddrs:       "다른 기기에서 열기:",
	KeyBannerRecommended: "추천",
	KeyBannerMDNS:        "또는 이름으로 접속: %s",
	KeyBannerCode:        "접속 코드: %s",
	KeyBannerScan:        "스캔하여 참여 (코드 불필요):",
	KeyBannerNoAuth:      "경고: --no-auth가 켜져 있습니다. 같은 네트워크의 누구나 항목을 보고 보낼 수 있습니다.",
	KeyHintAPIsolation:   "다른 기기가 연결되지 않으면 Wi-Fi의 클라이언트/AP 격리가 켜져 있을 수 있습니다.",
	KeyHintFirewallWin:   "연결되지 않으면 개인 네트워크에서 LanDrop을 허용하거나 다음을 실행하세요: %s",
	KeyHintMacLocalNet:   "연결되지 않으면 시스템 설정 → 개인정보 보호 및 보안 → 로컬 네트워크에서 터미널을 허용하세요.",
	KeyHintMDNSFailed:    "mDNS를 사용할 수 없습니다(%s). 위의 IP 주소를 사용하세요.",
	KeyErrPortExhausted:  "포트 %d–%d가 모두 사용 중입니다. --port로 다른 포트를 지정하세요.",
	KeyInfoKept:          "임시 파일 보관 위치: %s",
	KeyInfoBye:           "중지되었습니다. 임시 파일을 삭제했습니다.",
	KeySendNotRunning:    "이 컴퓨터에서 LanDrop이 실행 중이 아닙니다.",
	KeySendOK:            "보냈습니다.",
}
