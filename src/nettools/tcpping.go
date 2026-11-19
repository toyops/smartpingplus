package nettools

import (
	"net"
	"strconv"
	"time"
)

// RunTcpPing performs a TCP handshake (dial) to addr:port and returns the
// round-trip time in milliseconds. It behaves like a "TCP ping": the RTT is
// measured from the start of Dial to the moment the connection is established.
// The connection is closed immediately after a successful handshake.
func RunTcpPing(addr string, port int, timeout time.Duration) (float64, error) {
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return 0, err
	}
	rtt := time.Since(start)
	conn.Close()
	return float64(rtt.Nanoseconds()) / 1e6, nil
}

// ParseTcpTarget accepts a "host:port" string (or "host" with the default port
// 80) and returns the resolved host (IP string) and the port.
func ParseTcpTarget(target string) (string, int) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		// No port supplied: fall back to port 80 (HTTP) so an IP-only
		// target still produces a meaningful TCP probe.
		return target, 80
	}
	p := 80
	if port != "" {
		if parsed, perr := strconv.Atoi(port); perr == nil && parsed > 0 && parsed < 65536 {
			p = parsed
		}
	}
	return host, p
}
