package metrics

import "github.com/prometheus/client_golang/prometheus"

// Standard label sets used across metric definitions.
var (
	containerLabelNames = []string{"container_name", "compose_service", "compose_project", "image"}
	networkLabelNames   = append(containerLabelNames, "interface")
	blockIOLabelNames   = append(containerLabelNames, "device")
	infoLabelNames      = append(containerLabelNames, "container_id", "status", "health_status", "created")
)

// --- Memory metrics ---

var (
	MemoryUsage = prometheus.NewDesc(
		"container_memory_usage_bytes",
		"Current memory usage in bytes.",
		containerLabelNames, nil,
	)
	MemoryLimit = prometheus.NewDesc(
		"container_memory_limit_bytes",
		"Memory limit in bytes.",
		containerLabelNames, nil,
	)
	MemoryCache = prometheus.NewDesc(
		"container_memory_cache_bytes",
		"Memory used for cache in bytes.",
		containerLabelNames, nil,
	)
	MemoryRSS = prometheus.NewDesc(
		"container_memory_rss_bytes",
		"Resident set size in bytes.",
		containerLabelNames, nil,
	)
	MemorySwap = prometheus.NewDesc(
		"container_memory_swap_bytes",
		"Swap usage in bytes.",
		containerLabelNames, nil,
	)
	MemoryWorkingSet = prometheus.NewDesc(
		"container_memory_working_set_bytes",
		"Working set size in bytes (usage minus inactive file).",
		containerLabelNames, nil,
	)
	MemoryFailcnt = prometheus.NewDesc(
		"container_memory_failcnt",
		"Number of times memory limit was hit.",
		containerLabelNames, nil,
	)
)

// --- CPU metrics (counters in nanoseconds, converted to seconds) ---

var (
	CPUUsageTotal = prometheus.NewDesc(
		"container_cpu_usage_seconds_total",
		"Total CPU time consumed in seconds.",
		containerLabelNames, nil,
	)
	CPUUsageSystem = prometheus.NewDesc(
		"container_cpu_system_seconds_total",
		"CPU time in kernel mode in seconds.",
		containerLabelNames, nil,
	)
	CPUUsageUser = prometheus.NewDesc(
		"container_cpu_user_seconds_total",
		"CPU time in user mode in seconds.",
		containerLabelNames, nil,
	)
	CPUThrottledPeriods = prometheus.NewDesc(
		"container_cpu_throttling_periods_total",
		"Number of periods with throttling active.",
		containerLabelNames, nil,
	)
	CPUThrottledTime = prometheus.NewDesc(
		"container_cpu_throttled_seconds_total",
		"Total time throttled in seconds.",
		containerLabelNames, nil,
	)
)

// --- Network metrics ---

var (
	NetworkRxBytes = prometheus.NewDesc(
		"container_network_receive_bytes_total",
		"Total bytes received.",
		networkLabelNames, nil,
	)
	NetworkTxBytes = prometheus.NewDesc(
		"container_network_transmit_bytes_total",
		"Total bytes transmitted.",
		networkLabelNames, nil,
	)
	NetworkRxPackets = prometheus.NewDesc(
		"container_network_receive_packets_total",
		"Total packets received.",
		networkLabelNames, nil,
	)
	NetworkTxPackets = prometheus.NewDesc(
		"container_network_transmit_packets_total",
		"Total packets transmitted.",
		networkLabelNames, nil,
	)
	NetworkRxErrors = prometheus.NewDesc(
		"container_network_receive_errors_total",
		"Total receive errors.",
		networkLabelNames, nil,
	)
	NetworkTxErrors = prometheus.NewDesc(
		"container_network_transmit_errors_total",
		"Total transmit errors.",
		networkLabelNames, nil,
	)
	NetworkRxDropped = prometheus.NewDesc(
		"container_network_receive_dropped_total",
		"Total received packets dropped.",
		networkLabelNames, nil,
	)
	NetworkTxDropped = prometheus.NewDesc(
		"container_network_transmit_dropped_total",
		"Total transmitted packets dropped.",
		networkLabelNames, nil,
	)
)

// --- Block I/O metrics ---

