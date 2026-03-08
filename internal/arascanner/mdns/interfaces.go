package mdns

import (
	"net"
	"strings"
)

// PhysicalInterfaces returns non-virtual network interfaces, excluding
// Docker bridges, veth pairs, and loopback.
func PhysicalInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []net.Interface
	for _, iface := range all {
		name := iface.Name
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "virbr") ||
			strings.HasPrefix(name, "tun") ||
			strings.HasPrefix(name, "tap") ||
			strings.HasPrefix(name, "wg") ||
			strings.HasPrefix(name, "tailscale") ||
			strings.HasPrefix(name, "cni") ||
			strings.HasPrefix(name, "flannel") ||
			strings.HasPrefix(name, "calico") {
			continue
		}
		result = append(result, iface)
	}
	return result, nil
}
