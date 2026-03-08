package registry

import "github.com/jdillenberger/arastack/pkg/systemd"

// DoctorResult holds the result of a single doctor check.
type DoctorResult struct {
	Name           string
	Installed      bool
	Version        string
	Optional       bool
	InstallCommand string
}

// Tool defines a managed arastack tool.
type Tool struct {
	Name        string // "peer-scanner"
	BinaryName  string // "peer-scanner"
	ServiceName string // "peer-scanner"
	Description string // systemd unit description
	ExecArgs    string // "run"
	Port        int    // 7120 (0 if none)
	ConfigPath  string // "/etc/komphost/..." ("" if none)
	Order       int    // dependency order for setup

	ServiceConfig systemd.ServiceConfig

	DoctorCheck func() ([]DoctorResult, error) // wraps each tool's doctor.CheckAll()
	DoctorFix   func(DoctorResult) error        // wraps each tool's doctor.Fix()
	SetupFunc   func() error                     // custom setup (nil = doctor fix + service install)
}

// All returns all registered tools in dependency order.
func All() []Tool {
	return tools
}

// ByName returns a tool by name, or nil if not found.
func ByName(name string) *Tool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}

// Names returns all tool names in dependency order.
func Names() []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return names
}
