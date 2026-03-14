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

	"github.com/jdillenberger/arastack/internal/aradeploy/code"
	"github.com/jdillenberger/arastack/internal/aradeploy/compose"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/internal/aradeploy/wizard"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	"github.com/jdillenberger/arastack/pkg/executil"
	"github.com/jdillenberger/arastack/pkg/portcheck"
)

func newManager() (*deploy.Manager, error) {
	runner := &executil.Runner{}
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
	rootCmd.AddCommand(inspectCmd)
	deployCmd.Flags().StringP("values", "f", "", "YAML file with template values")
	deployCmd.Flags().StringSlice("set", nil, "Set values (key=value)")
	deployCmd.Flags().StringSlice("code", nil, "Mount code source (slot[/name]=source[#branch])")
	deployCmd.Flags().Bool("dry-run", false, "Show rendered files without deploying")
	deployCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	deployCmd.Flags().Bool("quick", false, "Accept all defaults and auto-generate secrets")
	deployCmd.ValidArgsFunction = completeTemplateNames

	removeCmd.Flags().Bool("purge-data", false, "Also remove app data volumes")
	removeCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
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

	inspectCmd.ValidArgsFunction = completeDeployedApps
}

var deployCmd = &cobra.Command{
	Use:   "deploy <app>",
	Short: "Deploy an app from a template",
	Example: `  aradeploy deploy nextcloud
  aradeploy deploy nextcloud -f values.yaml
  aradeploy deploy nextcloud --set domain=cloud.example.com --set admin_password=secret
  aradeploy deploy nextcloud --quick --yes
  aradeploy deploy nextcloud --dry-run
  aradeploy deploy wordpress --code themes/my-theme=./path/to/theme
  aradeploy deploy vite-app --code src=https://github.com/user/app.git#main`,
	Args: cobra.ExactArgs(1),
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
			data, err := os.ReadFile(valuesFile) // #nosec G304 -- user-specified values file
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

		codeFlags, _ := cmd.Flags().GetStringSlice("code")
		codeMap := make(map[string]string)
		for _, kv := range codeFlags {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid --code value: %q (expected slot[/name]=source[#branch])", kv)
			}
			codeMap[parts[0]] = parts[1]
		}

		// Resolve forward auth before deploy
		meta, hasMeta := mgr.Registry().Get(appName)
		autheliaDeployed := mgr.IsAutheliaDeployed()

		if quick {
			if hasMeta && autheliaDeployed {
				authMode := meta.Routing.AuthMode()
				if authMode == "required" {
					values["forward_auth"] = "true"
				}
			}
			return mgr.Deploy(appName, deploy.DeployOptions{
				Values:  values,
				Code:    codeMap,
				DryRun:  dryRun,
				Confirm: true,
			})
		}

		if !dryRun {
			if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				if hasMeta {
					if len(values) == 0 && len(meta.Values) > 0 {
						usedPorts, _ := portcheck.UsedPorts(mgr.Config().AppsDir)
						wizardValues, err := wizard.RunDeployWizard(meta, usedPorts)
						if err != nil {
							return err
						}
						values = wizardValues
					}
					if len(codeMap) == 0 && meta.Code != nil {
						wizardCode, err := wizard.RunCodeWizard(meta)
						if err != nil {
							return err
						}
						for k, v := range wizardCode {
							codeMap[k] = v
						}
					}
				}
			}
		}

		// Forward auth logic
		if hasMeta {
			authMode := meta.Routing.AuthMode()
			switch {
			case authMode == "required" && autheliaDeployed:
				values["forward_auth"] = "true"
			case authMode == "required" && !autheliaDeployed:
				fmt.Println("Warning: this app requires forward auth but authelia is not deployed. Deploy authelia first to enable authentication.")
			case authMode == "optional" && autheliaDeployed && !dryRun:
				if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
					enable, err := wizard.AskForwardAuth(appName)
					if err != nil {
						return err
					}
					if enable {
						values["forward_auth"] = "true"
					}
				}
			}
		}

		yes, _ := cmd.Flags().GetBool("yes")
		return mgr.Deploy(appName, deploy.DeployOptions{
			Values:  values,
			Code:    codeMap,
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
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			msg := fmt.Sprintf("Remove app %s (data will be kept)?", args[0])
			if purgeData {
				msg = fmt.Sprintf("Remove app %s (including all data)?", args[0])
			}
			if !cliutil.AskConfirmation(msg) {
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
		runner := &executil.Runner{}
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
			slog.Warn("'list --all' is deprecated, use 'aradeploy templates list' instead")
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
			fmt.Println("No apps deployed. Use 'aradeploy templates list' to see available templates.")
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

var inspectCmd = &cobra.Command{
	Use:   "inspect <app>",
	Short: "Show details of a deployed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		appName := args[0]
		info, err := mgr.GetDeployedInfo(appName)
		if err != nil {
			return fmt.Errorf("app %s is not deployed: %w", appName, err)
		}

		meta, hasMeta := mgr.Registry().Get(appName)

		// Build set of secret value names for masking.
		secretNames := make(map[string]bool)
		if hasMeta {
			for _, v := range meta.Values {
				if v.Secret {
					secretNames[v.Name] = true
				}
			}
		}

		if jsonOutput {
			type showOutput struct {
				Name       string                   `json:"name"`
				Version    string                   `json:"version"`
				DeployedAt string                   `json:"deployed_at"`
				DataDir    string                   `json:"data_dir"`
				Values     map[string]string        `json:"values"`
				Ports      []template.PortMapping   `json:"ports,omitempty"`
				Volumes    []template.VolumeMapping `json:"volumes,omitempty"`
				URLs       []string                 `json:"urls,omitempty"`
				Code       []code.Source            `json:"code,omitempty"`
			}
			maskedValues := make(map[string]string)
			for k, v := range info.Values {
				if secretNames[k] {
					maskedValues[k] = "***"
				} else {
					maskedValues[k] = v
				}
			}
			out := showOutput{
				Name:       info.Name,
				Version:    info.Version,
				DeployedAt: info.DeployedAt.Format("2006-01-02 15:04:05"),
				DataDir:    cfg.DataPath(appName),
				Values:     maskedValues,
			}
			if hasMeta {
				out.Ports = meta.Ports
				out.Volumes = meta.Volumes
			}
			if info.Routing != nil && info.Routing.Enabled {
				out.URLs = info.Routing.Domains
			}
			out.Code = info.Code
			return outputJSON(out)
		}

		fmt.Printf("App:         %s\n", info.Name)
		fmt.Printf("Version:     %s\n", info.Version)
		fmt.Printf("Deployed at: %s\n", info.DeployedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Data dir:    %s\n", cfg.DataPath(appName))

		if info.Routing != nil && info.Routing.Enabled && len(info.Routing.Domains) > 0 {
			fmt.Printf("\nURLs:\n")
			for _, d := range info.Routing.Domains {
				fmt.Printf("  https://%s\n", d)
			}
		}

		if hasMeta && len(meta.Ports) > 0 {
			fmt.Println("\nPorts:")
			for _, p := range meta.Ports {
				fmt.Printf("  %d:%d/%s  %s\n", p.Host, p.Container, p.Protocol, p.Description)
			}
		}

		if hasMeta && len(meta.Volumes) > 0 {
			fmt.Println("\nVolumes:")
			for _, v := range meta.Volumes {
				fmt.Printf("  %-15s %s  (%s)\n", v.Name, v.Container, v.Description)
			}
		}

		if len(info.Code) > 0 {
			fmt.Println("\nCode sources:")
			for _, cs := range info.Code {
				label := cs.Slot
				if cs.Name != "" {
					label += "/" + cs.Name
				}
				extra := cs.Type
				if cs.Branch != "" {
					extra += ", branch: " + cs.Branch
				}
				fmt.Printf("  %-25s %s  (%s)\n", label, cs.Source, extra)
			}
		}

		if len(info.Values) > 0 {
			fmt.Println("\nValues:")
			for k, v := range info.Values {
				if secretNames[k] {
					fmt.Printf("  %-20s ***\n", k)
				} else {
					fmt.Printf("  %-20s %s\n", k, v)
				}
			}
		}

		return nil
	},
}

const eventSpoolPath = "/var/lib/aradeploy/pending-events.json"

// pushUpdateFailedEvent sends an update-failed event to araalert.
// If delivery fails after retries, the event is spooled to disk for later retry.
func pushUpdateFailedEvent(appName string, updateErr error) {
	if cfg.Araalert.URL == "" {
		return
	}
	event := clients.Event{
		Type:     "update-failed",
		App:      appName,
		Message:  fmt.Sprintf("Update failed for %s: %v", appName, updateErr),
		Severity: "error",
	}
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	if err := ac.PushEvent(context.Background(), event); err != nil {
		slog.Warn("Failed to push alert event, spooling for retry", "app", appName, "error", err)
		spool := clients.NewEventSpool(eventSpoolPath)
		if spoolErr := spool.Add(event); spoolErr != nil {
			slog.Error("Failed to spool alert event", "app", appName, "error", spoolErr)
		}
	}
}

// flushSpooledEvents retries sending any previously spooled alert events.
func flushSpooledEvents() {
	if cfg.Araalert.URL == "" {
		return
	}
	spool := clients.NewEventSpool(eventSpoolPath)
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	spool.Flush(ac)
}
