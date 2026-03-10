package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
)

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage systemd services for arastack tools",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install [tool]",
	Short: "Install and start systemd service(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTool(args, func(t registry.Tool) error {
			fmt.Printf("Installing %s service...\n", t.Name)
			return t.ServiceConfig.Install()
		})
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall [tool]",
	Short: "Stop and remove systemd service(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTool(args, func(t registry.Tool) error {
			fmt.Printf("Uninstalling %s service...\n", t.Name)
			return t.ServiceConfig.Uninstall()
		})
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status [tool]",
	Short: "Show systemd service status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tools := registry.All()
		if len(args) == 1 {
			t := registry.ByName(args[0])
			if t == nil {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			c := exec.Command("systemctl", "status", t.ServiceName+".service") // #nosec G204 -- command arguments are not user-controlled
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			_ = c.Run()
			return nil
		}

		for _, t := range tools {
			status := "inactive"
			if t.ServiceConfig.IsActive() {
				status = "active"
			}
			portInfo := ""
			if t.Port > 0 {
				portInfo = fmt.Sprintf(" (port %d)", t.Port)
			}
			mark := "[ ]"
			if status == "active" {
				mark = "[x]"
			}
			fmt.Printf("  %s %-20s %s%s\n", mark, t.Name, status, portInfo)
		}
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start [tool]",
	Short: "Start systemd service(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTool(args, func(t registry.Tool) error {
			fmt.Printf("Starting %s...\n", t.Name)
			return t.ServiceConfig.Start()
		})
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop [tool]",
	Short: "Stop systemd service(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTool(args, func(t registry.Tool) error {
			fmt.Printf("Stopping %s...\n", t.Name)
			return t.ServiceConfig.Stop()
		})
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart [tool]",
	Short: "Restart systemd service(s)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return forEachTool(args, func(t registry.Tool) error {
			fmt.Printf("Restarting %s...\n", t.Name)
			return t.ServiceConfig.Restart()
		})
	},
}

func forEachTool(args []string, fn func(registry.Tool) error) error {
	if len(args) == 1 {
		t := registry.ByName(args[0])
		if t == nil {
			return fmt.Errorf("unknown tool: %s", args[0])
		}
		return fn(*t)
	}

	var errs []string
	for _, t := range registry.All() {
		if err := fn(t); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", t.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}
