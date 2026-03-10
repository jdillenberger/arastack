package avahi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jdillenberger/arastack/pkg/mdns"
)

const avahiConfigPath = "/etc/avahi/avahi-daemon.conf"

// EnsureAvahiConfig makes sure avahi-daemon.conf restricts mDNS to physical
// network interfaces so that Docker bridge interfaces don't cause hostname
// resolution to return container-network IPs.
//
// The function is idempotent: it skips if avahi-daemon is not installed or
// allow-interfaces is already configured. Errors are non-fatal (logged to stderr).
func EnsureAvahiConfig() {
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		return
	}

	data, err := os.ReadFile(avahiConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "avahi: cannot read %s: %v\n", avahiConfigPath, err)
		return
	}

	// If allow-interfaces is already configured (uncommented), skip.
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "allow-interfaces=") {
			return
		}
	}

	ifaces, err := mdns.PhysicalInterfaceNames()
	if err != nil || len(ifaces) == 0 {
		fmt.Fprintf(os.Stderr, "avahi: cannot detect physical interfaces: %v\n", err)
		return
	}

	content := string(data)
	ifaceList := strings.Join(ifaces, ",")
	directive := "allow-interfaces=" + ifaceList

	if strings.Contains(content, "#allow-interfaces=") {
		content = strings.Replace(content, "#allow-interfaces=eth0", directive, 1)
	} else {
		content = strings.Replace(content, "[server]\n", "[server]\n"+directive+"\n", 1)
	}

	if err := os.WriteFile(avahiConfigPath, []byte(content), 0o600); err != nil { // #nosec G703 -- avahiConfigPath is a well-known system path
		if errors.Is(err, os.ErrPermission) {
			fmt.Fprintf(os.Stderr, "avahi: cannot write %s (permission denied). Run with sudo or manually set:\n  %s\n", avahiConfigPath, directive)
		} else {
			fmt.Fprintf(os.Stderr, "avahi: cannot write %s: %v\n", avahiConfigPath, err)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "avahi: configured allow-interfaces=%s\n", ifaceList)

	if out, err := exec.CommandContext(context.Background(), "systemctl", "restart", "avahi-daemon").CombinedOutput(); err != nil { // #nosec G204 -- args are static
		fmt.Fprintf(os.Stderr, "avahi: restart failed: %v: %s\n", err, out)
	}
}
