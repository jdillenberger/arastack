package deploy

import (
	"fmt"
	"regexp"
)

var validAppName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ValidateAppName checks that an app name only contains safe characters.
func ValidateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name must not be empty")
	}
	if !validAppName.MatchString(name) {
		return fmt.Errorf("app name %q is invalid: must start with a letter or digit and contain only lowercase letters, digits, dots, hyphens, and underscores", name)
	}
	return nil
}

// DeployOptions holds parameters for a deploy operation.
type DeployOptions struct {
	Values  map[string]string
	Code    map[string]string // maps "slot[/name]" to source path/URL (optionally with #branch)
	DryRun  bool
	Confirm bool // if true, skip confirmation prompt
}

// UpgradeOptions holds parameters for an upgrade operation.
type UpgradeOptions struct {
	PatchOnly bool
	DryRun    bool
	Confirm   bool
}

// ManagerConfig holds configuration for the deployment manager.
// When aradeploy is used as a library, the caller constructs this directly.
type ManagerConfig struct {
	Hostname string
	AppsDir  string
	DataDir  string
	CodeDir  string

	Network NetworkConfig
	Docker  DockerConfig
	Routing RoutingConfig
}

// NetworkConfig holds network-related configuration.
type NetworkConfig struct {
	Domain  string
	WebPort int
}

// DockerConfig holds Docker-related configuration.
type DockerConfig struct {
	Runtime        string
	ComposeCommand string
	DefaultNetwork string
}

// RoutingConfig holds routing-related configuration.
type RoutingConfig struct {
	Enabled        bool
	Provider       string
	Domain         string
	DomainPriority []string
	HTTPS          HTTPSConfig
}

// HTTPSConfig holds HTTPS-related configuration.
type HTTPSConfig struct {
	Enabled   bool
	AcmeEmail string
}

// AppDir returns the directory for a deployed app.
func (c *ManagerConfig) AppDir(appName string) string {
	return c.AppsDir + "/" + appName
}

// DataPath returns the data directory for an app.
func (c *ManagerConfig) DataPath(appName string) string {
	return c.DataDir + "/" + appName
}

// CodePath returns the code directory for an app.
func (c *ManagerConfig) CodePath(appName string) string {
	return c.CodeDir + "/" + appName
}

// RoutingDomain returns the domain used for routing.
func (c *ManagerConfig) RoutingDomain() string {
	if c.Routing.Domain != "" {
		return c.Routing.Domain
	}
	return c.Hostname + "." + c.Network.Domain
}
