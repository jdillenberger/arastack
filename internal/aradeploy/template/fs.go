package template

import (
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"
)

// OverlayFS combines two filesystems where the upper layer takes precedence
// at the top-level directory boundary.
type OverlayFS struct {
	upper     fs.FS
	lower     fs.FS
	upperDirs map[string]bool
	lowerDirs map[string]bool
}

var (
	_ fs.FS         = (*OverlayFS)(nil)
	_ fs.ReadDirFS  = (*OverlayFS)(nil)
	_ fs.ReadFileFS = (*OverlayFS)(nil)
)

// NewOverlayFS creates an OverlayFS. upper takes precedence over lower.
func NewOverlayFS(lower, upper fs.FS) *OverlayFS {
	o := &OverlayFS{
		upper:     upper,
		lower:     lower,
		upperDirs: make(map[string]bool),
		lowerDirs: make(map[string]bool),
	}

	if entries, err := fs.ReadDir(lower, "."); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				o.lowerDirs[e.Name()] = true
			}
		}
	}

	if upper != nil {
		if entries, err := fs.ReadDir(upper, "."); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					o.upperDirs[e.Name()] = true
				}
			}
		}
	}

	return o
}

func topDir(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

func (o *OverlayFS) fsFor(name string) fs.FS {
	if o.upper != nil && o.upperDirs[topDir(name)] {
		return o.upper
	}
	return o.lower
}

// Open implements fs.FS.
func (o *OverlayFS) Open(name string) (fs.File, error) {
	return o.fsFor(name).Open(name)
}

// ReadFile implements fs.ReadFileFS.
func (o *OverlayFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(o.fsFor(name), name)
}

// ReadDir implements fs.ReadDirFS.
func (o *OverlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return o.mergedRootEntries()
	}
	return fs.ReadDir(o.fsFor(name), name)
}

func (o *OverlayFS) mergedRootEntries() ([]fs.DirEntry, error) {
	lowerEntries, err := fs.ReadDir(o.lower, ".")
	if err != nil {
		return nil, err
	}

	if o.upper == nil {
		return lowerEntries, nil
	}

	seen := make(map[string]bool)
	var merged []fs.DirEntry

	if upperEntries, err := fs.ReadDir(o.upper, "."); err == nil {
		for _, e := range upperEntries {
			seen[e.Name()] = true
			merged = append(merged, e)
		}
	}

	for _, e := range lowerEntries {
		if !seen[e.Name()] {
			merged = append(merged, e)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Name() < merged[j].Name()
	})

	return merged, nil
}

// Lower returns the lower (base) filesystem layer.
func (o *OverlayFS) Lower() fs.FS {
	return o.lower
}

// Source returns where a template comes from: "repo:<name>", "local", "override".
func (o *OverlayFS) Source(templateName string) string {
	inUpper := o.upper != nil && o.upperDirs[templateName]
	inLower := o.lowerDirs[templateName]

	switch {
	case inUpper && inLower:
		return "override"
	case inUpper:
		return "local"
	default:
		return "repo"
	}
}

// MergedFS combines multiple directory-based filesystems into a single fs.FS.
type MergedFS struct {
	layers []fs.FS
	dirs   map[string]int
}

var (
	_ fs.FS         = (*MergedFS)(nil)
	_ fs.ReadDirFS  = (*MergedFS)(nil)
	_ fs.ReadFileFS = (*MergedFS)(nil)
)

// NewMergedFS creates a MergedFS from a list of directory paths.
func NewMergedFS(dirs []string) *MergedFS {
	m := &MergedFS{
		dirs: make(map[string]int),
	}

	for i, dir := range dirs {
		dirFS := os.DirFS(dir)
		m.layers = append(m.layers, dirFS)

		entries, err := fs.ReadDir(dirFS, ".")
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, exists := m.dirs[e.Name()]; !exists {
				m.dirs[e.Name()] = i
			}
		}
	}

	return m
}

// RepoIndex returns which layer owns a template (-1 if not found).
func (m *MergedFS) RepoIndex(templateName string) int {
	idx, ok := m.dirs[templateName]
	if !ok {
		return -1
	}
	return idx
}

