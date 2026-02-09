package collector

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/fabienpiette/docker-stats-exporter/internal/metrics"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// DockerClient defines the Docker API methods needed by the container collector.
type DockerClient interface {
	ListContainers(ctx context.Context) ([]docker.Container, error)
	GetContainerStats(ctx context.Context, id string) (*docker.Stats, error)
}

// ContainerCollector implements prometheus.Collector for container metrics.
type ContainerCollector struct {
	client        DockerClient
	filter        *docker.Filter
	cache         *StatsCache
	timeout       time.Duration
	maxConcurrent int

	scrapeErrors int64
	mu           sync.Mutex
}

// NewContainerCollector creates a new container metrics collector.
func NewContainerCollector(client DockerClient, filter *docker.Filter, cache *StatsCache, cfg *config.Config) *ContainerCollector {
	return &ContainerCollector{
		client:        client,
		filter:        filter,
		cache:         cache,
		timeout:       cfg.Collection.Timeout,
		maxConcurrent: cfg.Performance.MaxConcurrent,
	}
}

// Describe sends all metric descriptors.
func (c *ContainerCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range metrics.AllContainerDescs() {
		ch <- d
	}
	ch <- metrics.ExporterScrapeDuration
	ch <- metrics.ExporterScrapeErrors
}

// Collect fetches container stats and emits Prometheus metrics.
func (c *ContainerCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	var scrapeErrors int64

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// 1. List all containers
	containers, err := c.client.ListContainers(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to list containers")
		scrapeErrors++
		c.emitSelfMetrics(ch, start, scrapeErrors)
		return
	}

	// 2. Apply filters
	var filtered []docker.Container
	for i := range containers {
		if c.filter.Match(&containers[i]) {
			filtered = append(filtered, containers[i])
		}
	}

	// 3. Collect stats concurrently with bounded worker pool
	type result struct {
		container docker.Container
		stats     *docker.Stats
		err       error
	}

	results := make([]result, len(filtered))
	var wg sync.WaitGroup
	sem := make(chan struct{}, c.maxConcurrent)

	for i, ctr := range filtered {
		// For stopped containers, emit state metrics only (no stats available)
		if ctr.State != "running" {
			results[i] = result{container: ctr}
			continue
		}

		// Check cache
		if cached, ok := c.cache.Get(ctr.ID); ok {
			results[i] = result{container: ctr, stats: cached}
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // acquire slot

		go func(idx int, container docker.Container) {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			stats, err := c.client.GetContainerStats(ctx, container.ID)
			results[idx] = result{container: container, stats: stats, err: err}
			if err == nil {
				c.cache.Set(container.ID, stats)
			}
		}(i, ctr)
	}
	wg.Wait()

	// 4. Emit metrics for each container
	now := time.Now()
	for _, r := range results {
		if r.err != nil {
			log.WithError(r.err).WithField("container", r.container.Name).Warn("Failed to get container stats, skipping")
			scrapeErrors++
			continue
		}

		labels := docker.ExtractLabels(&r.container)
		lv := labels.Values()

		// Always emit state metrics for all containers
		c.emitStateMetrics(ch, &r.container, lv, now)

		// Only emit resource metrics for running containers with stats
		if r.stats != nil {
			c.emitMemoryMetrics(ch, r.stats, lv)
			c.emitCPUMetrics(ch, r.stats, lv)
			c.emitNetworkMetrics(ch, r.stats, lv)
			c.emitBlockIOMetrics(ch, r.stats, lv)
			c.emitPIDsMetrics(ch, r.stats, lv)
		}
	}

	// Evict stale cache entries
	c.cache.EvictStale()

	c.emitSelfMetrics(ch, start, scrapeErrors)
}

func (c *ContainerCollector) emitMemoryMetrics(ch chan<- prometheus.Metric, s *docker.Stats, lv []string) {
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryUsage, prometheus.GaugeValue, float64(s.MemoryUsage), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryLimit, prometheus.GaugeValue, float64(s.MemoryLimit), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryCache, prometheus.GaugeValue, float64(s.MemoryCache), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryRSS, prometheus.GaugeValue, float64(s.MemoryRSS), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemorySwap, prometheus.GaugeValue, float64(s.MemorySwap), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryWorkingSet, prometheus.GaugeValue, float64(s.MemoryWorkingSet), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.MemoryFailcnt, prometheus.GaugeValue, float64(s.MemoryFailcnt), lv...))
}

