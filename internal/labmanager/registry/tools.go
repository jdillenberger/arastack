package registry

import (
	"github.com/jdillenberger/arastack/internal/labalert/config"
	labalertdoc "github.com/jdillenberger/arastack/internal/labalert/doctor"
	labbackupdoc "github.com/jdillenberger/arastack/internal/labbackup/doctor"
	dashcfg "github.com/jdillenberger/arastack/internal/labdashboard/config"
	labdashboarddoc "github.com/jdillenberger/arastack/internal/labdashboard/doctor"
	labdeploydoc "github.com/jdillenberger/arastack/internal/labdeploy/doctor"
	labnotifydoc "github.com/jdillenberger/arastack/internal/labnotify/doctor"
	peerscannerdoc "github.com/jdillenberger/arastack/internal/peerscanner/doctor"
	traefikmdnsdoc "github.com/jdillenberger/arastack/internal/traefikmdns/doctor"
	"github.com/jdillenberger/arastack/pkg/systemd"
)

// tools is the list of all managed arastack tools in dependency order.
var tools = []Tool{
	{
		Name:        "peer-scanner",
		BinaryName:  "peer-scanner",
		ServiceName: "peer-scanner",
		Description: "peer-scanner - Homelab peer discovery daemon",
		ExecArgs:    "run",
		Port:        7120,
		Order:       1,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "peer-scanner",
			ServiceName: "peer-scanner",
			Description: "peer-scanner - Homelab peer discovery daemon",
			ExecArgs:    "run",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			results := peerscannerdoc.CheckAll("/var/lib/peer-scanner")
			return convertPeerscannerResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			return peerscannerdoc.Fix(peerscannerdoc.CheckResult{
				Name:           r.Name,
				Installed:      r.Installed,
				Version:        r.Version,
				InstallCommand: r.InstallCommand,
			}, "/var/lib/peer-scanner")
		},
	},
	{
		Name:        "labnotify",
		BinaryName:  "labnotify",
		ServiceName: "labnotify",
		Description: "labnotify - Notification delivery service for komphost",
		ExecArgs:    "run",
		Port:        7140,
		ConfigPath:  "/etc/komphost/notify.yaml",
		Order:       2,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "labnotify",
			ServiceName: "labnotify",
			Description: "labnotify - Notification delivery service for komphost",
			ExecArgs:    "run",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			results := labnotifydoc.CheckAll()
			return convertLabnotifyResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			return labnotifydoc.Fix(labnotifydoc.CheckResult{
				Name:      r.Name,
				Installed: r.Installed,
				Version:   r.Version,
			})
		},
	},
	{
		Name:        "labalert",
		BinaryName:  "labalert",
		ServiceName: "labalert",
		Description: "labalert - Alert rule evaluation daemon",
		ExecArgs:    "run",
		Port:        7150,
		ConfigPath:  "/etc/komphost/alert.yaml",
		Order:       3,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "labalert",
			ServiceName: "labalert",
			Description: "labalert - Alert rule evaluation daemon",
			ExecArgs:    "run",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			cfg, err := config.Load("")
			if err != nil {
				return nil, err
			}
			results := labalertdoc.CheckAll(cfg)
			return convertLabalertResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}
			return labalertdoc.Fix(labalertdoc.CheckResult{
				Name:      r.Name,
				Installed: r.Installed,
				Version:   r.Version,
			}, cfg)
		},
	},
	{
		Name:        "labbackup",
		BinaryName:  "labbackup",
		ServiceName: "labbackup",
		Description: "labbackup backup service",
		ExecArgs:    "daemon",
		Port:        7160,
		ConfigPath:  "/etc/komphost/labbackup.yaml",
		Order:       4,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "labbackup",
			ServiceName: "labbackup",
			Description: "labbackup backup service",
			ExecArgs:    "daemon",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			results := labbackupdoc.CheckAll()
			return convertLabbackupResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			return labbackupdoc.Fix(labbackupdoc.CheckResult{
				Name:      r.Name,
				Installed: r.Installed,
				Version:   r.Version,
			})
		},
	},
	{
		Name:        "labdashboard",
		BinaryName:  "labdashboard",
		ServiceName: "labdashboard",
		Description: "labdashboard - Homelab dashboard",
		ExecArgs:    "run",
		Port:        8420,
		ConfigPath:  "/etc/komphost/labdashboard.yaml",
		Order:       5,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "labdashboard",
			ServiceName: "labdashboard",
			Description: "labdashboard - Homelab dashboard",
			ExecArgs:    "run",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			cfg, err := dashcfg.Load("")
			if err != nil {
				return nil, err
			}
			results := labdashboarddoc.CheckAll(cfg)
			return convertLabdashboardResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			cfg, err := dashcfg.Load("")
			if err != nil {
				return err
			}
			return labdashboarddoc.Fix(labdashboarddoc.CheckResult{
				Name:      r.Name,
				Installed: r.Installed,
				Version:   r.Version,
				Optional:  r.Optional,
			}, cfg)
		},
	},
	{
		Name:        "labdeploy",
		BinaryName:  "labdeploy",
		ServiceName: "labdeploy",
		Description: "labdeploy deployment manager",
		ExecArgs:    "update --all",
		Port:        0,
		ConfigPath:  "/etc/komphost/labdeploy.yaml",
		Order:       6,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "labdeploy",
			ServiceName: "labdeploy",
			Description: "labdeploy deployment manager",
			ExecArgs:    "update --all",
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			results := labdeploydoc.CheckAll()
			return convertLabdeployResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			return labdeploydoc.Fix(labdeploydoc.CheckResult{
				Name:           r.Name,
				Installed:      r.Installed,
				Version:        r.Version,
				InstallCommand: r.InstallCommand,
			})
		},
	},
	{
		Name:        "traefik-mdns",
		BinaryName:  "traefik-mdns",
		ServiceName: "traefik-mdns",
		Description: "traefik-mdns - Traefik Docker mDNS publisher",
		ExecArgs:    "run",
		Port:        0,
		Order:       7,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "traefik-mdns",
			ServiceName: "traefik-mdns",
			Description: "traefik-mdns - Traefik Docker mDNS publisher",
			ExecArgs:    "run",
			After:       []string{"docker.service"},
		},
		DoctorCheck: func() ([]DoctorResult, error) {
			results := traefikmdnsdoc.CheckAll()
			return convertTraefikmdnsResults(results), nil
		},
		DoctorFix: func(r DoctorResult) error {
			return traefikmdnsdoc.Fix(traefikmdnsdoc.CheckResult{
				Name:           r.Name,
				Installed:      r.Installed,
				Version:        r.Version,
				InstallCommand: r.InstallCommand,
			})
		},
	},
}

func convertPeerscannerResults(results []peerscannerdoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version, InstallCommand: r.InstallCommand}
	}
	return out
}

func convertLabnotifyResults(results []labnotifydoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version}
	}
	return out
}

func convertLabalertResults(results []labalertdoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version}
	}
	return out
}

func convertLabbackupResults(results []labbackupdoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version}
	}
	return out
}

func convertLabdashboardResults(results []labdashboarddoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version, Optional: r.Optional}
	}
	return out
}

func convertLabdeployResults(results []labdeploydoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version, InstallCommand: r.InstallCommand}
	}
	return out
}

func convertTraefikmdnsResults(results []traefikmdnsdoc.CheckResult) []DoctorResult {
	out := make([]DoctorResult, len(results))
	for i, r := range results {
		out[i] = DoctorResult{Name: r.Name, Installed: r.Installed, Version: r.Version, InstallCommand: r.InstallCommand}
	}
	return out
}