// Open implements fs.FS.
func (m *MergedFS) Open(name string) (fs.File, error) {
	if name == "." {
		return &mergedDir{m: m}, nil
	}
	top := topDir(name)
	idx, ok := m.dirs[top]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return m.layers[idx].Open(name)
}

// ReadFile implements fs.ReadFileFS.
func (m *MergedFS) ReadFile(name string) ([]byte, error) {
	top := topDir(name)
	idx, ok := m.dirs[top]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return fs.ReadFile(m.layers[idx], name)
}

// ReadDir implements fs.ReadDirFS.
func (m *MergedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == "." {
		return m.rootEntries()
	}
	top := topDir(name)
	idx, ok := m.dirs[top]
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	return fs.ReadDir(m.layers[idx], name)
}

func (m *MergedFS) rootEntries() ([]fs.DirEntry, error) {
	entries := make([]fs.DirEntry, 0, len(m.dirs))
	for name, idx := range m.dirs {
		info, err := fs.Stat(m.layers[idx], name)
		if err != nil {
			continue
		}
		entries = append(entries, fs.FileInfoToDirEntry(info))
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

type mergedDir struct {
	m *MergedFS
}

func (d *mergedDir) Stat() (fs.FileInfo, error) {
	return &dirInfo{name: "."}, nil
}

func (d *mergedDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: ".", Err: fs.ErrInvalid}
}

func (d *mergedDir) Close() error { return nil }

type dirInfo struct {
	name string
}

func (di *dirInfo) Name() string       { return di.name }
func (di *dirInfo) Size() int64        { return 0 }
func (di *dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o755 }
func (di *dirInfo) ModTime() time.Time { return time.Time{} }
func (di *dirInfo) IsDir() bool        { return true }
func (di *dirInfo) Sys() any           { return nil }

// BuildTemplateFS creates the template filesystem by layering:
// repos (base) → local (highest priority).
// No embedded FS — all templates come from git repos + local overrides.
func BuildTemplateFS(repoDirs []string, localDir string) fs.FS {
	if len(repoDirs) == 0 && localDir == "" {
		return os.DirFS(".")
	}

	// Start with merged repos as the base.
	var base fs.FS
	if len(repoDirs) > 0 {
		base = NewMergedFS(repoDirs)
	}

	if localDir == "" {
		if base != nil {
			return base
		}
		return os.DirFS(".")
	}

	// Auto-create the local templates directory.
	if err := os.MkdirAll(localDir, 0o750); err != nil {
		if base != nil {
			return base
		}
		return os.DirFS(".")
	}

	info, err := os.Stat(localDir)
	if err != nil || !info.IsDir() {
		if base != nil {
			return base
		}
		return os.DirFS(".")
	}

	localFS := os.DirFS(localDir)
	if base == nil {
		return localFS
	}

	return NewOverlayFS(base, localFS)
}

// ResolveSource determines where a template comes from in the layered filesystem.
func ResolveSource(fsys fs.FS, templateName string, repoNames []string) string {
	outer, ok := fsys.(*OverlayFS)
	if !ok {
		// Single-layer: check if it's a MergedFS (repos only)
		if merged, ok := fsys.(*MergedFS); ok {
			idx := merged.RepoIndex(templateName)
			if idx >= 0 && idx < len(repoNames) {
				return "repo:" + repoNames[idx]
			}
			return "repo"
		}
		return "local"
	}

	inLocal := outer.upperDirs[templateName]
	inner := outer.lower

	if merged, ok := inner.(*MergedFS); ok {
		_, inRepos := merged.dirs[templateName]

		var repoSource string
		if inRepos {
			idx := merged.RepoIndex(templateName)
			if idx >= 0 && idx < len(repoNames) {
				repoSource = "repo:" + repoNames[idx]
			}
		}

		switch {
		case inLocal && inRepos:
			return "override"
		case inLocal:
			return "local"
		case inRepos:
			if repoSource != "" {
				return repoSource
			}
			return "repo"
		default:
			return "local"
		}
	}

	// No repo layer — simple two-layer case.
	inInner := outer.lowerDirs[templateName]
	switch {
	case inLocal && inInner:
		return "override"
	case inLocal:
		return "local"
	default:
		return "repo"
	}
}
