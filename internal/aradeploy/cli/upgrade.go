package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/compose"
	"github.com/jdillenberger/arastack/internal/aradeploy/config"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/image"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().Bool("dry-run", false, "Show what would change without applying")
	upgradeCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	upgradeCmd.Flags().Bool("check", false, "Only show available image updates")
	upgradeCmd.Flags().Bool("patch-only", false, "Only apply patch-level image updates")
	upgradeCmd.Flags().Bool("all", false, "Upgrade all deployed apps")
	upgradeCmd.Flags().Bool("images-only", false, "Only pull latest images and recreate containers (no template upgrade)")
	upgradeCmd.ValidArgsFunction = completeDeployedApps
}

type imageUpdatePlan struct {
	ref    image.Ref
	update image.VersionUpdate
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [app]",
	Short: "Upgrade an app's template or container images",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		yes, _ := cmd.Flags().GetBool("yes")
		check, _ := cmd.Flags().GetBool("check")
		patchOnly, _ := cmd.Flags().GetBool("patch-only")
		all, _ := cmd.Flags().GetBool("all")
		imagesOnly, _ := cmd.Flags().GetBool("images-only")

		if all {
			if imagesOnly {
				flushSpooledEvents()
			}
			deployed, err := mgr.ListDeployed()
			if err != nil {
				return err
			}
			if len(deployed) == 0 {
				fmt.Println("No apps deployed.")
				return nil
			}
			var errs []string
			for _, appName := range deployed {
				fmt.Printf("=== %s ===\n", appName)
				if imagesOnly {
					if err := mgr.Update(appName); err != nil {
						fmt.Printf("  Error: %v\n", err)
						errs = append(errs, appName)
						pushUpdateFailedEvent(appName, err)
					} else {
						fmt.Printf("  %s updated.\n", appName)
					}
				} else {
					if err := runUpgrade(cfg, mgr, appName, dryRun, yes, check, patchOnly); err != nil {
						fmt.Printf("  Error: %v\n", err)
						errs = append(errs, appName)
						pushUpdateFailedEvent(appName, err)
					}
				}
				fmt.Println()
			}
			if len(errs) > 0 {
				return fmt.Errorf("upgrade failed for: %s", strings.Join(errs, ", "))
			}
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("app name required (or use --all)")
		}

		if imagesOnly {
			return mgr.Update(args[0])
		}
		return runUpgrade(cfg, mgr, args[0], dryRun, yes, check, patchOnly)
	},
}

