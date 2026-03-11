package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/code"
	"github.com/jdillenberger/arastack/internal/aradeploy/compose"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(codeCmd)
	codeCmd.AddCommand(codeAddCmd)
	codeCmd.AddCommand(codeRemoveCmd)
	codeCmd.AddCommand(codeListCmd)
	codeCmd.AddCommand(codeUpdateCmd)

	codeAddCmd.Flags().StringP("branch", "b", "", "Git branch to checkout")
	codeAddCmd.ValidArgsFunction = completeDeployedApps
	codeRemoveCmd.ValidArgsFunction = completeDeployedApps
	codeListCmd.ValidArgsFunction = completeDeployedApps
	codeUpdateCmd.ValidArgsFunction = completeDeployedApps
}

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Manage code sources for deployed apps",
}

var codeAddCmd = &cobra.Command{
	Use:   "add <app> <slot> [name] <source>",
	Short: "Add a code source to a deployed app",
	Example: `  aradeploy code add wordpress themes my-theme ./path/to/theme
  aradeploy code add wordpress plugins seo https://github.com/user/seo.git
  aradeploy code add vite-app src ./my-project --branch main`,
	Args: cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		appName := args[0]
		slotName := args[1]

		var name, source string
		if len(args) == 4 {
			name = args[2]
			source = args[3]
		} else {
			source = args[2]
		}

		branch, _ := cmd.Flags().GetString("branch")

		info, err := mgr.GetDeployedInfo(appName)
		if err != nil {
			return fmt.Errorf("app %s is not deployed: %w", appName, err)
		}

		meta, ok := mgr.Registry().Get(appName)
		if !ok {
			return fmt.Errorf("unknown app template: %s", appName)
		}
		if meta.Code == nil {
			return fmt.Errorf("template %s does not define any code slots", appName)
		}

		var slot *template.CodeSlot
		for i := range meta.Code.Slots {
			if meta.Code.Slots[i].Name == slotName {
				slot = &meta.Code.Slots[i]
				break
			}
		}
		if slot == nil {
			return fmt.Errorf("unknown code slot %q", slotName)
		}

		if slot.Multiple && name == "" {
			return fmt.Errorf("slot %q supports multiple items — provide a name: aradeploy code add %s %s <name> <source>", slotName, appName, slotName)
		}
		if !slot.Multiple && name != "" {
			return fmt.Errorf("slot %q does not support multiple items — omit the name", slotName)
		}
		if slot.InjectMode() == "build" && !meta.RequiresBuild {
			return fmt.Errorf("slot %q uses build injection but template %s does not have requires_build enabled", slotName, appName)
		}

		// Check for duplicate
		for _, existing := range info.Code {
			if existing.Slot == slotName && existing.Name == name {
				label := slotName
				if name != "" {
					label += "/" + name
				}
				return fmt.Errorf("code source %s already exists in %s — remove it first", label, appName)
			}
		}

		runner := &executil.Runner{}
		codeMgr := code.NewManager(cfg.CodeDir, runner)

		cs, err := codeMgr.Add(appName, *slot, name, source, branch)
		if err != nil {
			return err
		}

		info.Code = append(info.Code, cs)

		// Patch the existing compose file in-place (add mount)
		if slot.InjectMode() == "volume" {
			if err := patchComposeCodeVolumes(appName, meta.Code.Slots, info.Code); err != nil {
				return err
			}
		}

		// For build slots: copy into build context, build, then clean up
		var buildPaths []string
		if slot.InjectMode() == "build" {
			var copyErr error
			buildPaths, copyErr = code.CopyBuildSources(cfg.AppDir(appName), meta.Code.Slots, info.Code, cfg.CodeDir, appName, runner)
			if copyErr != nil {
				return fmt.Errorf("copying build sources: %w", copyErr)
			}
		}

		// Restart containers
		c := compose.New(runner, cfg.Docker.ComposeCommand)
		restartErr := cliutil.RunWithSpinner(fmt.Sprintf("Restarting %s...", appName), func() error {
			if meta.RequiresBuild {
				_, err := c.UpWithBuild(cfg.AppDir(appName))
				return err
			}
			_, err := c.Up(cfg.AppDir(appName))
			return err
		})

		code.CleanBuildSources(buildPaths)

		if restartErr != nil {
			return fmt.Errorf("restarting containers: %w", restartErr)
		}

		// Save state only after successful restart
		if err := mgr.SaveDeployedInfo(appName, info); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}

		label := slotName
		if name != "" {
			label += "/" + name
		}
		fmt.Printf("Code source %s added to %s.\n", label, appName)
		return nil
	},
}

