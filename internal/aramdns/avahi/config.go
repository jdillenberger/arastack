package avahi

import (
	"context"
	"errors"
	"log/slog"
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
// allow-interfaces is already configured. Errors are non-fatal (logged via slog).
func EnsureAvahiConfig() {
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		return
	}

	data, err := os.ReadFile(avahiConfigPath)
	if err != nil {
		slog.Warn("cannot read avahi config", "path", avahiConfigPath, "error", err)
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
		slog.Warn("cannot detect physical interfaces", "error", err)
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
			slog.Warn("cannot write avahi config (permission denied)", "path", avahiConfigPath, "directive", directive)
		} else {
			slog.Warn("cannot write avahi config", "path", avahiConfigPath, "error", err)
		}
		return
	}

	slog.Info("avahi configured", "allow-interfaces", ifaceList)

	if out, err := exec.CommandContext(context.Background(), "systemctl", "restart", "avahi-daemon").CombinedOutput(); err != nil { // #nosec G204 -- args are static
		slog.Warn("avahi restart failed", "error", err, "output", string(out))
	}
}
