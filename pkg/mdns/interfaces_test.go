package mdns

import "testing"

func Test_isVirtual(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"docker0", true},
		{"br-abc123", true},
		{"veth1234", true},
		{"virbr0", true},
		{"tun0", true},
		{"tap0", true},
		{"cni0", true},
		{"flannel.1", true},
		{"calico0", true},

		// VPN interfaces are NOT virtual
		{"wg0", false},
		{"wg1", false},
		{"tailscale0", false},

		// Physical interfaces
		{"eth0", false},
		{"enp3s0", false},
		{"eno1", false},
		{"wlan0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVirtual(tt.name); got != tt.want {
				t.Errorf("isVirtual(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func Test_isVPN(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"wg0", true},
		{"wg1", true},
		{"tailscale0", true},

		// Virtual but not VPN
		{"docker0", false},
		{"br-abc123", false},
		{"tun0", false},

		// Physical
		{"eth0", false},
		{"enp3s0", false},
		{"wlan0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVPN(tt.name); got != tt.want {
				t.Errorf("isVPN(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
