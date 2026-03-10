package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ejectCmd)
	ejectCmd.Flags().StringP("output", "o", "", "Output directory (default: ./aradeploy-eject)")
}

var ejectCmd = &cobra.Command{
	Use:   "eject",
	Short: "Export all generated configs for standalone use",
	Long: `Export all deployed app configs (docker-compose.yml, .env)
to a directory, so you can manage them without aradeploy.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		outputDir, _ := cmd.Flags().GetString("output")
		if outputDir == "" {
			outputDir = "aradeploy-eject"
		}

		deployed, err := mgr.ListDeployed()
		if err != nil {
			return err
		}

		if len(deployed) == 0 {
			fmt.Println("No apps deployed. Nothing to eject.")
			return nil
		}

		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		filesToCopy := []string{"docker-compose.yml", ".env"}

		var ejected int
		for _, appName := range deployed {
			appDir := cfg.AppDir(appName)
			destDir := filepath.Join(outputDir, appName)
			if err := os.MkdirAll(destDir, 0o750); err != nil {
				fmt.Printf("  %s: failed to create directory: %v\n", appName, err)
				continue
			}

			copied := 0
			for _, f := range filesToCopy {
				srcPath := filepath.Join(appDir, f)
				data, err := os.ReadFile(srcPath) // #nosec G304 -- path is constructed internally
				if err != nil {
					continue
				}
				destPath := filepath.Join(destDir, f)
				perm := os.FileMode(0o644)
				if f == ".env" {
					perm = 0o600
				}
				if err := os.WriteFile(destPath, data, perm); err != nil {
					fmt.Printf("  %s: failed to write %s: %v\n", appName, f, err)
					continue
				}
				copied++
			}

			if copied > 0 {
				fmt.Printf("  %s: exported %d file(s)\n", appName, copied)
				ejected++
			}
		}

		fmt.Printf("\nEjected %d app(s) to %s/\n", ejected, outputDir)
		fmt.Println("\nYou can now manage these apps with standard tools:")
		fmt.Println("  cd <app-dir> && docker compose up -d")
		return nil
	},
}
