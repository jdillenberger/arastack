package mdns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jdillenberger/arastack/pkg/doctor"
)

// Dependency defines a system dependency to check.
type Dependency struct {
	Name           string
	Binary         string   // binary name to look up with `which`
	Library        string   // shared library glob pattern
	VersionArgs    []string // args to get version
	InstallCommand string   // apt install command
}

// Dependencies returns the list of mDNS-relevant dependencies to check.
func Dependencies() []Dependency {
	return []Dependency{
		{
			Name:           "avahi-daemon",
			Binary:         "avahi-daemon",
			VersionArgs:    []string{"--version"},
			InstallCommand: "apt install -y avahi-daemon",
		},
		{
			Name:           "avahi-utils",
			Binary:         "avahi-browse",
			VersionArgs:    []string{"--version"},
			InstallCommand: "apt install -y avahi-utils",
		},
		{
			Name:           "libnss-mdns",
			Library:        "libnss_mdns*.so*",
			InstallCommand: "apt install -y libnss-mdns",
		},
	}
}

// CheckDependency runs a single dependency check.
func CheckDependency(dep Dependency) doctor.CheckResult {
	result := doctor.CheckResult{
		Name:           dep.Name,
		InstallCommand: dep.InstallCommand,
	}

	if dep.Library != "" {
		return checkLibrary(dep.Library, result)
	}

	path, err := exec.LookPath(dep.Binary)
	if err != nil {
		return result
	}

	result.Installed = true

	if len(dep.VersionArgs) > 0 {
		cmd := exec.CommandContext(context.Background(), path, dep.VersionArgs...) // #nosec G204 -- path is from LookPath
		out, err := cmd.CombinedOutput()
		if err == nil {
			ver := strings.TrimSpace(string(out))
			if idx := strings.IndexByte(ver, '\n'); idx != -1 {
				ver = ver[:idx]
			}
			result.Version = ver
		}
	}

	return result
}

func checkLibrary(pattern string, result doctor.CheckResult) doctor.CheckResult {
	libDirs := []string{"/lib", "/usr/lib"}
	cmd := exec.CommandContext(context.Background(), "find", append(libDirs, "-name", pattern, "-type", "f")...) // #nosec G204 -- pattern is from internal config
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		result.Installed = true
		result.Version = "installed"
	}
	return result
}

// CheckAllDependencies checks all mDNS dependencies and system configuration.
func CheckAllDependencies() []doctor.CheckResult {
	deps := Dependencies()
	results := make([]doctor.CheckResult, len(deps))
	for i, dep := range deps {
		results[i] = CheckDependency(dep)
	}
	results = append(results, CheckNSSwitchMDNS())
	results = append(results, CheckAvahiRunning())
	results = append(results, CheckAvahiInterfaces())
	results = append(results, CheckResolvedMDNS())
	results = append(results, CheckAvahiHostnameConflict())
	results = append(results, CheckAvahiReflector())
	results = append(results, CheckAvahiVPNInterfaces())
	return results
}

// CheckNSSwitchMDNS checks that /etc/nsswitch.conf has mdns4 (not mdns4_minimal)
// in the hosts line, and that /etc/mdns.allow is configured.
func CheckNSSwitchMDNS() doctor.CheckResult {
	result := doctor.CheckResult{Name: "nsswitch-mdns"}

	data, err := os.ReadFile("/etc/nsswitch.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		result.Version = "cannot read /etc/nsswitch.conf"
		return result
	}

	hasMDNS4 := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "hosts:") {
			fields := strings.Fields(line)
			for _, f := range fields {
				if f == "mdns4" || f == "mdns" {
					hasMDNS4 = true
				}
			}
			if !hasMDNS4 {
				if strings.Contains(line, "mdns4_minimal") {
					result.Version = "mdns4_minimal configured (only single-label .local — run doctor --fix)"
				} else {
					result.Version = "mdns NOT configured in hosts line"
				}
				return result
			}
			break
		}
	}

	if !hasMDNS4 {
		result.Version = "no hosts line found"
		return result
	}

	allowData, err := os.ReadFile("/etc/mdns.allow") // #nosec G304 -- fixed system config path
	if err != nil {
		result.Version = "mdns4 in nsswitch but /etc/mdns.allow missing (run doctor --fix)"
		return result
	}
	if strings.Contains(string(allowData), ".local") {
		result.Installed = true
		result.Version = "mdns4 configured, mdns.allow present"
	} else {
		result.Version = "mdns4 in nsswitch but /etc/mdns.allow missing .local entry (run doctor --fix)"
	}
	return result
}

