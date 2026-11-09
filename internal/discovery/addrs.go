// Package discovery ranks LAN addresses and announces the service over mDNS.
package discovery

import (
	"net"
	"regexp"
	"sort"
)

// IfaceInfo is one interface address candidate.
type IfaceInfo struct {
	Name     string
	IP       net.IP
	Up       bool
	Loopback bool
}

// Addr is a ranked usable address.
type Addr struct {
	IP          net.IP
	Iface       string
	Recommended bool
}

var virtualName = regexp.MustCompile(`(?i)^(docker|br-|veth|utun|vboxnet|vmnet|tun|tap|llw|awdl|bridge|zt|tailscale)`)

var preferredName = map[string]bool{"en0": true, "eth0": true, "wlan0": true, "wi-fi": true, "ethernet": true}

func class(ip net.IP) int {
	switch {
	case ip[0] == 192 && ip[1] == 168:
		return 0
	case ip[0] == 10:
		return 1
	case ip[0] == 172 && ip[1]&0xf0 == 16:
		return 2
	}
	return 3
}

// Rank filters and orders candidates; the first result is Recommended.
func Rank(in []IfaceInfo) []Addr {
	type cand struct {
		a    Addr
		cls  int
		pref bool
	}
	var cs []cand
	for _, i := range in {
		ip := i.IP.To4()
		if ip == nil || !i.Up || i.Loopback || ip.IsLoopback() || ip.IsLinkLocalUnicast() || virtualName.MatchString(i.Name) {
			continue
		}
		cs = append(cs, cand{Addr{IP: ip, Iface: i.Name}, class(ip), preferredName[lower(i.Name)]})
	}
	sort.SliceStable(cs, func(a, b int) bool {
		if cs[a].cls != cs[b].cls {
			return cs[a].cls < cs[b].cls
		}
		return cs[a].pref && !cs[b].pref
	})
	out := make([]Addr, len(cs))
	for i, c := range cs {
		out[i] = c.a
	}
	if len(out) > 0 {
		out[0].Recommended = true
	}
	return out
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// ListAddrs enumerates system interfaces and ranks their addresses.
func ListAddrs() ([]Addr, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var in []IfaceInfo
	for _, ifc := range ifs {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			in = append(in, IfaceInfo{Name: ifc.Name, IP: ip, Up: ifc.Flags&net.FlagUp != 0, Loopback: ifc.Flags&net.FlagLoopback != 0})
		}
	}
	return Rank(in), nil
}
