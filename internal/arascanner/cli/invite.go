package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

func init() {
	inviteCmd.Flags().Duration("ttl", 24*time.Hour, "token validity duration")
	rootCmd.AddCommand(inviteCmd)
}

// inviteCmd generates an invite token for remote peers to join the peer group.
var inviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Generate an invite token for a remote peer",
	Example: `  arascanner invite
  arascanner invite --ttl 1h`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ttl, _ := cmd.Flags().GetDuration("ttl")

		// Load the local store to get PSK and peer group info.
		st := store.New(cfg.Server.DataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		pg := st.PeerGroup()

		var oneTimeToken string
		var expires time.Time

		// Try the daemon API first so the invite is registered in-memory.
		if token, exp, err := createInviteViaAPI(cfg.Server.Port, pg.Secret, ttl); err == nil {
			oneTimeToken = token
			expires = exp
		} else {
			// Fallback: generate locally and write to disk directly.
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err != nil {
				return fmt.Errorf("generating invite token: %w", err)
			}
			oneTimeToken = hex.EncodeToString(tokenBytes)
			expires = time.Now().Add(ttl)

			st.AddInvite(peer.PendingInvite{
				Token:   oneTimeToken,
				Expires: expires,
			})

			if err := st.Save(); err != nil {
				return fmt.Errorf("saving store: %w", err)
			}
		}

		// Detect local IP.
		localIP, err := detectLocalIP()
		if err != nil {
			return fmt.Errorf("detecting local IP: %w", err)
		}

		// Read local CA cert for the invite token.
		var caCert string
		ldc, ldcErr := aradeployconfig.Load("")
		if ldcErr == nil {
			caPath := filepath.Join(ldc.DataDir, "traefik", "certs", "ca.crt")
			if data, err := os.ReadFile(caPath); err == nil { // #nosec G304 -- path is constructed internally
				caCert = string(data)
			}
		}

		// Build invite token — does NOT contain the PSK.
		token := peer.InviteToken{
			PeerGroup: pg.Name,
			Address:   localIP,
			Port:      cfg.Server.Port,
			Token:     oneTimeToken,
			CACert:    caCert,
			Expires:   expires,
		}

		data, err := json.Marshal(token)
		if err != nil {
			return fmt.Errorf("marshalling token: %w", err)
		}

		encoded := base64.StdEncoding.EncodeToString(data)

		fmt.Printf("Invite token generated (valid for %s).\n\n", ttl)
		fmt.Println("Run this on the remote peer:")
		fmt.Printf("  arascanner join %s\n\n", encoded)

		return nil
	},
}

// createInviteViaAPI tries to create an invite through the running daemon's API.
func createInviteViaAPI(port int, psk string, ttl time.Duration) (token string, expires time.Time, err error) {
	reqBody, _ := json.Marshal(struct {
		TTLSeconds int `json:"ttl_seconds"`
	}{TTLSeconds: int(ttl.Seconds())})

	url := fmt.Sprintf("http://127.0.0.1:%d/api/invites", port)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if psk != "" {
		req.Header.Set("Authorization", "Bearer "+psk)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Token   string    `json:"token"`
		Expires time.Time `json:"expires"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, err
	}
	return result.Token, result.Expires, nil
}

func detectLocalIP() (string, error) {
	// Try outbound UDP dial first (works when there's internet).
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "udp", "8.8.8.8:53")
	if err == nil {
		defer conn.Close() //nolint:errcheck // UDP connection close
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
