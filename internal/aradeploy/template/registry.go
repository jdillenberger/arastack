package template

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Registry manages available app templates.
type Registry struct {
	apps   map[string]*AppMeta
	tmplFS fs.FS
}

// NewRegistry scans the given filesystem and loads all app metadata.
func NewRegistry(tmplFS fs.FS) (*Registry, error) {
	r := &Registry{
		apps:   make(map[string]*AppMeta),
		tmplFS: tmplFS,
	}

	entries, err := fs.ReadDir(tmplFS, ".")
	if err != nil {
		return nil, fmt.Errorf("reading templates dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip internal directories (partials, shared templates)
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		appYAML := entry.Name() + "/app.yaml"
		data, err := fs.ReadFile(tmplFS, appYAML)
		if err != nil {
			continue // skip dirs without app.yaml
		}

		var meta AppMeta
		if err := yaml.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", appYAML, err)
		}

		if meta.Name == "" {
			meta.Name = entry.Name()
		}
		r.apps[meta.Name] = &meta
	}

	return r, nil
}

// Get returns the metadata for a specific app template.
func (r *Registry) Get(name string) (*AppMeta, bool) {
	meta, ok := r.apps[name]
	return meta, ok
}

// List returns all available app template names, sorted.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.apps))
	for name := range r.apps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// All returns all app metadata, sorted by name.
func (r *Registry) All() []*AppMeta {
	names := r.List()
	metas := make([]*AppMeta, len(names))
	for i, name := range names {
		metas[i] = r.apps[name]
	}
	return metas
}

// FS returns the template filesystem.
func (r *Registry) FS() fs.FS {
	return r.tmplFS
}
