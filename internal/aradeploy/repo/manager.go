package repo

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/pkg/executil"
)

// UpdateResult holds information about a repo update operation.
type UpdateResult struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	OldCommit string   `json:"old_commit"`
	NewCommit string   `json:"new_commit"`
	Changed   []string `json:"changed,omitempty"` // template names that changed
	UpToDate  bool     `json:"up_to_date"`
}

// Repo represents a git repository added as a template source.
type Repo struct {
	Name      string    `yaml:"name"`
	URL       string    `yaml:"url"`
	Ref       string    `yaml:"ref,omitempty"`
	AddedAt   time.Time `yaml:"added_at"`
	UpdatedAt time.Time `yaml:"updated_at,omitempty"`
}

// Manifest holds the list of tracked repos.
type Manifest struct {
	Repos []Repo `yaml:"repos"`
}

// Manager handles repo clone/pull operations and manifest persistence.
type Manager struct {
	reposDir     string
	manifestPath string
	runner       *executil.Runner
}

// NewManager creates a Manager.
func NewManager(reposDir, manifestPath string, runner *executil.Runner) *Manager {
	return &Manager{
		reposDir:     reposDir,
		manifestPath: manifestPath,
		runner:       runner,
	}
}

// Load reads the manifest from disk. Returns an empty manifest if the file
// does not exist.
func (m *Manager) Load() (*Manifest, error) {
	data, err := os.ReadFile(m.manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{}, nil
		}
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &manifest, nil
}

// Save writes the manifest to disk.
func (m *Manager) Save(manifest *Manifest) error {
	if err := os.MkdirAll(filepath.Dir(m.manifestPath), 0o750); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	return os.WriteFile(m.manifestPath, data, 0o600)
}

// Add clones a git repository and records it in the manifest.
func (m *Manager) Add(url, name, ref string) (*Repo, error) {
	if name == "" {
		name = NameFromURL(url)
	}
	if name == "" {
		return nil, fmt.Errorf("cannot derive repo name from URL %q; use --name", url)
	}

	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}

	for _, r := range manifest.Repos {
		if r.Name == name {
			return nil, fmt.Errorf("repo %q already exists", name)
		}
	}

	dest := filepath.Join(m.reposDir, name)
	if err := os.MkdirAll(m.reposDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating repos directory: %w", err)
	}

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, dest)

	if _, err := m.runner.Run("git", args...); err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}

	repo := Repo{
		Name:    name,
		URL:     url,
		Ref:     ref,
		AddedAt: time.Now().UTC().Truncate(time.Second),
	}
	manifest.Repos = append(manifest.Repos, repo)

	if err := m.Save(manifest); err != nil {
		return nil, err
	}
	return &repo, nil
}

// Remove deletes a cloned repo and removes it from the manifest.
func (m *Manager) Remove(name string) error {
	manifest, err := m.Load()
	if err != nil {
		return err
	}

	found := false
	repos := manifest.Repos[:0]
	for _, r := range manifest.Repos {
		if r.Name == name {
			found = true
			continue
		}
		repos = append(repos, r)
	}
	if !found {
		return fmt.Errorf("repo %q not found", name)
	}
	manifest.Repos = repos

	dest := filepath.Join(m.reposDir, name)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("removing repo directory: %w", err)
	}

	return m.Save(manifest)
}

// Update pulls the latest changes for a single repo.
func (m *Manager) Update(name string) (*UpdateResult, error) {
	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}

	idx := -1
	for i, r := range manifest.Repos {
		if r.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("repo %q not found", name)
	}

	dest := filepath.Join(m.reposDir, name)

	// Capture commit before pull.
	oldRes, err := m.runner.Run("git", "-C", dest, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	oldCommit := strings.TrimSpace(oldRes.Stdout)

	if _, err := m.runner.Run("git", "-C", dest, "pull"); err != nil {
		return nil, fmt.Errorf("git pull: %w", err)
	}

	// Capture commit after pull.
	newRes, err := m.runner.Run("git", "-C", dest, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse: %w", err)
	}
	newCommit := strings.TrimSpace(newRes.Stdout)

	result := &UpdateResult{
		Name:      name,
		Path:      dest,
		OldCommit: oldCommit,
		NewCommit: newCommit,
		UpToDate:  oldCommit == newCommit,
	}

	// Find which templates were affected.
	if !result.UpToDate {
		diffRes, err := m.runner.Run("git", "-C", dest, "diff", "--name-only", oldCommit+".."+newCommit)
		if err == nil {
			result.Changed = changedTemplates(diffRes.Stdout)
		}
	}

	manifest.Repos[idx].UpdatedAt = time.Now().UTC().Truncate(time.Second)
	return result, m.Save(manifest)
}

// UpdateAll pulls the latest changes for all repos.
func (m *Manager) UpdateAll() ([]UpdateResult, error) {
	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}
	var results []UpdateResult
	for _, r := range manifest.Repos {
		res, err := m.Update(r.Name)
		if err != nil {
			return results, err
		}
		results = append(results, *res)
	}
	return results, nil
}

// changedTemplates extracts unique top-level directory names from a
// newline-separated list of changed file paths (git diff --name-only output).
func changedTemplates(diffOutput string) []string {
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(diffOutput), "\n") {
		if line == "" {
			continue
		}
		// Top-level directory is the template name.
		dir := strings.SplitN(line, "/", 2)[0]
		// Skip dotfiles (e.g. .github, .gitignore).
		if strings.HasPrefix(dir, ".") {
			continue
		}
		seen[dir] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List returns all tracked repos.
func (m *Manager) List() ([]Repo, error) {
	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}
	return manifest.Repos, nil
}

// TemplateDirs returns the filesystem paths for all repos in manifest order.
func (m *Manager) TemplateDirs() ([]string, error) {
	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(manifest.Repos))
	for _, r := range manifest.Repos {
		dir := filepath.Join(m.reposDir, r.Name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

// RepoNames returns the names of repos that exist on disk, in the same order
// as TemplateDirs. This ensures the index alignment that MergedFS and
// ResolveSource depend on.
func (m *Manager) RepoNames() ([]string, error) {
	manifest, err := m.Load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(manifest.Repos))
	for _, r := range manifest.Repos {
		dir := filepath.Join(m.reposDir, r.Name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			names = append(names, r.Name)
		}
	}
	return names, nil
}

// DefaultRepoURL is the default template repository.
const DefaultRepoURL = "https://github.com/jdillenberger/arastack-templates.git"

// EnsureDefaults adds the default template repo when no repos are configured.
func (m *Manager) EnsureDefaults() error {
	manifest, err := m.Load()
	if err != nil {
		return err
	}
	if len(manifest.Repos) > 0 {
		return nil
	}
	_, err = m.Add(DefaultRepoURL, "", "")
	return err
}

// NameFromURL derives a repo name from a git URL by stripping .git and taking
// the last path segment.
func NameFromURL(url string) string {
	if i := strings.LastIndex(url, ":"); i >= 0 && !strings.Contains(url, "://") {
		url = url[i+1:]
	}

	base := path.Base(url)
	return strings.TrimSuffix(base, ".git")
}
