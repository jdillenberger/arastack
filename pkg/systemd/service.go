package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// ServiceConfig defines the parameters for a systemd service.
type ServiceConfig struct {
	BinaryName  string
	ServiceName string
	Description string
	ExecArgs    string   // additional args after the binary path (e.g. "run --port 7120")
	After       []string // additional After= units (e.g. "docker.service")
}

func (c *ServiceConfig) unitName() string {
	return c.ServiceName + ".service"
}

func (c *ServiceConfig) unitPath() string {
	return "/etc/systemd/system/" + c.unitName()
}

func (c *ServiceConfig) generateUnitFile() string {
	binPath := "/usr/local/bin/" + c.BinaryName
	// When running from the tool's own binary (e.g. `labalert service install`),
	// try to use the actual executable path. When running from labmanager for a
	// different tool, always use the standard install path.
	if exe, err := os.Executable(); err == nil {
		resolved, _ := filepath.EvalSymlinks(exe)
		if resolved == "" {
			resolved = exe
		}
		if filepath.Base(resolved) == c.BinaryName {
			binPath = resolved
		}
	}
	execStart := binPath
	if c.ExecArgs != "" {
		execStart += " " + c.ExecArgs
	}
	afterLine := "network-online.target"
	for _, a := range c.After {
		afterLine += " " + a
	}
	return fmt.Sprintf(`[Unit]
Description=%s
After=%s
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, c.Description, afterLine, execStart)
}

// Install writes the systemd unit file, reloads the daemon, and enables+starts
// the service. It can be called directly (e.g. from a setup command) without
// going through the cobra command tree.
func (c *ServiceConfig) Install() error {
	unit := c.generateUnitFile()

	if err := os.WriteFile(c.unitPath(), []byte(unit), 0o644); err != nil {
		return fmt.Errorf("writing unit file: %w (are you root?)", err)
	}
	fmt.Printf("Unit file written to %s\n", c.unitPath())

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", c.unitName()); err != nil {
		return err
	}
	if err := runSystemctl("start", c.unitName()); err != nil {
		return err
	}

	fmt.Println("Service installed and started.")
	return nil
}

// Start starts the systemd service.
func (c *ServiceConfig) Start() error {
	return runSystemctl("start", c.unitName())
}

// Stop stops the systemd service.
func (c *ServiceConfig) Stop() error {
	return runSystemctl("stop", c.unitName())
}

// Restart restarts the systemd service.
func (c *ServiceConfig) Restart() error {
	return runSystemctl("restart", c.unitName())
}

// Uninstall stops, disables, and removes the systemd service.
func (c *ServiceConfig) Uninstall() error {
	_ = runSystemctl("stop", c.unitName())
	_ = runSystemctl("disable", c.unitName())

	if err := os.Remove(c.unitPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}

	fmt.Println("Service uninstalled.")
	return nil
}

// IsActive returns true if the systemd service is currently active.
func (c *ServiceConfig) IsActive() bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", c.unitName())
	return cmd.Run() == nil
}

// NewCommand returns a cobra command group for managing a systemd service.
func NewCommand(cfg ServiceConfig) *cobra.Command {
	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: fmt.Sprintf("Manage the %s systemd service", cfg.BinaryName),
	}

	serviceCmd.AddCommand(newInstallCmd(&cfg))
	serviceCmd.AddCommand(newUninstallCmd(&cfg))
	serviceCmd.AddCommand(newStatusCmd(&cfg))

	return serviceCmd
}

func newInstallCmd(cfg *ServiceConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start the systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cfg.Install()
		},
	}
}

func newUninstallCmd(cfg *ServiceConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = runSystemctl("stop", cfg.unitName())
			_ = runSystemctl("disable", cfg.unitName())

			if err := os.Remove(cfg.unitPath()); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing unit file: %w", err)
			}

			if err := runSystemctl("daemon-reload"); err != nil {
				return err
			}

			fmt.Println("Service uninstalled.")
			return nil
		},
	}
}

func newStatusCmd(cfg *ServiceConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show systemd service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("systemctl", "status", cfg.unitName())
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			_ = c.Run()
			return nil
		},
	}
}

func runSystemctl(args ...string) error {
	c := exec.Command("systemctl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("systemctl %v: %w", args, err)
	}
	return nil
}
