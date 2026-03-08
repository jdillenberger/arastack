package template

// RoutingMeta holds per-template routing configuration.
type RoutingMeta struct {
	Enabled       *bool  `yaml:"enabled"`
	Subdomain     string `yaml:"subdomain"`
	Hostname      string `yaml:"hostname"`
	ContainerPort int    `yaml:"container_port"`
	Websocket     bool   `yaml:"websocket"`
	KeepPorts     *bool  `yaml:"keep_ports"`
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
	HealthCheck    *HealthCheck    `yaml:"health_check"`
	Backup         *BackupMeta     `yaml:"backup"`
	Requirements   *Requirements   `yaml:"requirements"`
	PostDeployInfo *PostDeployInfo `yaml:"post_deploy_info"`
	Hooks          *HooksMeta      `yaml:"hooks"`
	Routing        *RoutingMeta    `yaml:"routing"`
	RequiresBuild  bool            `yaml:"requires_build"`
	LintIgnore     []string        `yaml:"lint_ignore"`
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

// Value describes a configurable value in a template.
type Value struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Required    bool   `yaml:"required"`
	Secret      bool   `yaml:"secret"`
	AutoGen     string `yaml:"auto_gen"`
}

// HealthCheck defines an HTTP health check endpoint.
type HealthCheck struct {
	URL      string `yaml:"url"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
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
	Type    string `yaml:"type"`
	URL     string `yaml:"url"`
	Method  string `yaml:"method"`
	Body    string `yaml:"body"`
	Command string `yaml:"command"`
}
