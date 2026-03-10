package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/store"
)

func init() {
	tagsCmd.AddCommand(tagsSetCmd)
	tagsCmd.AddCommand(tagsRemoveCmd)
	tagsCmd.AddCommand(tagsListCmd)
	rootCmd.AddCommand(tagsCmd)
}

// tagsCmd is the parent command for tag management.
var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage local peer tags",
}

// tagsSetCmd sets one or more tags on the local peer.
var tagsSetCmd = &cobra.Command{
	Use:   "set key=value [key2=value2 ...]",
	Short: "Set tags on the local peer",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st := store.New(cfg.Server.DataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		self := st.Self()
		tags := self.Tags
		if tags == nil {
			tags = make(map[string]string)
		}

		for _, arg := range args {
			key, value, ok := strings.Cut(arg, "=")
			if !ok || key == "" {
				return fmt.Errorf("invalid tag format %q, expected key=value", arg)
			}
			tags[key] = value
		}

		st.SetSelfTags(tags)

		if err := st.Save(); err != nil {
			return fmt.Errorf("saving store: %w", err)
		}

		fmt.Println("Tags updated.")
		printTagTable(tags)
		return nil
	},
}

// tagsRemoveCmd removes one or more tags from the local peer.
var tagsRemoveCmd = &cobra.Command{
	Use:   "remove key [key2 ...]",
	Short: "Remove tags from the local peer",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st := store.New(cfg.Server.DataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		self := st.Self()
		tags := self.Tags
		if tags == nil {
			tags = make(map[string]string)
		}

		for _, key := range args {
			delete(tags, key)
		}

		st.SetSelfTags(tags)

		if err := st.Save(); err != nil {
			return fmt.Errorf("saving store: %w", err)
		}

		fmt.Println("Tags updated.")
		printTagTable(tags)
		return nil
	},
}

// tagsListCmd lists all tags on the local peer.
var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags on the local peer",
	RunE: func(cmd *cobra.Command, args []string) error {
		st := store.New(cfg.Server.DataDir)
		if err := st.Load(); err != nil {
			return fmt.Errorf("loading store: %w", err)
		}

		self := st.Self()
		if len(self.Tags) == 0 {
			fmt.Println("No tags set.")
			return nil
		}

		printTagTable(self.Tags)
		return nil
	},
}

func printTagTable(tags map[string]string) {
	if len(tags) == 0 {
		return
	}

	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tVALUE")
	for _, k := range keys {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", k, tags[k])
	}
	_ = w.Flush()
}
