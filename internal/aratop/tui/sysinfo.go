package tui

import (
	"fmt"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// SysInfo holds system-level resource information.
type SysInfo struct {
	Hostname string
	OS       string
	Arch     string
	Uptime   string
	CPUCores int

	CPUPercent  float64
	MemTotal    uint64
	MemUsed     uint64
	MemPercent  float64
	DiskTotal   uint64
	DiskUsed    uint64
	DiskPercent float64
}

func collectSysInfo() SysInfo {
	var s SysInfo
	s.Hostname, _ = os.Hostname()
	s.OS = runtime.GOOS
	s.Arch = runtime.GOARCH
	s.CPUCores = runtime.NumCPU()

	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		s.CPUPercent = pcts[0]
	}

	if vmem, err := mem.VirtualMemory(); err == nil {
		s.MemTotal = vmem.Total
		s.MemUsed = vmem.Used
		s.MemPercent = vmem.UsedPercent
	}

	if d, err := disk.Usage("/"); err == nil {
		s.DiskTotal = d.Total
		s.DiskUsed = d.Used
		s.DiskPercent = d.UsedPercent
	}

	if uptime, err := host.Uptime(); err == nil {
		days := uptime / 86400
		hours := (uptime % 86400) / 3600
		mins := (uptime % 3600) / 60
		if days > 0 {
			s.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
		} else {
			s.Uptime = fmt.Sprintf("%dh %dm", hours, mins)
		}
	} else {
		s.Uptime = "N/A"
	}

	return s
}
