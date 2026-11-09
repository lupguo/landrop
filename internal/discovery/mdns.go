package discovery

import (
	"net"

	"github.com/grandcat/zeroconf"
)

// Announce advertises name.local over mDNS as an _http._tcp service on ips.
// stop withdraws the announcement.
func Announce(name string, port int, ips []net.IP) (stop func(), err error) {
	strs := make([]string, len(ips))
	for i, ip := range ips {
		strs[i] = ip.String()
	}
	srv, err := zeroconf.RegisterProxy(name, "_http._tcp", "local.", port, name, strs, []string{"path=/"}, nil)
	if err != nil {
		return nil, err
	}
	return srv.Shutdown, nil
}
