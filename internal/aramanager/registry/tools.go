package registry

import (
	"github.com/jdillenberger/arastack/internal/araalert/config"
	araalertdoc "github.com/jdillenberger/arastack/internal/araalert/doctor"
	arabackupdoc "github.com/jdillenberger/arastack/internal/arabackup/doctor"
	dashcfg "github.com/jdillenberger/arastack/internal/aradashboard/config"
	aradashboarddoc "github.com/jdillenberger/arastack/internal/aradashboard/doctor"
	aradeploydoc "github.com/jdillenberger/arastack/internal/aradeploy/doctor"
	aramdnsdoc "github.com/jdillenberger/arastack/internal/aramdns/doctor"
	aranotifydoc "github.com/jdillenberger/arastack/internal/aranotify/doctor"
	arascannerdoc "github.com/jdillenberger/arastack/internal/arascanner/doctor"
	"github.com/jdillenberger/arastack/pkg/doctor"
	"github.com/jdillenberger/arastack/pkg/systemd"
)

// tools is the list of all managed arastack tools in dependency order.
var tools = []Tool{
	{
		Name:        "arascanner",
		BinaryName:  "arascanner",
		ServiceName: "arascanner",
		Description: "arascanner - Homelab peer discovery daemon",
		ExecArgs:    "run",
		Port:        7120,
		Order:       1,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "arascanner",
			ServiceName: "arascanner",
			Description: "arascanner - Homelab peer discovery daemon",
			ExecArgs:    "run",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			return arascannerdoc.CheckAll("/var/lib/arascanner"), nil
		},
		DoctorFix: func(r doctor.CheckResult) error {
			return arascannerdoc.Fix(r, "/var/lib/arascanner")
		},
	},
	{
		Name:        "aranotify",
		BinaryName:  "aranotify",
		ServiceName: "aranotify",
		Description: "aranotify - Notification delivery service",
		ExecArgs:    "run",
		Port:        7140,
		ConfigPath:  "/etc/arastack/config/aranotify.yaml",
		Order:       2,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "aranotify",
			ServiceName: "aranotify",
			Description: "aranotify - Notification delivery service",
			ExecArgs:    "run",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			return aranotifydoc.CheckAll(), nil
		},
		DoctorFix: aranotifydoc.Fix,
	},
	{
		Name:        "araalert",
		BinaryName:  "araalert",
		ServiceName: "araalert",
		Description: "araalert - Alert rule evaluation daemon",
		ExecArgs:    "run",
		Port:        7150,
		ConfigPath:  "/etc/arastack/config/araalert.yaml",
		Order:       3,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "araalert",
			ServiceName: "araalert",
			Description: "araalert - Alert rule evaluation daemon",
			ExecArgs:    "run",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			cfg, err := config.Load("")
			if err != nil {
				return nil, err
			}
			return araalertdoc.CheckAll(cfg), nil
		},
		DoctorFix: func(r doctor.CheckResult) error {
			cfg, err := config.Load("")
			if err != nil {
				return err
			}
			return araalertdoc.Fix(r, cfg)
		},
	},
	{
		Name:        "arabackup",
		BinaryName:  "arabackup",
		ServiceName: "arabackup",
		Description: "arabackup backup service",
		ExecArgs:    "run",
		Port:        7160,
		ConfigPath:  "/etc/arastack/config/arabackup.yaml",
		Order:       4,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "arabackup",
			ServiceName: "arabackup",
			Description: "arabackup backup service",
			ExecArgs:    "run",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			return arabackupdoc.CheckAll(), nil
		},
		DoctorFix: arabackupdoc.Fix,
	},
	{
		Name:        "aradashboard",
		BinaryName:  "aradashboard",
		ServiceName: "aradashboard",
		Description: "aradashboard - Homelab dashboard",
		ExecArgs:    "run",
		Port:        8420,
		ConfigPath:  "/etc/arastack/config/aradashboard.yaml",
		Order:       5,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "aradashboard",
			ServiceName: "aradashboard",
			Description: "aradashboard - Homelab dashboard",
			ExecArgs:    "run",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			cfg, err := dashcfg.Load("")
			if err != nil {
				return nil, err
			}
			return aradashboarddoc.CheckAll(cfg), nil
		},
		DoctorFix: func(r doctor.CheckResult) error {
			cfg, err := dashcfg.Load("")
			if err != nil {
				return err
			}
			return aradashboarddoc.Fix(r, cfg)
		},
	},
	{
		Name:        "aradeploy",
		BinaryName:  "aradeploy",
		ServiceName: "aradeploy",
		Description: "aradeploy deployment manager",
		ExecArgs:    "update --all",
		Port:        0,
		ConfigPath:  "/etc/arastack/config/aradeploy.yaml",
		Order:       6,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "aradeploy",
			ServiceName: "aradeploy",
			Description: "aradeploy deployment manager",
			ExecArgs:    "update --all",
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			return aradeploydoc.CheckAll(), nil
		},
		DoctorFix: aradeploydoc.Fix,
	},
	{
		Name:        "aramdns",
		BinaryName:  "aramdns",
		ServiceName: "aramdns",
		Description: "aramdns - Traefik Docker mDNS publisher",
		ExecArgs:    "run",
		Port:        0,
		Order:       7,
		ServiceConfig: systemd.ServiceConfig{
			BinaryName:  "aramdns",
			ServiceName: "aramdns",
			Description: "aramdns - Traefik Docker mDNS publisher",
			ExecArgs:    "run",
			After:       []string{"docker.service"},
			Group:       "arastack",
		},
		DoctorCheck: func() ([]doctor.CheckResult, error) {
			return aramdnsdoc.CheckAll(), nil
		},
		DoctorFix: aramdnsdoc.Fix,
	},
}
