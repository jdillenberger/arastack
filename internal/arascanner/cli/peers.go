package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/mdns"
	"github.com/jdillenberger/arastack/internal/arascanner/peer"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
)

type peersAPIResponse struct {
	Fleet peer.Fleet  `json:"fleet"`
	Self  peer.Peer   `json:"self"`
	Peers []peer.Peer `json:"peers"`
}

func init() {
	peersDiscoverCmd.Flags().DurationP("timeout", "t", 5*time.Second, "mDNS discovery timeout")
	peersCmd.AddCommand(peersDiscoverCmd)
	rootCmd.AddCommand(peersCmd)
}

// peersCmd lists known peers by querying the local API.
var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List known peers from the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load store to get PSK for authenticated API call.
		st := store.New(dataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		url := fmt.Sprintf("http://localhost:%d/api/peers", port)
		req, err := http.NewRequest("GET", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		fleet := st.Fleet()
		if fleet.Secret != "" {
			req.Header.Set("Authorization", "Bearer "+fleet.Secret)
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("could not reach daemon at localhost:%d — is arascanner running? (%w)", port, err)
		}
		defer resp.Body.Close() //nolint:errcheck // read-only body

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API returned status %d", resp.StatusCode)
		}

		var apiResp peersAPIResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		printPeerTable(apiResp.Peers)
		return nil
	},
}

// peersDiscoverCmd does a one-shot mDNS scan.
var peersDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Run one-shot mDNS discovery",
	RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetDuration("timeout")

		peers, err := mdns.Discover(timeout)
		if err != nil {
			return fmt.Errorf("mDNS discovery failed: %w", err)
		}

		if len(peers) == 0 {
			fmt.Println("No peers discovered.")
			return nil
		}

		printPeerTable(peers)
		return nil
	},
}

func printPeerTable(peers []peer.Peer) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "HOSTNAME\tROLE\tADDRESS\tPORT\tSOURCE\tTAGS\tLAST SEEN\tSTATUS")

	for _, p := range peers {
		status := "offline"
		if p.Online {
			status = "online"
		}

		tags := formatTags(p.Tags)

		lastSeen := "-"
		if !p.LastSeen.IsZero() {
			lastSeen = p.LastSeen.Format(time.DateTime)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			p.Hostname, p.Role, p.Address, p.Port, p.Source, tags, lastSeen, status)
	}

	_ = w.Flush()
}

func formatTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(tags))
	for k, v := range tags {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}
