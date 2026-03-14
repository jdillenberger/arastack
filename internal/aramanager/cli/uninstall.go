package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
)

var yesFlag bool

func init() {
	uninstallCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "skip interactive wizard and remove everything")
	rootCmd.AddCommand(uninstallCmd)
}

var uninstallCmd = &cobra.Command{
	Use:     "uninstall",
	Short:   "Uninstall arastack tools, services, and optionally config/deployments",
	Long:    "Interactive wizard to uninstall arastack components. Stops services, removes binaries, and optionally removes configuration files, deployments, and aramanager itself.",
	PreRunE: requireSudo,
	RunE: func(cmd *cobra.Command, args []string) error {
		removeConfig := false
		removeDeployments := false
		removeAramanager := false

		if yesFlag {
			removeConfig = true
			removeDeployments = true
			removeAramanager = true
		} else {
			var optionalTargets []string

			selectForm := huh.NewForm(
				huh.NewGroup(
					huh.NewMultiSelect[string]().
						Title("What else should be removed?").
						Description("Services and binaries are always removed.").
						Options(
							huh.NewOption("Remove configuration files (/etc/arastack/config/, ~/.arastack/config/)", "config"),
							huh.NewOption("Remove deployments (/opt/aradeploy/apps)", "deployments"),
						).
						Value(&optionalTargets),
				),
			)

			if err := selectForm.Run(); err != nil {
				return fmt.Errorf("wizard cancelled: %w", err)
			}

			for _, t := range optionalTargets {
				switch t {
				case "config":
					removeConfig = true
				case "deployments":
					removeDeployments = true
				}
			}

			managerForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Also remove aramanager itself?").
						Value(&removeAramanager),
				),
			)

			if err := managerForm.Run(); err != nil {
				return fmt.Errorf("wizard cancelled: %w", err)
			}

			// Build summary
			summary := "Will remove:\n"
			summary += "  - All arastack systemd services\n"
			summary += "  - All arastack tool binaries\n"
			if removeConfig {
				summary += "  - Configuration files\n"
			}
			if removeDeployments {
				summary += "  - Deployments (/opt/aradeploy/apps)\n"
			}
			if removeAramanager {
				summary += "  - aramanager binary\n"
			}

			fmt.Println(summary)

			var confirmed bool
			confirmForm := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Proceed with uninstall?").
						Value(&confirmed),
				),
			)

			if err := confirmForm.Run(); err != nil {
				return fmt.Errorf("wizard cancelled: %w", err)
			}

			if !confirmed {
				fmt.Println("Uninstall cancelled.")
				return nil
			}
		}

		var errs []string
		tools := registry.All()

		// 1. Stop and uninstall all systemd services
		fmt.Println("Stopping and removing systemd services...")
		for _, tool := range tools {
			fmt.Printf("  Uninstalling %s service...\n", tool.Name)
			if err := tool.ServiceConfig.Uninstall(); err != nil {
				errs = append(errs, fmt.Sprintf("%s service: %v", tool.Name, err))
			}
		}

		// 2. Remove all tool binaries
		fmt.Println("Removing tool binaries...")
		for _, tool := range tools {
			binPath, err := exec.LookPath(tool.BinaryName)
			if err != nil {
				binPath = "/usr/local/bin/" + tool.BinaryName
			}
			fmt.Printf("  Removing %s...\n", binPath)
			if err := removeFileWithSudo(binPath); err != nil {
				errs = append(errs, fmt.Sprintf("%s binary: %v", tool.Name, err))
			}
		}

		// 3. Remove config files
		if removeConfig {
			fmt.Println("Removing configuration files...")
			configDirs := []string{"/etc/arastack/config/"}
			if home, err := os.UserHomeDir(); err == nil {
				configDirs = append(configDirs, filepath.Join(home, ".arastack", "config"))
			}
			for _, dir := range configDirs {
				fmt.Printf("  Removing %s...\n", dir)
				if err := removeAllWithSudo(dir); err != nil {
					errs = append(errs, fmt.Sprintf("config %s: %v", dir, err))
				}
			}
		}

		// 4. Remove deployments
		if removeDeployments {
			appsDir := "/opt/aradeploy/apps"
			fmt.Printf("Removing deployments (%s)...\n", appsDir)
			if err := removeAllWithSudo(appsDir); err != nil {
				errs = append(errs, fmt.Sprintf("deployments: %v", err))
			}
		}

		// 5. Remove aramanager itself
		if removeAramanager {
			exe, err := os.Executable()
			if err != nil {
				errs = append(errs, fmt.Sprintf("aramanager: could not determine executable path: %v", err))
			} else {
				resolved, err := filepath.EvalSymlinks(exe)
				if err != nil {
					resolved = exe
				}
				fmt.Printf("Removing aramanager (%s)...\n", resolved)
				if err := removeFileWithSudo(resolved); err != nil {
					errs = append(errs, fmt.Sprintf("aramanager binary: %v", err))
				}
			}
		}

		if len(errs) > 0 {
			fmt.Println("\nErrors during uninstall:")
			for _, e := range errs {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("%d error(s) during uninstall", len(errs))
		}

		fmt.Println("\nUninstall complete.")
		return nil
	},
}

// removeAllWithSudo removes a directory tree, using sudo when not root
// (sudo credentials are already validated by PreRunE).
func removeAllWithSudo(path string) error {
	if os.Geteuid() == 0 {
		return os.RemoveAll(path)
	}
	cmd := exec.CommandContext(context.Background(), "sudo", "rm", "-rf", path) // #nosec G204 -- path is from internal uninstall logic
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo rm -rf %s: %w", path, err)
	}
	return nil
}

// removeFileWithSudo removes a file, using sudo when not root
// (sudo credentials are already validated by PreRunE).
func removeFileWithSudo(path string) error {
	if os.Geteuid() == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cmd := exec.CommandContext(context.Background(), "sudo", "rm", "-f", path) // #nosec G204 -- path is from internal uninstall logic
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo rm %s: %w", path, err)
	}
	return nil
}