var codeRemoveCmd = &cobra.Command{
	Use:   "remove <app> <slot> [name]",
	Short: "Remove a code source from a deployed app",
	Example: `  aradeploy code remove wordpress themes my-theme
  aradeploy code remove vite-app src`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		appName := args[0]
		slotName := args[1]
		var name string
		if len(args) == 3 {
			name = args[2]
		}

		info, err := mgr.GetDeployedInfo(appName)
		if err != nil {
			return fmt.Errorf("app %s is not deployed: %w", appName, err)
		}

		// Find and remove the code source from state
		found := false
		var removedSource code.Source
		var remaining []code.Source
		for _, cs := range info.Code {
			if cs.Slot == slotName && cs.Name == name {
				found = true
				removedSource = cs
				continue
			}
			remaining = append(remaining, cs)
		}
		if !found {
			label := slotName
			if name != "" {
				label += "/" + name
			}
			return fmt.Errorf("code source %s not found in %s", label, appName)
		}

		runner := &executil.Runner{}
		codeMgr := code.NewManager(cfg.CodeDir, runner)

		if err := codeMgr.Remove(appName, slotName, name); err != nil {
			return fmt.Errorf("removing code: %w", err)
		}

		// Patch the existing compose file in-place (remove mount) or clean build sources
		meta, ok := mgr.Registry().Get(appName)
		if ok && meta.Code != nil {
			slot, slotFound := findSlot(meta.Code.Slots, slotName)
			if slotFound && slot.InjectMode() == "build" {
				// Clean stale build source from app dir
				targetPath := filepath.Join(cfg.AppDir(appName), slotName)
				if name != "" {
					targetPath = filepath.Join(cfg.AppDir(appName), slotName, name)
				}
				_ = os.RemoveAll(targetPath)
			} else {
				// Remove volumes for the removed sources only
				removed := []code.Source{removedSource}
				if err := removeComposeCodeVolumes(appName, meta.Code.Slots, removed); err != nil {
					return err
				}
			}
		}

		info.Code = remaining

		// Restart containers
		c := compose.New(runner, cfg.Docker.ComposeCommand)
		if err := cliutil.RunWithSpinner(fmt.Sprintf("Restarting %s...", appName), func() error {
			if ok && meta.RequiresBuild {
				_, err := c.UpWithBuild(cfg.AppDir(appName))
				return err
			}
			_, err := c.Up(cfg.AppDir(appName))
			return err
		}); err != nil {
			return fmt.Errorf("restarting containers: %w", err)
		}

		// Save state only after successful restart
		if err := mgr.SaveDeployedInfo(appName, info); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}

		label := slotName
		if name != "" {
			label += "/" + name
		}
		fmt.Printf("Code source %s removed from %s.\n", label, appName)
		return nil
	},
}

var codeListCmd = &cobra.Command{
	Use:   "list <app>",
	Short: "List code sources for a deployed app",
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

		if len(info.Code) == 0 {
			fmt.Printf("No code sources configured for %s.\n", appName)
			return nil
		}

		if jsonOutput {
			return outputJSON(info.Code)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "SLOT\tNAME\tSOURCE\tTYPE\tBRANCH")
		for _, cs := range info.Code {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", cs.Slot, cs.Name, cs.Source, cs.Type, cs.Branch)
		}
		_ = w.Flush()
		return nil
	},
}

