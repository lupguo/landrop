package app

import (
	"fmt"
	"net"
)

// Listen binds 0.0.0.0 on the first free port in port..port+tries-1 and
// returns the listener and the port it got. It binds IPv4 explicitly: with
// "tcp" Go opens a dual-stack [::] socket, which on macOS/BSD can coexist with
// another program's IPv4 listener on the same port, so the port would look
// free while IPv4 clients still reach the other program.
func Listen(port, tries int) (net.Listener, int, error) {
	var last error
	for p := port; p < port+tries && p <= 65535; p++ {
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
