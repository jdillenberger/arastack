package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(joinCmd)
}

// joinCmd joins a peer group using an invite token from another peer.
var joinCmd = &cobra.Command{
	Use:   "join <token>",
	Short: "Join a peer group using an invite token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Decode the base64 token.
		raw, err := base64.StdEncoding.DecodeString(args[0])
		if err != nil {
			return fmt.Errorf("invalid token encoding: %w", err)
		}

		// Parse JSON into InviteToken.
		var token peer.InviteToken
		if err := json.Unmarshal(raw, &token); err != nil {
			return fmt.Errorf("invalid token format: %w", err)
		}

		// Validate expiry.
		if time.Now().After(token.Expires) {
			return fmt.Errorf("invite token expired at %s", token.Expires.Format(time.DateTime))
		}

		// Detect our local IP.
		localIP, err := detectLocalIP()
		if err != nil {
			return fmt.Errorf("detecting local IP: %w", err)
		}

		// Load local store to read self info.
		st := store.New(cfg.Server.DataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}
		self := st.Self()

		// POST to the originator's /api/join endpoint.
		joinReq := struct {
			Hostname string            `json:"hostname"`
			Address  string            `json:"address"`
			Port     int               `json:"port"`
			Version  string            `json:"version"`
			Role     string            `json:"role"`
			Tags     map[string]string `json:"tags,omitempty"`
		}{
			Hostname: cfg.Server.Hostname,
			Address:  localIP,
			Port:     cfg.Server.Port,
			Version:  version.Version,
			Role:     self.Role,
			Tags:     self.Tags,
		}

		body, err := json.Marshal(joinReq)
		if err != nil {
			return fmt.Errorf("marshalling join request: %w", err)
		}

		url := fmt.Sprintf("http://%s:%d/api/join", token.Address, token.Port)
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token.Token)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("contacting originator at %s:%d: %w", token.Address, token.Port, err)
		}
		defer resp.Body.Close() //nolint:errcheck // read-only body

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("join rejected by originator (status %d)", resp.StatusCode)
		}

		// Parse response to get peer group info and originator peer info.
		var joinResp struct {
			PeerGroup peer.PeerGroup    `json:"peer_group"`
			PSK       string            `json:"psk"`
			Hostname  string            `json:"hostname"`
			Address   string            `json:"address"`
			Port      int               `json:"port"`
			Version   string            `json:"version"`
			Role      string            `json:"role"`
			Tags      map[string]string `json:"tags,omitempty"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&joinResp); err != nil {
			return fmt.Errorf("decoding join response: %w", err)
		}

		// Save peer group info (name + PSK received from server) to local store.
		st.SetPeerGroup(peer.PeerGroup{
			Name:   joinResp.PeerGroup.Name,
			Secret: joinResp.PSK,
		})

		// Save originator as a peer with source="invite".
		originator := peer.Peer{
			Hostname: joinResp.Hostname,
			Address:  joinResp.Address,
			Port:     joinResp.Port,
			Version:  joinResp.Version,
			Role:     joinResp.Role,
			Source:   peer.SourceInvite,
			Tags:     joinResp.Tags,
			LastSeen: time.Now(),
			Online:   true,
		}
		st.Upsert(originator)

		if err := st.Save(); err != nil {
			return fmt.Errorf("saving store: %w", err)
		}

		fmt.Printf("Joined peer group %q via %s.\n", joinResp.PeerGroup.Name, joinResp.Hostname)
		fmt.Printf("Peer group secret saved to %s/peers.yaml\n", cfg.Server.DataDir)
		return nil
	},
}
