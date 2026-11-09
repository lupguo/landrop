package i18n

var ja = map[string]string{
	KeyBannerTitle:       "LanDrop を起動しました",
	KeyBannerAddrs:       "他のデバイスで開く：",
	KeyBannerRecommended: "おすすめ",
	KeyBannerMDNS:        "または名前でアクセス：%s",
	KeyBannerCode:        "アクセスコード：%s",
	KeyBannerScan:        "スキャンして参加（コード不要）：",
	KeyBannerNoAuth:      "警告：--no-auth が有効です。同じネットワーク上の誰でもアイテムを閲覧・送信できます。",
	KeyHintAPIsolation:   "他のデバイスから接続できない場合、Wi-Fi のクライアント/AP 分離が有効になっている可能性があります。",
	KeyHintFirewallWin:   "接続できない場合は、プライベートネットワークで LanDrop を許可するか、次を実行してください：%s",
	KeyHintMacLocalNet:   "接続できない場合は、「システム設定 → プライバシーとセキュリティ → ローカルネットワーク」でお使いのターミナルを有効にしてください。",
	KeyHintMDNSFailed:    "mDNS を利用できません（%s）。上記の IP アドレスを使用してください。",
	KeyErrPortExhausted:  "ポート %d～%d はすべて使用中です。--port で別のポートを指定してください。",
	KeyInfoKept:          "一時ファイルの保存先：%s",
	KeyInfoBye:           "停止しました。一時ファイルを削除しました。",
	KeySendNotRunning:    "このマシンでは LanDrop が起動していません。",
	KeySendOK:            "送信しました。",
}
