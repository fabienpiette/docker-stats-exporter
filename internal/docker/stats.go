package docker

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
)

// Stats holds parsed container statistics.
type Stats struct {
	// Memory
	MemoryUsage      uint64
	MemoryLimit      uint64
	MemoryCache      uint64
	MemoryRSS        uint64
	MemorySwap       uint64
	MemoryWorkingSet uint64
	MemoryFailcnt    uint64

	// CPU (raw nanosecond counters)
	CPUUsageTotal       uint64
	CPUUsageSystem      uint64
	CPUUsageUser        uint64
	CPUThrottledPeriods uint64
	CPUThrottledTime    uint64
	OnlineCPUs          uint32

	// Network per interface
	Networks map[string]NetworkStats

	// Block I/O per device
	BlockIO map[string]BlockIOStats

	// Container state
	ContainerID  string
	Name         string
	Image        string
	Labels       map[string]string
	Status       string
	Health       string
	StartedAt    time.Time
	RestartCount int
	ExitCode     int

	Timestamp time.Time
}

// NetworkStats holds per-interface network counters.
type NetworkStats struct {
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  uint64
	TxErrors  uint64
	RxDropped uint64
	TxDropped uint64
}

// BlockIOStats holds per-device I/O counters.
type BlockIOStats struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
}

// Container holds basic container info from a list call.
type Container struct {
	ID           string
	Name         string
	Image        string
	Labels       map[string]string
	Status       string
	State        string
	Health       string
	StartedAt    time.Time
	RestartCount int
	ExitCode     int
}

// SystemInfo holds Docker daemon info.
type SystemInfo struct {
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
	Volumes           int
	Networks          int
	ServerVersion     string
}

// ParseDockerStats converts raw Docker API responses into our Stats struct.
func ParseDockerStats(statsJSON *types.StatsJSON, containerJSON *types.ContainerJSON) *Stats {
	s := &Stats{
		Timestamp: statsJSON.Read,
	}

	// Container identity
	s.ContainerID = containerJSON.ID
	s.Name = trimLeadingSlash(containerJSON.Name)
	s.Image = containerJSON.Config.Image
	s.Labels = containerJSON.Config.Labels
	s.Status = containerJSON.State.Status
	s.RestartCount = containerJSON.RestartCount
	s.ExitCode = containerJSON.State.ExitCode

	if containerJSON.State.Health != nil {
		s.Health = containerJSON.State.Health.Status
	}

	if containerJSON.State.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, containerJSON.State.StartedAt); err == nil {
			s.StartedAt = t
		}
	}

	// Memory
	parseMemoryStats(s, &statsJSON.MemoryStats)

	// CPU
	parseCPUStats(s, &statsJSON.CPUStats)

	// Network
	s.Networks = make(map[string]NetworkStats, len(statsJSON.Networks))
	for iface, net := range statsJSON.Networks {
		s.Networks[iface] = NetworkStats{
			RxBytes:   net.RxBytes,
			TxBytes:   net.TxBytes,
			RxPackets: net.RxPackets,
			TxPackets: net.TxPackets,
			RxErrors:  net.RxErrors,
			TxErrors:  net.TxErrors,
			RxDropped: net.RxDropped,
			TxDropped: net.TxDropped,
		}
	}

	// Block I/O
	s.BlockIO = parseBlockIOStats(&statsJSON.BlkioStats)

	return s
}

func parseMemoryStats(s *Stats, mem *containertypes.MemoryStats) {
	s.MemoryUsage = mem.Usage
	s.MemoryLimit = mem.Limit
	s.MemoryFailcnt = mem.Failcnt

	// These fields live in the Stats map and differ between cgroup v1 and v2.
	// Docker SDK normalizes most of this, but cache/rss/swap need extraction.
	if v, ok := mem.Stats["cache"]; ok {
		s.MemoryCache = v
	}
	if v, ok := mem.Stats["rss"]; ok {
		s.MemoryRSS = v
	} else if v, ok := mem.Stats["anon"]; ok {
		// cgroup v2 uses "anon" instead of "rss"
		s.MemoryRSS = v
	}
	if v, ok := mem.Stats["swap"]; ok {
		s.MemorySwap = v
	}

	// Working set = usage - inactive_file (what the OOM killer looks at)
	inactiveFile := uint64(0)
	if v, ok := mem.Stats["inactive_file"]; ok {
		inactiveFile = v
	} else if v, ok := mem.Stats["total_inactive_file"]; ok {
		inactiveFile = v
	}
	if mem.Usage > inactiveFile {
		s.MemoryWorkingSet = mem.Usage - inactiveFile
	} else {
		s.MemoryWorkingSet = mem.Usage
	}
}

func parseCPUStats(s *Stats, cpu *containertypes.CPUStats) {
	s.CPUUsageTotal = cpu.CPUUsage.TotalUsage
	s.CPUUsageSystem = cpu.CPUUsage.UsageInKernelmode
	s.CPUUsageUser = cpu.CPUUsage.UsageInUsermode
	s.CPUThrottledPeriods = cpu.ThrottlingData.ThrottledPeriods
	s.CPUThrottledTime = cpu.ThrottlingData.ThrottledTime
	s.OnlineCPUs = cpu.OnlineCPUs
}

func parseBlockIOStats(bio *containertypes.BlkioStats) map[string]BlockIOStats {
	devices := make(map[string]BlockIOStats)

	for _, entry := range bio.IoServiceBytesRecursive {
		key := deviceKey(entry.Major, entry.Minor)
		d := devices[key]
		switch entry.Op {
		case "Read", "read":
			d.ReadBytes = entry.Value
		case "Write", "write":
			d.WriteBytes = entry.Value
		}
		devices[key] = d
	}

	for _, entry := range bio.IoServicedRecursive {
		key := deviceKey(entry.Major, entry.Minor)
		d := devices[key]
		switch entry.Op {
		case "Read", "read":
			d.ReadOps = entry.Value
		case "Write", "write":
			d.WriteOps = entry.Value
		}
		devices[key] = d
	}

	return devices
}

func deviceKey(major, minor uint64) string {
	return fmt.Sprintf("%d:%d", major, minor)
}

func trimLeadingSlash(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
