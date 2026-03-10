package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/pkg/cliutil"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configEditCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or initialize tool configuration files",
}

var configShowCmd = &cobra.Command{
	Use:   "show [tool]",
	Short: "Show config file paths and contents",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var tools []registry.Tool
		if len(args) == 1 {
			t := registry.ByName(args[0])
			if t == nil {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			tools = []registry.Tool{*t}
		} else {
			tools = registry.All()
		}

		for _, t := range tools {
			if t.ConfigPath == "" {
				fmt.Printf("%-20s no config file\n", t.Name)
				continue
			}

			if _, err := os.Stat(t.ConfigPath); os.IsNotExist(err) {
				fmt.Printf("%-20s %s (not found)\n", t.Name, t.ConfigPath)
			} else {
				fmt.Printf("%-20s %s\n", t.Name, t.ConfigPath)
				if len(args) == 1 {
					data, err := os.ReadFile(t.ConfigPath)
					if err != nil {
						fmt.Printf("  error reading: %v\n", err)
					} else {
						fmt.Println(string(data))
					}
				}
			}
		}
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init [tool]",
	Short: "Create default config files via doctor --fix",
	Long:  "Runs each tool's doctor fix for config-related checks to create missing config files.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var tools []registry.Tool
		if len(args) == 1 {
			t := registry.ByName(args[0])
			if t == nil {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			tools = []registry.Tool{*t}
		} else {
			tools = registry.All()
		}

		for _, t := range tools {
			if t.ConfigPath == "" {
				continue
			}

			if _, err := os.Stat(t.ConfigPath); err == nil {
				fmt.Printf("%-20s %s (already exists)\n", t.Name, t.ConfigPath)
				continue
			}

			if t.DoctorCheck == nil || t.DoctorFix == nil {
				fmt.Printf("%-20s %s (no auto-init available)\n", t.Name, t.ConfigPath)
				continue
			}

			results, err := t.DoctorCheck()
			if err != nil {
				fmt.Printf("%-20s error: %v\n", t.Name, err)
				continue
			}

			fixed := false
			for _, r := range results {
				if !r.Installed && (r.Name == "config-file" || r.Name == "config") {
					if err := t.DoctorFix(r); err != nil {
						fmt.Printf("%-20s fix failed: %v\n", t.Name, err)
					} else {
						fmt.Printf("%-20s %s (created)\n", t.Name, t.ConfigPath)
						fixed = true
					}
				}
			}
			if !fixed {
				fmt.Printf("%-20s %s (no config check found)\n", t.Name, t.ConfigPath)
			}
		}
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate [tool]",
	Short: "Validate config files for all or a specific tool",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var tools []registry.Tool
		if len(args) == 1 {
			t := registry.ByName(args[0])
			if t == nil {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			tools = []registry.Tool{*t}
		} else {
			tools = registry.All()
		}

		allOK := true
		for _, t := range tools {
			if t.ConfigPath == "" {
				continue
			}

			if _, err := os.Stat(t.ConfigPath); os.IsNotExist(err) {
				fmt.Printf("  %s %-20s config file not found: %s\n", cliutil.StatusFail("✗"), t.Name, t.ConfigPath)
				allOK = false
				continue
			}

			if t.ConfigValidate == nil {
				fmt.Printf("  %s %-20s no validator available\n", cliutil.StatusWarn("!"), t.Name)
				continue
			}

			errs := t.ConfigValidate()
			if len(errs) > 0 {
				allOK = false
				fmt.Printf("  %s %-20s %s\n", cliutil.StatusFail("✗"), t.Name, t.ConfigPath)
				for _, e := range errs {
					fmt.Printf("      %s\n", e)
				}
			} else {
				fmt.Printf("  %s %-20s %s\n", cliutil.StatusOK("✓"), t.Name, t.ConfigPath)
			}
		}

		if !allOK {
			return fmt.Errorf("some configs have issues")
		}
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit <tool>",
	Short: "Open a tool's config file in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := registry.ByName(args[0])
		if t == nil {
			return fmt.Errorf("unknown tool: %s", args[0])
		}
		if t.ConfigPath == "" {
			return fmt.Errorf("%s has no config file", t.Name)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, t.ConfigPath) // #nosec G204 -- user's own $EDITOR
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}
