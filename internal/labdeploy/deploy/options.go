package deploy

// DeployOptions holds parameters for a deploy operation.
type DeployOptions struct {
	Values  map[string]string
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
// When labdeploy is used as a library, the caller constructs this directly.
type ManagerConfig struct {
	Hostname string
	AppsDir  string
	DataDir  string

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
	Enabled  bool
	Provider string
	Domain   string
	HTTPS    HTTPSConfig
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

// RoutingDomain returns the domain used for routing.
func (c *ManagerConfig) RoutingDomain() string {
	if c.Routing.Domain != "" {
		return c.Routing.Domain
	}
	return c.Hostname + "." + c.Network.Domain
}
