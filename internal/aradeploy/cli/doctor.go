package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/certs"
	"github.com/jdillenberger/arastack/internal/aradeploy/doctor"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	pkgdoctor "github.com/jdillenberger/arastack/pkg/doctor"
)

func init() {
	doctorCmd.Flags().Bool("fix", false, "Auto-fix failing checks")
	doctorCmd.Flags().String("check", "", "Comma-separated check categories: domains,labels,certs,containers,envvars")
	doctorCmd.Flags().String("app", "", "Check a single app only")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system dependencies and deployment consistency",
	Long:  "Checks system dependencies, routing domains, Traefik labels, certificates, container health, and environment variables.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")
		checkFlag, _ := cmd.Flags().GetString("check")
		appFlag, _ := cmd.Flags().GetString("app")

		// Phase 1: System dependency checks.
		fmt.Println("System Dependencies")
		depResults := doctor.CheckAll()

		var depFailed []pkgdoctor.CheckResult
		for _, r := range depResults {
			if r.Installed {
				fmt.Printf("  %s %-30s %s\n", cliutil.StatusOK("✓"), r.Name, r.Version)
			} else {
				fmt.Printf("  %s %-30s %s\n", cliutil.StatusFail("✗"), r.Name, "missing")
				depFailed = append(depFailed, r)
			}
		}

		if fix && len(depFailed) > 0 {
			for _, r := range depFailed {
				fmt.Printf("  Fixing %s...\n", r.Name)
				if err := doctor.Fix(r); err != nil {
					fmt.Printf("    Failed: %v\n", err)
				} else {
					fmt.Printf("    Fixed.\n")
				}
			}
		}

		// Phase 2: Deployment consistency checks.
		fmt.Println()
		fmt.Println("Deployment Checks")

		mgr, err := newManager()
		if err != nil {
			fmt.Printf("  %s %-30s %s\n", cliutil.StatusFail("✗"), "manager", err.Error())
			if len(depFailed) > 0 {
				return fmt.Errorf("some checks failed")
			}
			return nil
		}

		cm := certs.NewManager(cfg.DataPath("traefik"))
		dc := doctor.NewDeploymentChecker(mgr, cm)

		// Parse category filter.
		var categories []doctor.Category
		if checkFlag != "" {
			for _, c := range strings.Split(checkFlag, ",") {
				categories = append(categories, doctor.Category(strings.TrimSpace(c)))
			}
		}

		// Parse app filter.
		var apps []string
		if appFlag != "" {
			apps = []string{appFlag}
		}

		results, err := dc.CheckAll(apps, categories)
		if err != nil {
			return err
		}

		if jsonOutput {
			return outputJSON(results)
		}

		var fixable []doctor.DeployCheckResult
		var hasIssues bool
		currentApp := ""
		for _, r := range results {
			if r.App != currentApp {
				if r.App != "" {
					fmt.Printf("\n  %s\n", r.App)
				}
				currentApp = r.App
			}

			name := r.Name
			if r.App == "" {
				// Global check (e.g. certs), no app prefix needed.
				name = r.Name
			}

			switch r.Severity {
			case doctor.SeverityOK:
				fmt.Printf("    %s %-24s %s\n", cliutil.StatusOK("✓"), name, r.Detail)
			case doctor.SeverityWarn:
				fmt.Printf("    %s %-24s %s\n", cliutil.StatusWarn("⚠"), name, r.Detail)
				hasIssues = true
				if r.Fixable {
					fixable = append(fixable, r)
				}
			case doctor.SeverityFail:
				fmt.Printf("    %s %-24s %s\n", cliutil.StatusFail("✗"), name, r.Detail)
				hasIssues = true
				if r.Fixable {
					fixable = append(fixable, r)
				}
			}
		}

		if fix && len(fixable) > 0 {
			fmt.Println()
			for _, r := range fixable {
				label := r.Name
				if r.App != "" {
					label = r.App + "/" + r.Name
				}
				fmt.Printf("  Fixing %s...\n", label)
				if err := r.FixFunc(); err != nil {
					fmt.Printf("    Failed: %v\n", err)
				} else {
					fmt.Printf("    Fixed.\n")
				}
			}
		} else if len(fixable) > 0 {
			fmt.Printf("\n%d fixable issue(s). Run with --fix to resolve.\n", len(fixable))
		}

		if !hasIssues && len(depFailed) == 0 {
			fmt.Println("\nAll checks passed.")
		} else if hasIssues || len(depFailed) > 0 {
			return fmt.Errorf("some checks failed")
		}

		return nil
	},
}