func runUpgrade(cfg *config.Config, mgr *deploy.Manager, appName string, dryRun, yes, check, patchOnly bool) error {
	info, err := mgr.GetDeployedInfo(appName)
	if err != nil {
		return fmt.Errorf("app %s is not deployed: %w", appName, err)
	}

	appDir := cfg.AppDir(appName)
	composePath := filepath.Join(appDir, "docker-compose.yml")
	composeData, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	refs, _ := image.ScanDeployed(composeData)
	var imageUpdates []imageUpdatePlan
	var floatingTags []image.Ref
	resolver := image.NewResolver()

	for _, ref := range refs {
		if _, err := image.ParseSemver(ref.Tag); err != nil {
			floatingTags = append(floatingTags, ref)
			continue
		}
		updates, err := resolver.FindNewerVersions(ref)
		if err != nil {
			fmt.Printf("  Warning: could not check %s — registry may be unavailable or rate-limited: %v\n", ref.String(), err)
			continue
		}
		for _, u := range updates {
			if patchOnly && u.Type != "patch" {
				continue
			}
			imageUpdates = append(imageUpdates, imageUpdatePlan{ref: ref, update: u})
		}
	}

	if check {
		if len(imageUpdates) == 0 && len(floatingTags) == 0 {
			fmt.Printf("No image updates available for %s.\n", appName)
			return nil
		}
		if len(imageUpdates) > 0 {
			fmt.Printf("Available image updates for %s:\n", appName)
			for _, iu := range imageUpdates {
				fmt.Printf("  %s: %s -> %s (%s)\n", iu.ref.String(), iu.update.CurrentTag, iu.update.NewTag, iu.update.Type)
			}
		}
		if len(floatingTags) > 0 {
			fmt.Printf("\nFloating (non-semver) tags in %s:\n", appName)
			for _, ref := range floatingTags {
				fmt.Printf("  %s (tag: %s)\n", ref.String(), ref.Tag)
			}
		}
		return nil
	}

	meta, ok := mgr.Registry().Get(appName)
	templateChanged := false
	if ok {
		fmt.Printf("App:              %s\n", appName)
		fmt.Printf("Deployed version: %s\n", info.Version)
		fmt.Printf("Template version: %s\n", meta.Version)
		if info.Version != meta.Version {
			fmt.Printf("\nTemplate upgrade available: %s -> %s\n", info.Version, meta.Version)
			templateChanged = true
		}
	}

	if len(imageUpdates) > 0 {
		fmt.Println("\nImage updates:")
		for _, iu := range imageUpdates {
			fmt.Printf("  %s: %s -> %s (%s)\n", iu.ref.String(), iu.update.CurrentTag, iu.update.NewTag, iu.update.Type)
		}
	}

	if len(floatingTags) > 0 {
		fmt.Println("\nNote: floating (non-semver) tags detected:")
		for _, ref := range floatingTags {
			fmt.Printf("  %s (tag: %s)\n", ref.String(), ref.Tag)
		}
		fmt.Println("  Use 'upgrade --images-only' to pull latest versions of these images.")
	}

	hasTemplateChanges := false
	var rendered map[string]string
	if ok && templateChanged {
		renderer := template.NewRenderer(mgr.Registry())
		rendered, err = renderer.RenderAllFiles(appName, info.Values)
		if err != nil {
			return fmt.Errorf("rendering templates: %w", err)
		}

		fmt.Println("\nTemplate changes:")
		for name, newContent := range rendered {
			existingPath := filepath.Join(appDir, name)
			existingData, err := os.ReadFile(existingPath) // #nosec G304 -- path is constructed internally
			if err != nil {
				fmt.Printf("  + %s (new file)\n", name)
				hasTemplateChanges = true
				continue
			}
			if string(existingData) != newContent {
				fmt.Printf("  ~ %s (modified)\n", name)
				hasTemplateChanges = true
			}
		}
		if !hasTemplateChanges {
			fmt.Println("  No template file changes detected.")
		}
	}

	if !hasTemplateChanges && len(imageUpdates) == 0 {
		fmt.Println("\nNothing to upgrade.")
		return nil
	}

	if dryRun {
		fmt.Println("\nDry run — no changes applied.")
		return nil
	}

	if !yes && !deploy.AskConfirmation("Apply upgrade?") {
		fmt.Println("Upgrade cancelled.")
		return nil
	}

	origCompose, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return fmt.Errorf("reading compose file for backup: %w", err)
	}

	if hasTemplateChanges && rendered != nil {
		for name, content := range rendered {
			outPath := filepath.Join(appDir, name)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
				return fmt.Errorf("creating directory for %s: %w", name, err)
			}
			if err := os.WriteFile(outPath, []byte(content), 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", name, err)
			}
		}
	}

	if len(imageUpdates) > 0 {
		content, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
		if err != nil {
			return fmt.Errorf("reading compose file for image update: %w", err)
		}
		text := string(content)
		for _, iu := range imageUpdates {
			oldImage := iu.ref.String()
			newRef := iu.ref
			newRef.Tag = iu.update.NewTag
			text = strings.ReplaceAll(text, oldImage, newRef.String())
		}
		if err := os.WriteFile(composePath, []byte(text), 0o600); err != nil { // #nosec G703 -- composePath is constructed from config
			return fmt.Errorf("writing updated compose file: %w", err)
		}
	}

	runner := &executil.Runner{Verbose: verbose}
	c := compose.New(runner, cfg.Docker.ComposeCommand)
	fmt.Printf("Recreating containers for %s...\n", appName)
	var composeUpErr error
	if meta != nil && meta.RequiresBuild {
		_, composeUpErr = c.UpWithBuild(appDir)
	} else {
		_, composeUpErr = c.Up(appDir)
	}
	if composeUpErr != nil {
		fmt.Printf("Container recreation failed, rolling back compose file...\n")
		if rollbackErr := os.WriteFile(composePath, origCompose, 0o600); rollbackErr != nil { // #nosec G703 -- composePath is constructed from config
			return fmt.Errorf("recreating containers: %w (rollback also failed: %w)", composeUpErr, rollbackErr)
		}
		return fmt.Errorf("recreating containers (rolled back): %w", composeUpErr)
	}

	if ok && templateChanged {
		info.Version = meta.Version
	}
	if err := mgr.SaveDeployedInfo(appName, info); err != nil {
		return fmt.Errorf("updating deploy info: %w", err)
	}

	fmt.Printf("\nApp %s upgraded successfully.\n", appName)
	return nil
}