// CheckAvahiRunning checks if avahi-daemon is active.
func CheckAvahiRunning() doctor.CheckResult {
	result := doctor.CheckResult{
		Name:           "avahi-daemon-running",
		InstallCommand: "systemctl enable --now avahi-daemon",
	}

	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "avahi-daemon") // #nosec G204 -- args are not user-controlled
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		result.Installed = true
		result.Version = "active"
	} else {
		result.Version = strings.TrimSpace(string(out))
	}
	return result
}

// CheckAvahiInterfaces verifies that avahi-daemon.conf restricts mDNS to
// physical interfaces so Docker bridge interfaces don't hijack .local resolution.
func CheckAvahiInterfaces() doctor.CheckResult {
	result := doctor.CheckResult{Name: "avahi-interfaces"}

	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		result.Version = "avahi-daemon not installed, skipped"
		result.Installed = true
		return result
	}

	data, err := os.ReadFile("/etc/avahi/avahi-daemon.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		result.Version = "cannot read /etc/avahi/avahi-daemon.conf"
		return result
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "allow-interfaces=") {
			result.Installed = true
			result.Version = strings.TrimSpace(line)
			return result
		}
	}

	ifaces, _ := AllowedInterfaceNames()
	result.Version = fmt.Sprintf("allow-interfaces not set — Docker interfaces may hijack .local resolution (run doctor --fix to set allow-interfaces=%s)", strings.Join(ifaces, ","))
	return result
}

// CheckResolvedMDNS checks if systemd-resolved has mDNS enabled globally.
func CheckResolvedMDNS() doctor.CheckResult {
	result := doctor.CheckResult{Name: "resolved-mdns"}

	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "systemd-resolved") // #nosec G204 -- args are not user-controlled
	out, _ := cmd.CombinedOutput()
	if strings.TrimSpace(string(out)) != "active" {
		result.Installed = true
		result.Version = "systemd-resolved not active, skipped"
		return result
	}

	cmd = exec.CommandContext(context.Background(), "resolvectl", "status") // #nosec G204 -- args are not user-controlled
	out, err := cmd.CombinedOutput()
	if err != nil {
		result.Version = "cannot query resolvectl status"
		return result
	}

	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Protocols:") {
			if strings.Contains(trimmed, "+mDNS") {
				result.Installed = true
				result.Version = "mDNS enabled globally"
			} else if strings.Contains(trimmed, "-mDNS") {
				result.Version = "mDNS disabled globally in systemd-resolved (run doctor --fix)"
			}
			return result
		}
	}

	result.Version = "could not determine mDNS status"
	return result
}

// CheckAvahiHostnameConflict detects if avahi-daemon has renamed the hostname
// due to mDNS conflicts, typically caused by orphaned avahi-publish processes.
func CheckAvahiHostnameConflict() doctor.CheckResult {
	result := doctor.CheckResult{Name: "avahi-hostname-conflict"}

	hostname, err := os.Hostname()
	if err != nil {
		result.Version = "cannot determine hostname"
		return result
	}

	cmd := exec.CommandContext(context.Background(), "avahi-resolve", "-n", hostname+".local") // #nosec G204 -- hostname is from os.Hostname
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		result.Installed = true
		result.Version = hostname + ".local resolves OK"
		return result
	}

	cmd = exec.CommandContext(context.Background(), "journalctl", "-u", "avahi-daemon", "--since", "1 hour ago", "--no-pager", "-q") // #nosec G204 -- args are not user-controlled
	out, _ = cmd.CombinedOutput()
	if strings.Contains(string(out), "Host name conflict") {
		result.Version = hostname + ".local has hostname conflict — kill orphaned avahi-publish processes and restart avahi-daemon"
		return result
	}

	result.Version = hostname + ".local not resolvable via mDNS"
	return result
}

// CheckAvahiReflector checks if the Avahi reflector is enabled when VPN interfaces exist.
func CheckAvahiReflector() doctor.CheckResult {
	result := doctor.CheckResult{Name: "avahi-reflector"}

	vpnIfaces, _ := VPNInterfaceNames()
	if len(vpnIfaces) == 0 {
		result.Installed = true
		result.Version = "no VPN interfaces, reflector not needed"
		return result
	}

	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		result.Installed = true
		result.Version = "avahi-daemon not installed, skipped"
		return result
	}

	data, err := os.ReadFile("/etc/avahi/avahi-daemon.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		result.Version = "cannot read /etc/avahi/avahi-daemon.conf"
		return result
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "enable-reflector=yes" {
			result.Installed = true
			result.Version = fmt.Sprintf("reflector enabled (VPN: %s)", strings.Join(vpnIfaces, ","))
			return result
		}
	}

	result.Version = fmt.Sprintf("VPN interfaces detected (%s) but reflector not enabled (run doctor --fix)", strings.Join(vpnIfaces, ","))
	return result
}

