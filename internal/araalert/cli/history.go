package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/araalert/alert"
	"github.com/jdillenberger/arastack/internal/araalert/config"
)

func init() {
	historyCmd.Flags().IntP("count", "n", 20, "Number of recent alerts to show")
	rootCmd.AddCommand(historyCmd)
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent alert history",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}

		count, _ := cmd.Flags().GetInt("count")

		store := alert.NewStore(cfg.DataDir)
		history, err := store.LoadHistory()
		if err != nil {
			return err
		}

		if len(history) == 0 {
			fmt.Println("No alert history.")
			return nil
		}

		// Show most recent entries.
		start := 0
		if len(history) > count {
			start = len(history) - count
		}
		recent := history[start:]

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "TIME\tSEVERITY\tTYPE\tMESSAGE")
		for _, a := range recent {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				a.Timestamp.Format(time.DateTime), a.Severity, a.Type, a.Message)
		}
		_ = w.Flush()
		return nil
	},
}
