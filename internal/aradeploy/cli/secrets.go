package cli

import (
	"fmt"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/spf13/cobra"
)

var secretsInternal bool

func init() {
	rootCmd.AddCommand(secretsCmd)
	secretsCmd.ValidArgsFunction = completeSecretsArgs
	secretsCmd.Flags().BoolVar(&secretsInternal, "internal", false, "Include internal secrets (DB passwords, encryption keys)")
}

// completeSecretsArgs completes app names for arg 0 and secret names for arg 1.
func completeSecretsArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeDeployedApps(cmd, args, toComplete)
	}
	if len(args) == 1 {
		mgr, err := newManager()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		secrets, err := mgr.GetSecrets(args[0])
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		var names []string
		for _, s := range secrets {
			names = append(names, s.Name)
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

var secretsCmd = &cobra.Command{
	Use:   "secrets [app] [secret-name]",
	Short: "Show secrets for a deployed app",
	Long: `Show secrets for a deployed app.

Without arguments, lists all deployed apps that have secrets.
With an app name, shows login credentials (and internal secrets with --internal).
With an app name and secret name, prints just that secret's value (for scripting).`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		// No app specified: list all deployed apps that have secrets.
		if len(args) == 0 {
			return listAppsWithSecrets(mgr)
		}

		appName := args[0]
		secrets, err := mgr.GetSecrets(appName)
		if err != nil {
			return err
		}

		if len(secrets) == 0 {
			fmt.Printf("No secrets found for %s.\n", appName)
			return nil
		}

		// Second positional arg: print a single secret's value for scripting.
		if len(args) == 2 {
			secretName := args[1]
			for _, s := range secrets {
				if s.Name == secretName {
					fmt.Print(s.Value)
					return nil
				}
			}
			return fmt.Errorf("secret %q not found for %s", secretName, appName)
		}

		if jsonOutput {
			if !secretsInternal {
				var filtered []interface{}
				for _, s := range secrets {
					if s.UserFacing {
						filtered = append(filtered, s)
					}
				}
				if len(filtered) == 0 {
					fmt.Printf("No login credentials found for %s.\n", appName)
					return nil
				}
				return outputJSON(filtered)
			}
			return outputJSON(secrets)
		}

		// Split into user-facing and internal.
		var userFacing, internal []struct{ label, value string }
		for _, s := range secrets {
			label := s.Name
			if s.Description != "" {
				label = s.Description
			}
			entry := struct{ label, value string }{label, s.Value}
			if s.UserFacing {
				userFacing = append(userFacing, entry)
			} else {
				internal = append(internal, entry)
			}
		}

		fmt.Printf("Secrets for %s:\n\n", appName)

		if len(userFacing) > 0 {
			fmt.Println("  Login credentials:")
			for _, e := range userFacing {
				fmt.Printf("    %-30s %s\n", e.label+":", e.value)
			}
		}

		if secretsInternal && len(internal) > 0 {
			if len(userFacing) > 0 {
				fmt.Println()
			}
			fmt.Println("  Internal secrets:")
			for _, e := range internal {
				fmt.Printf("    %-30s %s\n", e.label+":", e.value)
			}
		} else if !secretsInternal && len(internal) > 0 && len(userFacing) == 0 {
			fmt.Printf("  No login credentials. Use --internal to show internal secrets.\n")
		} else if !secretsInternal && len(internal) > 0 {
			fmt.Printf("\n  Use --internal to show %d internal secret(s).\n", len(internal))
		}

		return nil
	},
}

func listAppsWithSecrets(mgr *deploy.Manager) error {
	deployed, err := mgr.ListDeployed()
	if err != nil {
		return err
	}
	if len(deployed) == 0 {
		fmt.Println("No deployed apps.")
		return nil
	}

	found := false
	for _, name := range deployed {
		secrets, err := mgr.GetSecrets(name)
		if err != nil || len(secrets) == 0 {
			continue
		}
		userCount := 0
		for _, s := range secrets {
			if s.UserFacing {
				userCount++
			}
		}
		if userCount > 0 || secretsInternal {
			if !found {
				fmt.Println("Apps with secrets:")
				fmt.Println()
				found = true
			}
			fmt.Printf("  %-20s %d login credential(s)", name, userCount)
			if secretsInternal {
				fmt.Printf(", %d internal", len(secrets)-userCount)
			}
			fmt.Println()
		}
	}

	if !found {
		fmt.Println("No deployed apps have secrets.")
	}
	return nil
}
