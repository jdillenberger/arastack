package stats

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemStats holds system statistics for display.
type SystemStats struct {
	CPUPercent  float64
	MemPercent  float64
	MemUsedGB   string
	MemTotalGB  string
	DiskPercent float64
	DiskUsedGB  string
	DiskTotalGB string
	Uptime      string
}

// cpuCache holds a background-updated CPU percentage so Collect() is non-blocking.
var cpuCache struct {
	mu      sync.RWMutex
	percent float64
	once    sync.Once
}

func initCPUCache() {
	// Seed with a non-blocking zero-interval call (returns since-last or zero).
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		cpuCache.mu.Lock()
		cpuCache.percent = pcts[0]
		cpuCache.mu.Unlock()
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
				cpuCache.mu.Lock()
				cpuCache.percent = pcts[0]
				cpuCache.mu.Unlock()
			}
		}
	}()
}

func cachedCPUPercent() float64 {
	cpuCache.once.Do(initCPUCache)
	cpuCache.mu.RLock()
	defer cpuCache.mu.RUnlock()
	return cpuCache.percent
}

// Collect gathers current system stats.
func Collect() SystemStats {
	stats := SystemStats{}

	stats.CPUPercent = cachedCPUPercent()

	vmem, err := mem.VirtualMemory()
	if err == nil {
		stats.MemPercent = vmem.UsedPercent
		stats.MemUsedGB = fmt.Sprintf("%.1f", float64(vmem.Used)/(1024*1024*1024))
		stats.MemTotalGB = fmt.Sprintf("%.1f", float64(vmem.Total)/(1024*1024*1024))
	}

	diskStat, err := disk.Usage("/")
	if err == nil {
		stats.DiskPercent = diskStat.UsedPercent
		stats.DiskUsedGB = fmt.Sprintf("%.1f", float64(diskStat.Used)/(1024*1024*1024))
		stats.DiskTotalGB = fmt.Sprintf("%.1f", float64(diskStat.Total)/(1024*1024*1024))
	}

	uptime, err := host.Uptime()
	if err == nil {
		days := uptime / 86400
		hours := (uptime % 86400) / 3600
		mins := (uptime % 3600) / 60
		if days > 0 {
			stats.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
		} else {
			stats.Uptime = fmt.Sprintf("%dh %dm", hours, mins)
		}
	} else {
		stats.Uptime = "N/A"
	}

	return stats
}

// JSON returns stats as a JSON-friendly map.
func JSON() map[string]interface{} {
	s := Collect()
	hostname, _ := os.Hostname()
	return map[string]interface{}{
		"hostname":      hostname,
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"cpu_percent":   s.CPUPercent,
		"mem_percent":   s.MemPercent,
		"mem_used_gb":   s.MemUsedGB,
		"mem_total_gb":  s.MemTotalGB,
		"disk_percent":  s.DiskPercent,
		"disk_used_gb":  s.DiskUsedGB,
		"disk_total_gb": s.DiskTotalGB,
		"uptime":        s.Uptime,
	}
}
