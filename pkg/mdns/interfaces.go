package mdns

import (
	"net"
	"strings"
)

// virtualPrefixes lists network interface name prefixes that are virtual/container-related.
var virtualPrefixes = []string{
	"docker",    // Docker default bridge
	"br-",       // Docker user-defined bridges
	"veth",      // Docker container veth pairs
	"virbr",     // libvirt
	"tun",       // VPN tunnels
	"tap",       // TAP devices
	"wg",        // WireGuard
	"tailscale", // Tailscale VPN
	"cni",       // CNI networks (Kubernetes/Podman)
	"flannel",   // Flannel overlay
	"calico",    // Calico overlay
}

// PhysicalInterfaces returns non-virtual, non-loopback network interfaces.
func PhysicalInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []net.Interface
	for _, iface := range all {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtual(iface.Name) {
			continue
		}
		result = append(result, iface)
	}
	return result, nil
}

// PhysicalInterfaceNames returns the names of non-virtual, non-loopback network interfaces.
func PhysicalInterfaceNames() ([]string, error) {
	ifaces, err := PhysicalInterfaces()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Name
	}
	return names, nil
}

func isVirtual(name string) bool {
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
