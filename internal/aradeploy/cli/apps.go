package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/internal/aradeploy/compose"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/internal/aradeploy/wizard"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func newManager() (*deploy.Manager, error) {
	runner := &executil.Runner{Verbose: verbose}
	repoMgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)
	if err := repoMgr.EnsureDefaults(); err != nil {
		return nil, fmt.Errorf("ensuring default repos: %w", err)
	}
	repoDirs, _ := repoMgr.TemplateDirs()
	tmplFS := template.BuildTemplateFS(repoDirs, cfg.TemplatesDir)
	return deploy.NewManager(cfg.ToManagerConfig(), runner, tmplFS)
}

func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr, err := newManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return mgr.Registry().List(), cobra.ShellCompDirectiveNoFileComp
}

func completeDeployedApps(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	mgr, err := newManager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	deployed, err := mgr.ListDeployed()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return deployed, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	// Top-level shortcuts
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(updateCmd)

	deployCmd.Flags().StringP("values", "f", "", "YAML file with template values")
	deployCmd.Flags().StringSlice("set", nil, "Set values (key=value)")
	deployCmd.Flags().Bool("dry-run", false, "Show rendered files without deploying")
	deployCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	deployCmd.Flags().Bool("quick", false, "Accept all defaults and auto-generate secrets")
	deployCmd.ValidArgsFunction = completeTemplateNames

	removeCmd.Flags().Bool("purge-data", false, "Also remove app data volumes")
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	removeCmd.ValidArgsFunction = completeDeployedApps

	startCmd.ValidArgsFunction = completeDeployedApps
	stopCmd.ValidArgsFunction = completeDeployedApps
	restartCmd.ValidArgsFunction = completeDeployedApps

	statusCmd.ValidArgsFunction = completeDeployedApps

	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().IntP("lines", "n", 100, "Number of lines to show")
	logsCmd.ValidArgsFunction = completeDeployedApps

	listCmd.Flags().Bool("all", false, "Show all available templates (not just deployed)")
	listCmd.Flags().String("filter", "", "Filter apps by name or description substring")
	listCmd.Flags().String("category", "", "Filter apps by category")

	updateCmd.Flags().Bool("all", false, "Update all deployed apps")
	updateCmd.ValidArgsFunction = completeDeployedApps

	infoCmd.ValidArgsFunction = completeTemplateNames
}

var deployCmd = &cobra.Command{
	Use:   "deploy <app>",
	Short: "Deploy an app from a template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		appName := args[0]
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		quick, _ := cmd.Flags().GetBool("quick")

		values := make(map[string]string)
		valuesFile, _ := cmd.Flags().GetString("values")
		if valuesFile != "" {
			data, err := os.ReadFile(valuesFile)
			if err != nil {
				return fmt.Errorf("reading values file: %w", err)
			}
			if err := yaml.Unmarshal(data, &values); err != nil {
				return fmt.Errorf("parsing values file: %w", err)
			}
		}

		setValues, _ := cmd.Flags().GetStringSlice("set")
		for _, kv := range setValues {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid --set value: %q (expected key=value)", kv)
			}
			values[parts[0]] = parts[1]
		}

		if quick {
			return mgr.Deploy(appName, deploy.DeployOptions{
				Values:  values,
				DryRun:  dryRun,
				Confirm: true,
			})
		}

		if len(values) == 0 && !dryRun {
			if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				meta, ok := mgr.Registry().Get(appName)
				if ok && len(meta.Values) > 0 {
					wizardValues, err := wizard.RunDeployWizard(meta)
					if err != nil {
						return err
					}
					values = wizardValues
				}
			}
		}

		yes, _ := cmd.Flags().GetBool("yes")
		return mgr.Deploy(appName, deploy.DeployOptions{
			Values:  values,
			DryRun:  dryRun,
			Confirm: yes,
		})
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <app>",
	Short: "Remove a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}
		purgeData, _ := cmd.Flags().GetBool("purge-data")
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			msg := fmt.Sprintf("Remove app %s (data will be kept)?", args[0])
			if purgeData {
				msg = fmt.Sprintf("Remove app %s (including all data)?", args[0])
			}
			if !deploy.AskConfirmation(msg) {
				fmt.Println("Aborted.")
				return nil
			}
		}
		return mgr.Remove(args[0], !purgeData)
	},
}

