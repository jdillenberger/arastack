package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/image"
	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(pinCmd)
	pinCmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	pinCmd.Flags().Bool("update", false, "Apply pin changes to templates")
	pinCmd.ValidArgsFunction = completeTemplateNames
}

var pinCmd = &cobra.Command{
	Use:   "pin [app]",
	Short: "Resolve floating image tags to pinned versions",
	Long:  "Scan templates for floating tags (latest, release) and resolve them via registry API to the highest semver tag with the same digest.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{Verbose: verbose}
		repoMgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)
		repoDirs, _ := repoMgr.TemplateDirs()
		tmplFS := template.BuildTemplateFS(repoDirs, cfg.TemplatesDir)

		entries, err := image.ScanFloatingTags(tmplFS)
		if err != nil {
			return fmt.Errorf("scanning templates: %w", err)
		}

		if len(args) > 0 {
			appName := args[0]
			var filtered []image.FloatingTagEntry
			for _, e := range entries {
				if e.AppName == appName {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}

		if len(entries) == 0 {
			fmt.Println("No floating tags found.")
			return nil
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		update, _ := cmd.Flags().GetBool("update")

		resolver := image.NewResolver()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "APP\tIMAGE\tFLOATING\tPINNED\tSTATUS")

		for _, entry := range entries {
			result, err := resolver.ResolveFloatingTag(entry.Ref)
			if err != nil {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t-\terror: %v\n", entry.AppName, entry.Image, entry.Ref.Tag, err)
				continue
			}

			if result.PinnedTag == "" {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t-\tno semver tag found\n", entry.AppName, entry.Image, entry.Ref.Tag)
				continue
			}

			status := "found"
			if !update && !dryRun {
				status = "found (use --update to apply)"
			}
			if dryRun {
				status = "dry-run"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", entry.AppName, entry.Image, entry.Ref.Tag, result.PinnedTag, status)
		}
		_ = w.Flush()
		return nil
	},
}
