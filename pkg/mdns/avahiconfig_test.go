package mdns

import (
	"strings"
	"testing"
)

func TestBuildAvahiConfig(t *testing.T) {
	baseConfig := `[server]
#host-name=foo
#allow-interfaces=eth0

[wide-area]
enable-wide-area=yes

[publish]
`

	t.Run("fresh config adds interfaces and reflector", func(t *testing.T) {
		result := BuildAvahiConfig(baseConfig, []string{"eth0", "wg0"}, true)

		if !strings.Contains(result, "allow-interfaces=eth0,wg0") {
			t.Error("expected allow-interfaces=eth0,wg0")
		}
		if !strings.Contains(result, "[reflector]\nenable-reflector=yes") {
			t.Error("expected reflector section with enable-reflector=yes")
		}
	})

	t.Run("updates existing interfaces", func(t *testing.T) {
		existing := strings.Replace(baseConfig, "#allow-interfaces=eth0", "allow-interfaces=eth0", 1)
		result := BuildAvahiConfig(existing, []string{"eth0", "enp3s0", "wg0"}, false)

		if !strings.Contains(result, "allow-interfaces=eth0,enp3s0,wg0") {
			t.Error("expected updated allow-interfaces")
		}
		if strings.Contains(result, "[reflector]") {
			t.Error("should not add reflector section when disabled")
		}
	})

	t.Run("no VPN no reflector", func(t *testing.T) {
		result := BuildAvahiConfig(baseConfig, []string{"eth0"}, false)

		if !strings.Contains(result, "allow-interfaces=eth0") {
			t.Error("expected allow-interfaces=eth0")
		}
		if strings.Contains(result, "enable-reflector=yes") {
			t.Error("should not enable reflector")
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		first := BuildAvahiConfig(baseConfig, []string{"eth0", "wg0"}, true)
		second := BuildAvahiConfig(first, []string{"eth0", "wg0"}, true)

		if first != second {
			t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("disables existing reflector", func(t *testing.T) {
		withReflector := baseConfig + "\n[reflector]\nenable-reflector=yes\n"
		result := BuildAvahiConfig(withReflector, []string{"eth0"}, false)

		if strings.Contains(result, "enable-reflector=yes") {
			t.Error("should have disabled reflector")
		}
		if !strings.Contains(result, "#enable-reflector=no") {
			t.Error("expected commented-out reflector")
		}
	})

	t.Run("disable idempotent", func(t *testing.T) {
		withReflector := baseConfig + "\n[reflector]\nenable-reflector=yes\n"
		first := BuildAvahiConfig(withReflector, []string{"eth0"}, false)
		second := BuildAvahiConfig(first, []string{"eth0"}, false)

		if first != second {
			t.Errorf("disable not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
		}
	})

	t.Run("no server section leaves content unchanged", func(t *testing.T) {
		noServer := "# minimal config\nsome-key=value\n"
		result := BuildAvahiConfig(noServer, []string{"eth0"}, false)

		if strings.Contains(result, "allow-interfaces=") {
			t.Error("should not insert allow-interfaces without [server] section")
		}
	})

	t.Run("handles spaced comment variant", func(t *testing.T) {
		spacedConfig := "[server]\n# allow-interfaces=eth0\n\n[wide-area]\n"
		result := BuildAvahiConfig(spacedConfig, []string{"eth0", "wg0"}, false)

		if !strings.Contains(result, "allow-interfaces=eth0,wg0") {
			t.Errorf("expected allow-interfaces=eth0,wg0, got:\n%s", result)
		}
		if strings.Count(result, "allow-interfaces") != 1 {
			t.Errorf("expected exactly one allow-interfaces line, got:\n%s", result)
		}
	})

	t.Run("handles spaces around equals", func(t *testing.T) {
		spacedConfig := "[server]\nallow-interfaces = eth0\n\n[reflector]\nenable-reflector = yes\n"
		result := BuildAvahiConfig(spacedConfig, []string{"eth0", "wg0"}, false)

		if !strings.Contains(result, "allow-interfaces=eth0,wg0") {
			t.Errorf("expected allow-interfaces=eth0,wg0, got:\n%s", result)
		}
		if strings.Contains(result, "enable-reflector=yes") && !strings.Contains(result, "#enable-reflector=no") {
			t.Errorf("expected reflector disabled, got:\n%s", result)
		}
	})

	t.Run("enables reflector when explicitly disabled", func(t *testing.T) {
		withDisabled := baseConfig + "\n[reflector]\nenable-reflector=no\n"
		result := BuildAvahiConfig(withDisabled, []string{"eth0", "wg0"}, true)

		if !strings.Contains(result, "enable-reflector=yes") {
			t.Error("expected enable-reflector=yes")
		}
		if strings.Contains(result, "enable-reflector=no") {
			t.Error("should not have enable-reflector=no")
		}
	})
}
