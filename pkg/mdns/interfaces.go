package mdns

import (
	"net"
	"sort"
	"strings"
)

// virtualPrefixes lists network interface name prefixes that are virtual/container-related.
var virtualPrefixes = []string{
	"docker",  // Docker default bridge
	"br-",     // Docker user-defined bridges
	"veth",    // Docker container veth pairs
	"virbr",   // libvirt
	"tun",     // VPN tunnels (OpenVPN-style)
	"tap",     // TAP devices
	"cni",     // CNI networks (Kubernetes/Podman)
	"flannel", // Flannel overlay
	"calico",  // Calico overlay
}

// vpnPrefixes lists network interface name prefixes for VPN tunnels.
var vpnPrefixes = []string{
	"wg",        // WireGuard
	"tailscale", // Tailscale VPN
}

// PhysicalInterfaces returns non-virtual, non-VPN, non-loopback network interfaces.
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
		if isVirtual(iface.Name) || isVPN(iface.Name) {
			continue
		}
		result = append(result, iface)
	}
	return result, nil
}

// PhysicalInterfaceNames returns the names of non-virtual, non-VPN, non-loopback network interfaces.
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

// VPNInterfaces returns active VPN network interfaces.
func VPNInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []net.Interface
	for _, iface := range all {
		if isVPN(iface.Name) {
			result = append(result, iface)
		}
	}
	return result, nil
}

// VPNInterfaceNames returns the names of active VPN network interfaces.
func VPNInterfaceNames() ([]string, error) {
	ifaces, err := VPNInterfaces()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Name
	}
	return names, nil
}

// AllowedInterfaces returns physical + VPN interfaces (for Avahi allow-interfaces).
func AllowedInterfaces() ([]net.Interface, error) {
	phys, err := PhysicalInterfaces()
	if err != nil {
		return nil, err
	}
	vpn, err := VPNInterfaces()
	if err != nil {
		return nil, err
	}
	return append(phys, vpn...), nil
}

// AllowedInterfaceNames returns the names of physical + VPN interfaces,
// sorted for stable ordering (prevents needless avahi config rewrites).
func AllowedInterfaceNames() ([]string, error) {
	ifaces, err := AllowedInterfaces()
	if err != nil {
		return nil, err
	}

	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Name
	}
	sort.Strings(names)
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

func isVPN(name string) bool {
	for _, prefix := range vpnPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
