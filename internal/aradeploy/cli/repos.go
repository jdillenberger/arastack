package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(reposCmd)
	reposCmd.AddCommand(reposAddCmd)
	reposCmd.AddCommand(reposRemoveCmd)
	reposCmd.AddCommand(reposListCmd)
	reposCmd.AddCommand(reposUpdateCmd)

	reposAddCmd.Flags().String("name", "", "Repository name (default: derived from URL)")
	reposAddCmd.Flags().String("ref", "", "Git branch or tag to track")
}

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage template repositories",
}

var reposAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a git template repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{}
		mgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)

		name, _ := cmd.Flags().GetString("name")
		ref, _ := cmd.Flags().GetString("ref")

		r, err := mgr.Add(args[0], name, ref)
		if err != nil {
			return err
		}

		fmt.Printf("Added repo %s from %s\n", r.Name, r.URL)
		return nil
	},
}

var reposRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a template repository",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{}
		mgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)

		if err := mgr.Remove(args[0]); err != nil {
			return err
		}
		fmt.Printf("Removed repo %s\n", args[0])
		return nil
	},
}

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List template repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{}
		mgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)

		repos, err := mgr.List()
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			fmt.Println("No template repositories configured.")
			return nil
		}

		if jsonOutput {
			return outputJSON(repos)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tURL\tREF\tUPDATED")
		for _, r := range repos {
			updated := "-"
			if !r.UpdatedAt.IsZero() {
				updated = r.UpdatedAt.Format("2006-01-02 15:04")
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.URL, r.Ref, updated)
		}
		_ = w.Flush()
		return nil
	},
}

var reposUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Pull latest from repos",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{}
		mgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)

		if len(args) > 0 {
			if err := mgr.Update(args[0]); err != nil {
				return err
			}
			fmt.Printf("Updated repo %s\n", args[0])
		} else {
			if err := mgr.UpdateAll(); err != nil {
				return err
			}
			fmt.Println("All repos updated.")
		}
		return nil
	},
}