var (
	FSReadBytes = prometheus.NewDesc(
		"container_fs_reads_bytes_total",
		"Total bytes read from disk.",
		blockIOLabelNames, nil,
	)
	FSWriteBytes = prometheus.NewDesc(
		"container_fs_writes_bytes_total",
		"Total bytes written to disk.",
		blockIOLabelNames, nil,
	)
	FSReadOps = prometheus.NewDesc(
		"container_fs_reads_total",
		"Total read operations.",
		blockIOLabelNames, nil,
	)
	FSWriteOps = prometheus.NewDesc(
		"container_fs_writes_total",
		"Total write operations.",
		blockIOLabelNames, nil,
	)
)

// --- Container state metrics ---

var (
	ContainerLastSeen = prometheus.NewDesc(
		"container_last_seen",
		"Timestamp when container was last seen.",
		containerLabelNames, nil,
	)
	ContainerStartTime = prometheus.NewDesc(
		"container_start_time_seconds",
		"Container start time as Unix timestamp.",
		containerLabelNames, nil,
	)
	ContainerUptime = prometheus.NewDesc(
		"container_uptime_seconds",
		"Container uptime in seconds.",
		containerLabelNames, nil,
	)
	ContainerInfo = prometheus.NewDesc(
		"container_info",
		"Container information (value always 1).",
		infoLabelNames, nil,
	)
	ContainerHealthStatus = prometheus.NewDesc(
		"container_health_status",
		"Container health status (0=none, 1=starting, 2=healthy, 3=unhealthy).",
		containerLabelNames, nil,
	)
	ContainerRestartCount = prometheus.NewDesc(
		"container_restart_count",
		"Number of times container has been restarted.",
		containerLabelNames, nil,
	)
	ContainerExitCode = prometheus.NewDesc(
		"container_exit_code",
		"Last exit code of the container.",
		containerLabelNames, nil,
	)
)

// --- System metrics ---

var (
	DockerContainersTotal = prometheus.NewDesc(
		"docker_containers_total",
		"Total number of containers.",
		[]string{"state"}, nil,
	)
	DockerImagesTotal = prometheus.NewDesc(
		"docker_images_total",
		"Total number of images.",
		nil, nil,
	)
	DockerVolumesTotal = prometheus.NewDesc(
		"docker_volumes_total",
		"Total number of volumes.",
		nil, nil,
	)
	DockerNetworksTotal = prometheus.NewDesc(
		"docker_networks_total",
		"Total number of networks.",
		nil, nil,
	)
)

// --- Exporter self-metrics ---

var (
	ExporterBuildInfo = prometheus.NewDesc(
		"exporter_build_info",
		"Exporter build information.",
		[]string{"version", "commit", "build_date", "go_version"}, nil,
	)
	ExporterScrapeDuration = prometheus.NewDesc(
		"exporter_scrape_duration_seconds",
		"Time spent collecting metrics.",
		[]string{"collector"}, nil,
	)
	ExporterScrapeErrors = prometheus.NewDesc(
		"exporter_scrape_errors_total",
		"Total number of errors during collection.",
		[]string{"collector"}, nil,
	)
	ExporterUp = prometheus.NewDesc(
		"exporter_up",
		"Whether the exporter is up.",
		nil, nil,
	)
)

// AllContainerDescs returns all metric descriptors for the container collector.
func AllContainerDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		MemoryUsage, MemoryLimit, MemoryCache, MemoryRSS, MemorySwap, MemoryWorkingSet, MemoryFailcnt,
		CPUUsageTotal, CPUUsageSystem, CPUUsageUser, CPUThrottledPeriods, CPUThrottledTime,
		NetworkRxBytes, NetworkTxBytes, NetworkRxPackets, NetworkTxPackets,
		NetworkRxErrors, NetworkTxErrors, NetworkRxDropped, NetworkTxDropped,
		FSReadBytes, FSWriteBytes, FSReadOps, FSWriteOps,
		ContainerLastSeen, ContainerStartTime, ContainerUptime, ContainerInfo,
		ContainerHealthStatus, ContainerRestartCount, ContainerExitCode,
	}
}

// AllSystemDescs returns all metric descriptors for the system collector.
func AllSystemDescs() []*prometheus.Desc {
	return []*prometheus.Desc{
		DockerContainersTotal, DockerImagesTotal, DockerVolumesTotal, DockerNetworksTotal,
		ExporterBuildInfo, ExporterUp, ExporterScrapeDuration, ExporterScrapeErrors,
	}
}
