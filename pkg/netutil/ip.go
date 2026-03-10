package netutil

import (
	"context"
	"net"
)

// DetectLocalIP returns the primary non-loopback IPv4 address.
func DetectLocalIP() string {
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "udp4", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close() //nolint:errcheck // UDP connection close
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr == nil {
		return ""
	}
	return addr.IP.String()
}