var codeUpdateCmd = &cobra.Command{
	Use:   "update <app>",
	Short: "Update all code sources for a deployed app",
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

		if len(info.Code) == 0 {
			fmt.Printf("No code sources configured for %s.\n", appName)
			return nil
		}

		runner := &executil.Runner{}
		codeMgr := code.NewManager(cfg.CodeDir, runner)

		fmt.Printf("Updating code sources for %s...\n", appName)
		if err := codeMgr.Update(appName, info.Code); err != nil {
			return err
		}

		// Check if any build slots need rebuild
		meta, ok := mgr.Registry().Get(appName)
		hasBuildSlots := false
		if ok && meta.Code != nil {
			slotMap := make(map[string]string)
			for _, s := range meta.Code.Slots {
				slotMap[s.Name] = s.InjectMode()
			}
			for _, cs := range info.Code {
				if slotMap[cs.Slot] == "build" {
					hasBuildSlots = true
					break
				}
			}
		}

		// For build slots: copy into build context, build, then clean up
		var buildPaths []string
		if hasBuildSlots && ok && meta.Code != nil {
			var copyErr error
			buildPaths, copyErr = code.CopyBuildSources(cfg.AppDir(appName), meta.Code.Slots, info.Code, cfg.CodeDir, appName, runner)
			if copyErr != nil {
				return fmt.Errorf("copying build sources: %w", copyErr)
			}
		}

		c := compose.New(runner, cfg.Docker.ComposeCommand)
		var restartErr error
		if hasBuildSlots {
			restartErr = cliutil.RunWithSpinner(fmt.Sprintf("Rebuilding %s...", appName), func() error {
				_, err := c.UpWithBuild(cfg.AppDir(appName))
				return err
			})
		} else {
			restartErr = cliutil.RunWithSpinner(fmt.Sprintf("Restarting %s...", appName), func() error {
				_, err := c.Restart(cfg.AppDir(appName))
				return err
			})
		}

		code.CleanBuildSources(buildPaths)

		if restartErr != nil {
			return fmt.Errorf("restarting containers: %w", restartErr)
		}

		fmt.Printf("Code sources updated for %s.\n", appName)
		return nil
	},
}

// patchComposeCodeVolumes reads the existing docker-compose.yml, injects code volumes, and writes it back.
func patchComposeCodeVolumes(appName string, slots []template.CodeSlot, sources []code.Source) error {
	composePath := filepath.Join(cfg.AppDir(appName), "docker-compose.yml")
	data, err := os.ReadFile(composePath) // #nosec G304 -- path constructed from config
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	modified, err := code.InjectCodeVolumes(string(data), slots, sources, cfg.CodeDir, appName)
	if err != nil {
		return fmt.Errorf("injecting code volumes: %w", err)
	}

	if err := os.WriteFile(composePath, []byte(modified), 0o600); err != nil { // #nosec G703 -- composePath is constructed from config
		return fmt.Errorf("writing compose file: %w", err)
	}
	return nil
}

// removeComposeCodeVolumes reads the existing docker-compose.yml, removes the specified code volumes, and writes it back.
func removeComposeCodeVolumes(appName string, slots []template.CodeSlot, removed []code.Source) error {
	composePath := filepath.Join(cfg.AppDir(appName), "docker-compose.yml")
	data, err := os.ReadFile(composePath) // #nosec G304 -- path constructed from config
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	modified, err := code.RemoveCodeVolumes(string(data), slots, removed, cfg.CodeDir, appName)
	if err != nil {
		return fmt.Errorf("removing code volumes: %w", err)
	}

	if err := os.WriteFile(composePath, []byte(modified), 0o600); err != nil { // #nosec G703 -- composePath is constructed from config
		return fmt.Errorf("writing compose file: %w", err)
	}
	return nil
}

// findSlot looks up a code slot by name.
func findSlot(slots []template.CodeSlot, name string) (template.CodeSlot, bool) {
	for _, s := range slots {
		if s.Name == name {
			return s, true
		}
	}
	return template.CodeSlot{}, false
}
