package template

// RoutingMeta holds per-template routing configuration.
type RoutingMeta struct {
	Enabled       *bool  `yaml:"enabled"`
	Subdomain     string `yaml:"subdomain"`
	Hostname      string `yaml:"hostname"`
	ContainerPort int    `yaml:"container_port"`
	Websocket     bool   `yaml:"websocket"`
	KeepPorts     *bool  `yaml:"keep_ports"`
	Auth          string `yaml:"auth"` // "none" (default), "optional", "required"
}

// AuthMode returns the effective auth mode, defaulting to "none".
func (r *RoutingMeta) AuthMode() string {
	if r == nil || r.Auth == "" {
		return "none"
	}
	return r.Auth
}

// AppMeta holds metadata from a template's app.yaml.
type AppMeta struct {
	Name           string          `yaml:"name"`
	Description    string          `yaml:"description"`
	Category       string          `yaml:"category"`
	Version        string          `yaml:"version"`
	Ports          []PortMapping   `yaml:"ports"`
	Volumes        []VolumeMapping `yaml:"volumes"`
	Values         []Value         `yaml:"values"`
	Dependencies   []string        `yaml:"dependencies"`
	Backup         *BackupMeta     `yaml:"backup"`
	Requirements   *Requirements   `yaml:"requirements"`
	PostDeployInfo *PostDeployInfo `yaml:"post_deploy_info"`
	Hooks          *HooksMeta      `yaml:"hooks"`
	Routing        *RoutingMeta    `yaml:"routing"`
	Code           *CodeMeta       `yaml:"code"`
	RequiresBuild  bool            `yaml:"requires_build"`
	LintIgnore     []string        `yaml:"lint_ignore"`
}

// CodeMeta defines code deployment slots for a template.
type CodeMeta struct {
	Slots []CodeSlot `yaml:"slots"`
}

// CodeSlot describes a location where user code can be mounted or built into a container.
type CodeSlot struct {
	Name        string `yaml:"name"`      // e.g. "themes", "src"
	Container   string `yaml:"container"` // e.g. "/var/www/html/wp-content/themes/{name}"
	Description string `yaml:"description"`
	Inject      string `yaml:"inject"`   // "build" or "volume" (default: "volume")
	Multiple    bool   `yaml:"multiple"` // true = many items (WordPress plugins/themes)
	Required    bool   `yaml:"required"` // must be provided at deploy time
	Service     string `yaml:"service"`  // target compose service (optional, defaults to primary)
}

// InjectMode returns the effective injection mode, defaulting to "volume".
func (s *CodeSlot) InjectMode() string {
	if s.Inject == "build" {
		return "build"
	}
	return "volume"
}

// PortMapping describes a port exposed by the app.
type PortMapping struct {
	Host        int    `yaml:"host"`
	Container   int    `yaml:"container"`
	Protocol    string `yaml:"protocol"`
	Description string `yaml:"description"`
	ValueName   string `yaml:"value_name"`
}

// VolumeMapping describes a volume mount.
type VolumeMapping struct {
	Name        string `yaml:"name"`
	Container   string `yaml:"container"`
	Description string `yaml:"description"`
}

// PortValueNameSet returns the set of value names that are referenced by port
// mappings. This is the authoritative way to identify which values represent ports.
func (m *AppMeta) PortValueNameSet() map[string]bool {
	set := make(map[string]bool)
	for _, p := range m.Ports {
		if p.ValueName != "" {
			set[p.ValueName] = true
		}
	}
	return set
}

// Value describes a configurable value in a template.
type Value struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Required    bool   `yaml:"required"`
	Secret      bool   `yaml:"secret"`
	AutoGen     string `yaml:"auto_gen"`
	Validation  string `yaml:"validation"`
	UserFacing  bool   `yaml:"user_facing"` // show to user after deploy (e.g. admin tokens, passwords)
}

// BackupMeta defines backup configuration for an app.
type BackupMeta struct {
	Paths    []string `yaml:"paths"`
	PreHook  string   `yaml:"pre_hook"`
	PostHook string   `yaml:"post_hook"`
}

// Requirements defines system requirements for an app.
type Requirements struct {
	MinRAM  string   `yaml:"min_ram"`
	MinDisk string   `yaml:"min_disk"`
	Arch    []string `yaml:"arch"`
}

// PostDeployInfo holds information displayed after a successful deployment.
type PostDeployInfo struct {
	AccessURL   string   `yaml:"access_url"`
	Credentials string   `yaml:"credentials"`
	Notes       []string `yaml:"notes"`
}

// HooksMeta defines lifecycle hooks for an app.
type HooksMeta struct {
	PostDeploy []Hook `yaml:"post_deploy"`
	PreRemove  []Hook `yaml:"pre_remove"`
}

// Hook defines a single lifecycle hook action.
type Hook struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Method   string `yaml:"method"`
	Body     string `yaml:"body"`
	Command  string `yaml:"command"`
	Required bool   `yaml:"required"`
}