func (c *ContainerCollector) emitCPUMetrics(ch chan<- prometheus.Metric, s *docker.Stats, lv []string) {
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.CPUUsageTotal, prometheus.CounterValue, float64(s.CPUUsageTotal)*metrics.NanosecondsToSeconds, lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.CPUUsageSystem, prometheus.CounterValue, float64(s.CPUUsageSystem)*metrics.NanosecondsToSeconds, lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.CPUUsageUser, prometheus.CounterValue, float64(s.CPUUsageUser)*metrics.NanosecondsToSeconds, lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.CPUThrottledPeriods, prometheus.CounterValue, float64(s.CPUThrottledPeriods), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.CPUThrottledTime, prometheus.CounterValue, float64(s.CPUThrottledTime)*metrics.NanosecondsToSeconds, lv...))
}

func (c *ContainerCollector) emitNetworkMetrics(ch chan<- prometheus.Metric, s *docker.Stats, lv []string) {
	for iface, net := range s.Networks {
		nlv := append(lv, iface)
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkRxBytes, prometheus.CounterValue, float64(net.RxBytes), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkTxBytes, prometheus.CounterValue, float64(net.TxBytes), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkRxPackets, prometheus.CounterValue, float64(net.RxPackets), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkTxPackets, prometheus.CounterValue, float64(net.TxPackets), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkRxErrors, prometheus.CounterValue, float64(net.RxErrors), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkTxErrors, prometheus.CounterValue, float64(net.TxErrors), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkRxDropped, prometheus.CounterValue, float64(net.RxDropped), nlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.NetworkTxDropped, prometheus.CounterValue, float64(net.TxDropped), nlv...))
	}
}

func (c *ContainerCollector) emitBlockIOMetrics(ch chan<- prometheus.Metric, s *docker.Stats, lv []string) {
	for device, bio := range s.BlockIO {
		dlv := append(lv, device)
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.FSReadBytes, prometheus.CounterValue, float64(bio.ReadBytes), dlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.FSWriteBytes, prometheus.CounterValue, float64(bio.WriteBytes), dlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.FSReadOps, prometheus.CounterValue, float64(bio.ReadOps), dlv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.FSWriteOps, prometheus.CounterValue, float64(bio.WriteOps), dlv...))
	}
}

func (c *ContainerCollector) emitPIDsMetrics(ch chan<- prometheus.Metric, s *docker.Stats, lv []string) {
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.PIDsCurrent, prometheus.GaugeValue, float64(s.PIDsCurrent), lv...))
}

func (c *ContainerCollector) emitStateMetrics(ch chan<- prometheus.Metric, ctr *docker.Container, lv []string, now time.Time) {
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerLastSeen, prometheus.GaugeValue, float64(now.Unix()), lv...))

	if !ctr.StartedAt.IsZero() {
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerStartTime, prometheus.GaugeValue, float64(ctr.StartedAt.Unix()), lv...))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerUptime, prometheus.GaugeValue, now.Sub(ctr.StartedAt).Seconds(), lv...))
	}

	// container_info: extra labels for informational purposes
	infoLV := append(lv, ctr.ID[:12], ctr.Status, ctr.Health, ctr.StartedAt.Format(time.RFC3339))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerInfo, prometheus.GaugeValue, 1, infoLV...))

	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerHealthStatus, prometheus.GaugeValue, metrics.HealthStatusToFloat(ctr.Health), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerRestartCount, prometheus.GaugeValue, float64(ctr.RestartCount), lv...))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ContainerExitCode, prometheus.GaugeValue, float64(ctr.ExitCode), lv...))
}

func (c *ContainerCollector) emitSelfMetrics(ch chan<- prometheus.Metric, start time.Time, errors int64) {
	duration := time.Since(start).Seconds()
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterScrapeDuration, prometheus.GaugeValue, duration, "container"))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterScrapeErrors, prometheus.CounterValue, float64(errors), "container"))
}