// CheckAvahiVPNInterfaces verifies that VPN interfaces are included in avahi allow-interfaces.
func CheckAvahiVPNInterfaces() doctor.CheckResult {
	result := doctor.CheckResult{Name: "avahi-vpn-interfaces"}

	vpnIfaces, _ := VPNInterfaceNames()
	if len(vpnIfaces) == 0 {
		result.Installed = true
		result.Version = "no VPN interfaces present"
		return result
	}

	if _, err := exec.LookPath("avahi-daemon"); err != nil {
		result.Installed = true
		result.Version = "avahi-daemon not installed, skipped"
		return result
	}

	data, err := os.ReadFile("/etc/avahi/avahi-daemon.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		result.Version = "cannot read /etc/avahi/avahi-daemon.conf"
		return result
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "allow-interfaces=") {
			allowed := strings.TrimPrefix(trimmed, "allow-interfaces=")
			allowedSet := make(map[string]bool)
			for _, name := range strings.Split(allowed, ",") {
				allowedSet[strings.TrimSpace(name)] = true
			}

			var missing []string
			for _, vpn := range vpnIfaces {
				if !allowedSet[vpn] {
					missing = append(missing, vpn)
				}
			}

			if len(missing) > 0 {
				result.Version = fmt.Sprintf("VPN interfaces not in allow-interfaces: %s (run doctor --fix)", strings.Join(missing, ","))
				return result
			}

			result.Installed = true
			result.Version = fmt.Sprintf("VPN interfaces included: %s", strings.Join(vpnIfaces, ","))
			return result
		}
	}

	result.Version = fmt.Sprintf("allow-interfaces not set — VPN interfaces (%s) not allowed (run doctor --fix)", strings.Join(vpnIfaces, ","))
	return result
}

// FixAvahiReflector enables the Avahi reflector and adds VPN interfaces to allow-interfaces.
func FixAvahiReflector() error {
	return fixAvahiConfig(true)
}

// FixNSSwitchMDNS fixes the nsswitch.conf and mdns.allow configuration.
func FixNSSwitchMDNS() error {
	data, err := os.ReadFile("/etc/nsswitch.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		return fmt.Errorf("reading /etc/nsswitch.conf: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	nssModified := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "hosts:") {
			if strings.Contains(trimmed, "mdns4_minimal") {
				lines[i] = strings.Replace(line, "mdns4_minimal", "mdns4", 1)
				nssModified = true
			} else if !strings.Contains(trimmed, "mdns4") && !strings.Contains(trimmed, "mdns") {
				lines[i] = strings.Replace(line, "dns", "mdns4 [NOTFOUND=return] dns", 1)
				nssModified = true
			}
			break
		}
	}

	if nssModified {
		tmpFile, err := os.CreateTemp("", "nsswitch.conf.arastack.*")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		if _, err := tmpFile.WriteString(strings.Join(lines, "\n")); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("writing temp file: %w", err)
		}
		_ = tmpFile.Close()
		cmd := exec.CommandContext(context.Background(), "sudo", "cp", tmpPath, "/etc/nsswitch.conf") // #nosec G204 -- args are not user-controlled
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("updating /etc/nsswitch.conf: %w\n%s", err, string(out))
		}
		fmt.Println("    Updated /etc/nsswitch.conf: mdns4_minimal -> mdns4")
	}

	allowData, _ := os.ReadFile("/etc/mdns.allow") // #nosec G304 -- fixed system config path
	if !strings.Contains(string(allowData), ".local") {
		tmpFile, err := os.CreateTemp("", "mdns.allow.arastack.*")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		if _, err := tmpFile.WriteString(".local\n.local.\n"); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("writing temp file: %w", err)
		}
		_ = tmpFile.Close()
		cmd := exec.CommandContext(context.Background(), "sudo", "cp", tmpPath, "/etc/mdns.allow") // #nosec G204 -- args are not user-controlled
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("creating /etc/mdns.allow: %w\n%s", err, string(out))
		}
		fmt.Println("    Created /etc/mdns.allow with .local domain")
	}

	return nil
}

// FixAvahiInterfaces sets allow-interfaces in avahi-daemon.conf to allowed interfaces (physical + VPN).
// It preserves the existing reflector state.
func FixAvahiInterfaces() error {
	return fixAvahiConfig(currentReflectorEnabled())
}

// currentReflectorEnabled reads the current avahi config and returns whether the reflector is enabled.
func currentReflectorEnabled() bool {
	data, err := os.ReadFile("/etc/avahi/avahi-daemon.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "enable-reflector=yes" {
			return true
		}
	}
	return false
}

