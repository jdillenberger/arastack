package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/peerscanner/peer"
	"github.com/jdillenberger/arastack/internal/peerscanner/store"
)

func init() {
	inviteCmd.Flags().Duration("ttl", 24*time.Hour, "token validity duration")
	rootCmd.AddCommand(inviteCmd)
}

// inviteCmd generates an invite token for remote peers to join the fleet.
var inviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Generate an invite token for a remote peer",
	RunE: func(cmd *cobra.Command, args []string) error {
		ttl, _ := cmd.Flags().GetDuration("ttl")

		// Load the local store directly (works without daemon).
		st := store.New(dataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		fleet := st.Fleet()

		// Generate a random 32-byte one-time token.
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return fmt.Errorf("generating invite token: %w", err)
		}
		oneTimeToken := hex.EncodeToString(tokenBytes)
		expires := time.Now().Add(ttl)

		// Store the pending invite so the server can validate it later.
		st.AddInvite(peer.PendingInvite{
			Token:   oneTimeToken,
			Expires: expires,
		})

		// Save the store (persists invite + any freshly initialised PSK).
		if err := st.Save(); err != nil {
			return fmt.Errorf("saving store: %w", err)
		}

		// Detect local IP.
		localIP, err := detectLocalIP()
		if err != nil {
			return fmt.Errorf("detecting local IP: %w", err)
		}

		// Build invite token — does NOT contain the PSK.
		token := peer.InviteToken{
			Fleet:   fleet.Name,
			Address: localIP,
			Port:    port,
			Token:   oneTimeToken,
			Expires: expires,
		}

		data, err := json.Marshal(token)
		if err != nil {
			return fmt.Errorf("marshalling token: %w", err)
		}

		encoded := base64.StdEncoding.EncodeToString(data)

		fmt.Printf("Invite token generated (valid for %s).\n\n", ttl)
		fmt.Println("Run this on the remote peer:")
		fmt.Printf("  peer-scanner join %s\n\n", encoded)

		return nil
	},
}

func detectLocalIP() (string, error) {
	// Try outbound UDP dial first (works when there's internet).
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String(), nil
	}

	// Fallback: find the first non-loopback IPv4 address (for air-gapped networks).
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("listing interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}
