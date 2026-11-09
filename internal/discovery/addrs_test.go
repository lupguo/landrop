package discovery

import (
	"net"
	"testing"
)

func TestRank(t *testing.T) {
	in := []IfaceInfo{
		{"docker0", net.ParseIP("172.17.0.1"), true, false}, {"lo0", net.ParseIP("127.0.0.1"), true, true},
		{"en7", net.ParseIP("10.0.0.5"), true, false}, {"en0", net.ParseIP("192.168.1.23"), true, false},
		{"utun3", net.ParseIP("100.64.0.2"), true, false}, {"en1", net.ParseIP("169.254.3.3"), true, false},
	}
	out := Rank(in)
	if len(out) != 2 || out[0].IP.String() != "192.168.1.23" || !out[0].Recommended || out[1].Recommended {
		t.Fatal(out)
	}
}

func TestRankOrderingAndDrops(t *testing.T) {
	in := []IfaceInfo{
		{"eth1", net.ParseIP("203.0.113.9"), true, false},
		{"eth2", net.ParseIP("172.20.0.4"), true, false},
		{"Tailscale0", net.ParseIP("100.100.1.1"), true, false},
		{"en5", net.ParseIP("10.1.1.1"), true, false},
		{"eth0", net.ParseIP("10.2.2.2"), true, false},
		{"wlan1", net.ParseIP("192.168.5.5"), true, false},
		{"Wi-Fi", net.ParseIP("192.168.0.7"), true, false},
		{"en9", net.ParseIP("192.168.9.9"), false, false},
		{"en8", net.ParseIP("fe80::1"), true, false},
	}
	out := Rank(in)
	var got []string
	for _, a := range out {
		got = append(got, a.IP.String())
	}
	want := []string{"192.168.0.7", "192.168.5.5", "10.2.2.2", "10.1.1.1", "172.20.0.4", "203.0.113.9"}
	if len(got) != len(want) {
		t.Fatal(got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal(got, want)
		}
	}
	if !out[0].Recommended || out[0].Iface != "Wi-Fi" {
		t.Fatal(out[0])
	}
}