// fixAvahiConfig updates avahi-daemon.conf with allowed interfaces and optionally enables the reflector.
func fixAvahiConfig(enableReflector bool) error {
	ifaces, err := AllowedInterfaceNames()
	if err != nil {
		return fmt.Errorf("detecting allowed interfaces: %w", err)
	}
	if len(ifaces) == 0 {
		return fmt.Errorf("no allowed interfaces detected")
	}

	data, err := os.ReadFile("/etc/avahi/avahi-daemon.conf") // #nosec G304 -- fixed system config path
	if err != nil {
		return fmt.Errorf("reading /etc/avahi/avahi-daemon.conf: %w", err)
	}

	content := BuildAvahiConfig(string(data), ifaces, enableReflector)

	tmpFile, err := os.CreateTemp("", "avahi-daemon.conf.arastack.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	_ = tmpFile.Close()
	cmd := exec.CommandContext(context.Background(), "sudo", "cp", tmpPath, "/etc/avahi/avahi-daemon.conf") // #nosec G204 -- args are not user-controlled
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("updating /etc/avahi/avahi-daemon.conf: %w\n%s", err, string(out))
	}

	ifaceList := strings.Join(ifaces, ",")
	if enableReflector {
		fmt.Printf("    Set allow-interfaces=%s and enable-reflector=yes in /etc/avahi/avahi-daemon.conf\n", ifaceList)
	} else {
		fmt.Printf("    Set allow-interfaces=%s in /etc/avahi/avahi-daemon.conf\n", ifaceList)
	}

	cmd = exec.CommandContext(context.Background(), "sudo", "systemctl", "restart", "avahi-daemon") // #nosec G204 -- args are not user-controlled
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restarting avahi-daemon: %w\n%s", err, string(out))
	}

	fmt.Println("    Restarted avahi-daemon")
	return nil
}

// FixResolvedMDNS enables mDNS in systemd-resolved via a drop-in config.
func FixResolvedMDNS() error {
	dropInDir := "/etc/systemd/resolved.conf.d"
	dropInFile := dropInDir + "/arastack-mdns.conf"
	content := "[Resolve]\nMulticastDNS=resolve\n"

	cmd := exec.CommandContext(context.Background(), "sudo", "mkdir", "-p", dropInDir) // #nosec G204 -- args are not user-controlled
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", dropInDir, err)
	}

	cmd = exec.CommandContext(context.Background(), "sudo", "tee", dropInFile) // #nosec G204 -- args are not user-controlled
	cmd.Stdin = strings.NewReader(content)
	cmd.Stderr = os.Stderr
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("writing %s: %w", dropInFile, err)
	}
	fmt.Printf("    Created %s\n", dropInFile)

	cmd = exec.CommandContext(context.Background(), "sudo", "systemctl", "restart", "systemd-resolved") // #nosec G204 -- args are not user-controlled
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restarting systemd-resolved: %w", err)
	}
	fmt.Println("    Restarted systemd-resolved")

	return nil
}

// FixAvahiHostnameConflict kills orphaned avahi-publish processes and restarts avahi-daemon.
func FixAvahiHostnameConflict() error {
	cmd := exec.CommandContext(context.Background(), "pkill", "-f", "avahi-publish") // #nosec G204 -- args are not user-controlled
	_ = cmd.Run()

	cmd = exec.CommandContext(context.Background(), "sudo", "systemctl", "restart", "avahi-daemon") // #nosec G204 -- args are not user-controlled
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restarting avahi-daemon: %w\n%s", err, string(out))
	}

	fmt.Println("    Killed orphaned avahi-publish processes and restarted avahi-daemon")
	return nil
}

// FixDependency installs a missing dependency using its install command.
func FixDependency(result doctor.CheckResult) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "nsswitch-mdns":
		return FixNSSwitchMDNS()
	case "avahi-interfaces":
		return FixAvahiInterfaces()
	case "resolved-mdns":
		return FixResolvedMDNS()
	case "avahi-hostname-conflict":
		return FixAvahiHostnameConflict()
	case "avahi-reflector":
		return FixAvahiReflector()
	case "avahi-vpn-interfaces":
		return FixAvahiInterfaces()
	}

	if result.InstallCommand == "" {
		return fmt.Errorf("no install command for %s", result.Name)
	}

	parts := strings.Fields(result.InstallCommand)
	cmd := exec.CommandContext(context.Background(), "sudo", parts...) // #nosec G204 -- install command from internal config
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing %s: %w\n%s", result.Name, err, string(out))
	}
	return nil
}
