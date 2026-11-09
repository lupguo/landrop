package app

import (
	"fmt"
	"net"
)

// Listen binds 0.0.0.0 on the first free port in port..port+tries-1 and
// returns the listener and the port it got.
//
// A port counts as busy unless both 127.0.0.1:p and 0.0.0.0:p are free:
//   - On macOS/BSD a wildcard bind succeeds even while another program holds
//     127.0.0.1:p, and loopback dials then reach that program. The browser
//     launch (?t=<token>) and `landrop send` (secret header) dial 127.0.0.1,
//     so they would leak credentials. A loopback probe bind runs first.
//   - It binds IPv4 explicitly. With "tcp", Go opens a dual-stack [::]
//     socket, which can coexist with another program's IPv4 listener on the
//     same port.
func Listen(port, tries int) (net.Listener, int, error) {
	var last error
	for p := port; p < port+tries && p <= 65535; p++ {
		probe, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			last = err
			continue
		}
		probe.Close()
		l, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			return l, p, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no ports to try")
	}
	return nil, 0, fmt.Errorf("listen on ports %d-%d: %w", port, port+tries-1, last)
}
