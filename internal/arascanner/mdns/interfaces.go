package mdns

import (
	"net"

	pkgmdns "github.com/jdillenberger/arastack/pkg/mdns"
)

// PhysicalInterfaces returns non-virtual network interfaces, excluding
// Docker bridges, veth pairs, and loopback.
func PhysicalInterfaces() ([]net.Interface, error) {
	return pkgmdns.PhysicalInterfaces()
}
