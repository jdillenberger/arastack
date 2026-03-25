package avahi

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jdillenberger/arastack/pkg/mdns"
)

const avahiConfigPath = "/etc/avahi/avahi-daemon.conf"

// avahiRestartCooldown prevents rapid avahi-daemon restarts when VPN interfaces flap.
const avahiRestartCooldown = 60 * time.Second

var (
	lastAvahiRestart   time.Time
	lastAvahiRestartMu sync.Mutex
)

// EnsureAvahiConfig makes sure avahi-daemon.conf restricts mDNS to allowed
// network interfaces (physical + VPN) and optionally enables the reflector
// for mDNS bridging across VPN tunnels.
//
// The function is idempotent: it skips if avahi-daemon is not installed.
// It compares desired vs actual config and only restarts when changed.
// Errors are non-fatal (logged via slog). Assumes it runs as root.
func EnsureAvahiConfig(vpnReflector bool) {
	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		return
	}

	data, err := os.ReadFile(avahiConfigPath)
	if err != nil {
		slog.Warn("cannot read avahi config", "path", avahiConfigPath, "error", err)
		return
	}

	ifaces, err := mdns.AllowedInterfaceNames()
	if err != nil || len(ifaces) == 0 {
		slog.Warn("cannot detect allowed interfaces", "error", err)
		return
	}

	vpnIfaces, vpnErr := mdns.VPNInterfaceNames()
	if vpnErr != nil {
		slog.Warn("cannot detect VPN interfaces", "error", vpnErr)
	}
	enableReflector := vpnReflector && len(vpnIfaces) > 0

	newContent := mdns.BuildAvahiConfig(string(data), ifaces, enableReflector)

	if newContent == string(data) {
		return
	}

	// Check cooldown before writing — avoid writing a config we can't activate.
	lastAvahiRestartMu.Lock()
	if time.Since(lastAvahiRestart) < avahiRestartCooldown {
		lastAvahiRestartMu.Unlock()
		slog.Info("avahi config change deferred (cooldown)", "last_restart", lastAvahiRestart)
		return
	}
	lastAvahiRestartMu.Unlock()

	if err := os.WriteFile(avahiConfigPath, []byte(newContent), 0o644); err != nil { // #nosec G306,G703 -- avahi-daemon needs read access, 0644 matches default; path is a const
		if errors.Is(err, os.ErrPermission) {
			slog.Warn("cannot write avahi config (permission denied)", "path", avahiConfigPath)
		} else {
			slog.Warn("cannot write avahi config", "path", avahiConfigPath, "error", err)
		}
		return
	}

	slog.Info("avahi config updated", "allow-interfaces", strings.Join(ifaces, ","), "reflector", enableReflector)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "restart", "avahi-daemon").CombinedOutput(); err != nil { // #nosec G204 -- args are static
		slog.Warn("avahi restart failed", "error", err, "output", string(out))
	}

	lastAvahiRestartMu.Lock()
	lastAvahiRestart = time.Now()
	lastAvahiRestartMu.Unlock()
}