var startCmd = &cobra.Command{
	Use:   "start <app>",
	Short: "Start a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}
		return mgr.Start(args[0])
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop <app>",
	Short: "Stop a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}
		return mgr.Stop(args[0])
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart <app>",
	Short: "Restart a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}
		return mgr.Restart(args[0])
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [app]",
	Short: "Show container status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			deployed, err := mgr.ListDeployed()
			if err != nil {
				return err
			}

			if jsonOutput {
				type appStatus struct {
					Name   string `json:"name"`
					Status string `json:"status"`
					Error  string `json:"error,omitempty"`
				}
				var statuses []appStatus
				for _, name := range deployed {
					status, err := mgr.Status(name)
					if err != nil {
						statuses = append(statuses, appStatus{Name: name, Error: err.Error()})
					} else {
						statuses = append(statuses, appStatus{Name: name, Status: status})
					}
				}
				return outputJSON(statuses)
			}

			for _, name := range deployed {
				fmt.Printf("=== %s ===\n", name)
				status, err := mgr.Status(name)
				if err != nil {
					fmt.Printf("  Error: %v\n", err)
					continue
				}
				fmt.Println(status)
			}
			return nil
		}

		status, err := mgr.Status(args[0])
		if err != nil {
			return err
		}

		if jsonOutput {
			return outputJSON(map[string]string{"name": args[0], "status": status})
		}

		fmt.Print(status)
		return nil
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "Show app logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{Verbose: verbose}
		c := compose.New(runner, cfg.Docker.ComposeCommand)
		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")
		return c.Logs(cfg.AppDir(args[0]), os.Stdout, follow, lines)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List apps",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		showAll, _ := cmd.Flags().GetBool("all")
		filter, _ := cmd.Flags().GetString("filter")
		category, _ := cmd.Flags().GetString("category")
		filterLower := strings.ToLower(filter)

		if showAll {
			type appListEntry struct {
				Name        string `json:"name"`
				Category    string `json:"category"`
				Description string `json:"description"`
				Status      string `json:"status"`
			}

			deployed, _ := mgr.ListDeployed()
			deployedSet := make(map[string]bool)
			for _, d := range deployed {
				deployedSet[d] = true
			}

			var entries []appListEntry
			for _, meta := range mgr.Registry().All() {
				if filter != "" {
					nameLower := strings.ToLower(meta.Name)
					descLower := strings.ToLower(meta.Description)
					if !strings.Contains(nameLower, filterLower) && !strings.Contains(descLower, filterLower) {
						continue
					}
				}
				if category != "" && !strings.EqualFold(meta.Category, category) {
					continue
				}

				status := "available"
				if deployedSet[meta.Name] {
					status = "deployed"
				}
				entries = append(entries, appListEntry{
					Name: meta.Name, Category: meta.Category,
					Description: meta.Description, Status: status,
				})
			}

			if jsonOutput {
				return outputJSON(entries)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tCATEGORY\tDESCRIPTION\tSTATUS")
			for _, e := range entries {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Name, e.Category, e.Description, e.Status)
			}
			_ = w.Flush()
			return nil
		}

		deployed, err := mgr.ListDeployed()
		if err != nil {
			return err
		}

		if len(deployed) == 0 {
			if jsonOutput {
				return outputJSON([]struct{}{})
			}
			fmt.Println("No apps deployed. Use 'aradeploy list --all' to see available templates.")
			return nil
		}

		type deployedEntry struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			DeployedAt string `json:"deployed_at"`
		}

		var entries []deployedEntry
		for _, name := range deployed {
			if filter != "" && !strings.Contains(strings.ToLower(name), filterLower) {
				continue
			}
			info, err := mgr.GetDeployedInfo(name)
			if err != nil {
				entries = append(entries, deployedEntry{Name: name, DeployedAt: "error reading info"})
				continue
			}
			if category != "" {
				if meta, ok := mgr.Registry().Get(name); ok {
					if !strings.EqualFold(meta.Category, category) {
						continue
					}
				}
			}
			entries = append(entries, deployedEntry{
				Name:       info.Name,
				Version:    info.Version,
				DeployedAt: info.DeployedAt.Format("2006-01-02 15:04"),
			})
		}

		if jsonOutput {
			return outputJSON(entries)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tVERSION\tDEPLOYED")
		for _, e := range entries {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Version, e.DeployedAt)
		}
		_ = w.Flush()
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <app>",
	Short: "Show app template details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		meta, ok := mgr.Registry().Get(args[0])
		if !ok {
			return fmt.Errorf("unknown app template: %s", args[0])
		}

		if jsonOutput {
			return outputJSON(meta)
		}

		fmt.Printf("App: %s\n", meta.Name)
		fmt.Printf("Description: %s\n", meta.Description)
		fmt.Printf("Category: %s\n", meta.Category)
		fmt.Printf("Version: %s\n", meta.Version)

		if len(meta.Ports) > 0 {
			fmt.Println("\nPorts:")
			for _, p := range meta.Ports {
				fmt.Printf("  %d:%d/%s  %s\n", p.Host, p.Container, p.Protocol, p.Description)
			}
		}

		if len(meta.Volumes) > 0 {
			fmt.Println("\nVolumes:")
			for _, v := range meta.Volumes {
				fmt.Printf("  %-15s %s  (%s)\n", v.Name, v.Container, v.Description)
			}
		}

		if len(meta.Values) > 0 {
			fmt.Println("\nValues:")
			for _, v := range meta.Values {
				req := ""
				if v.Required {
					req = " [required]"
				}
				def := ""
				if v.Default != "" {
					def = fmt.Sprintf(" (default: %s)", v.Default)
				}
				secret := ""
				if v.Secret {
					secret = " [secret]"
				}
				autoGen := ""
				if v.AutoGen != "" {
					autoGen = fmt.Sprintf(" [auto: %s]", v.AutoGen)
				}
				fmt.Printf("  %-20s %s%s%s%s%s\n", v.Name, v.Description, def, req, secret, autoGen)
			}
		}

		if len(meta.Dependencies) > 0 {
			fmt.Printf("\nDependencies: %s\n", strings.Join(meta.Dependencies, ", "))
		}

		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update [app]",
	Short: "Pull latest images and recreate containers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		updateAll, _ := cmd.Flags().GetBool("all")

		if updateAll {
			deployed, err := mgr.ListDeployed()
			if err != nil {
				return err
			}
			if len(deployed) == 0 {
				fmt.Println("No apps deployed.")
				return nil
			}
			for _, appName := range deployed {
				fmt.Printf("Updating %s...\n", appName)
				if err := mgr.Update(appName); err != nil {
					fmt.Printf("  Update failed for %s: %v\n", appName, err)
					pushUpdateFailedEvent(appName, err)
					continue
				}
				fmt.Printf("  %s updated.\n", appName)
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("app name required (or use --all)")
		}

		return mgr.Update(args[0])
	},
}

// pushUpdateFailedEvent sends an update-failed event to araalert (best-effort).
func pushUpdateFailedEvent(appName string, updateErr error) {
	if cfg.Araalert.URL == "" {
		return
	}
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	err := ac.PushEvent(context.Background(), clients.Event{
		Type:     "update-failed",
		App:      appName,
		Message:  fmt.Sprintf("Update failed for %s: %v", appName, updateErr),
		Severity: "error",
	})
	if err != nil {
		slog.Warn("Failed to push alert event", "app", appName, "error", err)
	}
}
