package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/image"
)

type outdatedResult struct {
	App     string                `json:"app"`
	Image   string                `json:"image"`
	Current string                `json:"current"`
	Updates []image.VersionUpdate `json:"updates,omitempty"`
	Error   string                `json:"error,omitempty"`
}

func init() {
	rootCmd.AddCommand(outdatedCmd)
	outdatedCmd.ValidArgsFunction = completeDeployedApps
}

var outdatedCmd = &cobra.Command{
	Use:   "outdated [app]",
	Short: "Check for newer container image versions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		var appsToCheck []string
		if len(args) > 0 {
			appsToCheck = []string{args[0]}
		} else {
			deployed, err := mgr.ListDeployed()
			if err != nil {
				return err
			}
			appsToCheck = deployed
		}

		if len(appsToCheck) == 0 {
			fmt.Println("No apps deployed.")
			return nil
		}

		type imageCheck struct {
			appName string
			ref     image.Ref
			img     string
		}
		var checks []imageCheck

		for _, appName := range appsToCheck {
			composePath := filepath.Join(cfg.AppDir(appName), "docker-compose.yml")
			data, err := os.ReadFile(composePath)
			if err != nil {
				continue
			}
			refs, err := image.ScanDeployed(data)
			if err != nil {
				continue
			}
			for _, ref := range refs {
				if _, err := image.ParseSemver(ref.Tag); err != nil {
					continue
				}
				checks = append(checks, imageCheck{appName: appName, ref: ref, img: ref.String()})
			}
		}

		if len(checks) == 0 {
			fmt.Println("No semver-tagged images found in deployed apps.")
			return nil
		}

		resolver := image.NewResolver()
		results := make([]outdatedResult, len(checks))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5)

		for i, chk := range checks {
			wg.Add(1)
			go func(idx int, c imageCheck) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				r := outdatedResult{App: c.appName, Image: c.img, Current: c.ref.Tag}
				updates, err := resolver.FindNewerVersions(c.ref)
				if err != nil {
					r.Error = err.Error()
				} else {
					r.Updates = updates
				}
				results[idx] = r
			}(i, chk)
		}
		wg.Wait()

		var withUpdates []outdatedResult
		for _, r := range results {
			if len(r.Updates) > 0 || r.Error != "" {
				withUpdates = append(withUpdates, r)
			}
		}

		if jsonOutput {
			return outputJSON(results)
		}

		if len(withUpdates) == 0 {
			fmt.Println("All images are up to date.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "APP\tIMAGE\tCURRENT\tLATEST\tTYPE")
		for _, r := range withUpdates {
			if r.Error != "" {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t-\terror: %s\n", r.App, r.Image, r.Current, r.Error)
				continue
			}
			for _, u := range r.Updates {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.App, r.Image, u.CurrentTag, u.NewTag, u.Type)
			}
		}
		_ = w.Flush()
		return nil
	},
}
