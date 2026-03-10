package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/lint"
	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(templatesCmd)
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesExportCmd)
	templatesCmd.AddCommand(templatesDeleteCmd)
	templatesCmd.AddCommand(templatesPathCmd)
	templatesCmd.AddCommand(templatesNewCmd)
	templatesCmd.AddCommand(templatesLintCmd)

	templatesLintCmd.Flags().Bool("suppress", false, "Add all current warnings/infos to lint_ignore in app.yaml")
	templatesLintCmd.ValidArgsFunction = completeTemplateNames
	templatesExportCmd.Flags().Bool("force", false, "Overwrite existing local template")
	templatesExportCmd.ValidArgsFunction = completeTemplateNames
	templatesDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	templatesDeleteCmd.ValidArgsFunction = completeLocalTemplates
	templatesNewCmd.Flags().Bool("dockerfile", false, "Generate Dockerfile-based template")
}

func completeLocalTemplates(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := os.ReadDir(cfg.TemplatesDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage app templates",
	Long:  "List, export, and manage app templates.",
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available templates with source info",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		type templateEntry struct {
			Name        string `json:"name"`
			Category    string `json:"category"`
			Description string `json:"description"`
			Source      string `json:"source"`
			Status      string `json:"status"`
		}

		runner := &executil.Runner{Verbose: verbose}
		repoMgr := repo.NewManager(cfg.ReposDir(), cfg.ManifestPath(), runner)
		repoNames, _ := repoMgr.RepoNames()

		deployed, _ := mgr.ListDeployed()
		deployedSet := make(map[string]bool)
		for _, d := range deployed {
			deployedSet[d] = true
		}

		var entries []templateEntry
		for _, meta := range mgr.Registry().All() {
			source := template.ResolveSource(mgr.Registry().FS(), meta.Name, repoNames)
			status := "available"
			if deployedSet[meta.Name] {
				status = "deployed"
			}
			entries = append(entries, templateEntry{
				Name:        meta.Name,
				Category:    meta.Category,
				Description: meta.Description,
				Source:      source,
				Status:      status,
			})
		}

		if jsonOutput {
			return outputJSON(entries)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tCATEGORY\tSOURCE\tDESCRIPTION\tSTATUS")
		for _, e := range entries {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.Name, e.Category, e.Source, e.Description, e.Status)
		}
		_ = w.Flush()
		return nil
	},
}

var templatesExportCmd = &cobra.Command{
	Use:   "export <template>",
	Short: "Copy a template to the local templates directory for customization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		templateName := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if _, ok := mgr.Registry().Get(templateName); !ok {
			return fmt.Errorf("unknown template: %s", templateName)
		}

		destDir := filepath.Join(cfg.TemplatesDir, templateName)
		if _, err := os.Stat(destDir); err == nil && !force {
			return fmt.Errorf("local template %s already exists (use --force to overwrite)", templateName)
		}

		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return fmt.Errorf("creating template directory: %w", err)
		}

		tmplDir := templateName
		err = fs.WalkDir(mgr.Registry().FS(), tmplDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relPath := strings.TrimPrefix(path, tmplDir+"/")
			if relPath == tmplDir {
				return nil
			}

			destPath := filepath.Join(destDir, relPath)
			if d.IsDir() {
				return os.MkdirAll(destPath, 0o750)
			}

			data, err := fs.ReadFile(mgr.Registry().FS(), path)
			if err != nil {
				return err
			}
			return os.WriteFile(destPath, data, 0o600)
		})
		if err != nil {
			return fmt.Errorf("copying template: %w", err)
		}

		fmt.Printf("Exported template %s to %s/\n", templateName, destDir)
		return nil
	},
}

var templatesDeleteCmd = &cobra.Command{
	Use:   "delete <template>",
	Short: "Remove a local template override",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		templateName := args[0]
		templateDir := filepath.Join(cfg.TemplatesDir, templateName)
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			return fmt.Errorf("local template %s not found", templateName)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes && !cliutil.AskConfirmation(fmt.Sprintf("Delete local template %q?", templateName)) {
			fmt.Println("Cancelled.")
			return nil
		}

		if err := os.RemoveAll(templateDir); err != nil {
			return fmt.Errorf("removing template: %w", err)
		}

		fmt.Printf("Removed local template %s.\n", templateName)
		return nil
	},
}

var templatesPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the local templates directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(cfg.TemplatesDir)
		return nil
	},
}

var templatesNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold a new app template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		destDir := filepath.Join(cfg.TemplatesDir, name)
		dockerfile, _ := cmd.Flags().GetBool("dockerfile")

		if err := template.Scaffold(destDir, name, template.ScaffoldOptions{Dockerfile: dockerfile}); err != nil {
			return err
		}

		fmt.Printf("Created template scaffold at %s/\n", destDir)
		fmt.Println("\nFiles created:")
		fmt.Println("  app.yaml                  - Template metadata (edit this first)")
		fmt.Println("  docker-compose.yml.tmpl   - Docker Compose template")
		fmt.Println("  .env.tmpl                 - Environment variables template")
		if dockerfile {
			fmt.Println("  Dockerfile                - Container build instructions")
			fmt.Println("  .dockerignore             - Files excluded from build context")
		}
		fmt.Println("\nNext steps:")
		fmt.Printf("  1. Edit %s/app.yaml\n", destDir)
		fmt.Printf("  2. Edit %s/docker-compose.yml.tmpl\n", destDir)
		fmt.Printf("  3. aradeploy deploy %s\n", name)
		return nil
	},
}

var templatesLintCmd = &cobra.Command{
	Use:   "lint [template]",
	Short: "Lint templates for best practices",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr, err := newManager()
		if err != nil {
			return err
		}

		linter := lint.NewLinter(mgr.Registry())

		var findings []lint.Finding
		if len(args) > 0 {
			findings = linter.LintTemplate(args[0])
		} else {
			result := linter.LintAll()
			findings = result.Findings
		}

		if jsonOutput {
			summary := lint.CountSummary(findings)
			return outputJSON(struct {
				Findings []lint.Finding `json:"findings"`
				Summary  lint.Summary   `json:"summary"`
			}{findings, summary})
		}

		if len(findings) == 0 {
			fmt.Println("No issues found.")
			return nil
		}

		// Group by template
		byTemplate := make(map[string][]lint.Finding)
		var templateOrder []string
		for _, f := range findings {
			if _, ok := byTemplate[f.Template]; !ok {
				templateOrder = append(templateOrder, f.Template)
			}
			byTemplate[f.Template] = append(byTemplate[f.Template], f)
		}
		sort.Strings(templateOrder)

		for _, tmpl := range templateOrder {
			fmt.Printf("\n=== %s ===\n", tmpl)
			for _, f := range byTemplate[tmpl] {
				fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Check, f.Message)
			}
		}

		summary := lint.CountSummary(findings)
		fmt.Printf("\n%d error(s), %d warning(s), %d info(s)\n", summary.Errors, summary.Warnings, summary.Infos)
		return nil
	},
}
