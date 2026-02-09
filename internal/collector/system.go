package collector

import (
	"context"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/fabienpiette/docker-stats-exporter/internal/metrics"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// Build information, set via ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// SystemCollector implements prometheus.Collector for Docker system-level metrics.
type SystemCollector struct {
	client  *docker.Client
	timeout time.Duration
}

// NewSystemCollector creates a new system metrics collector.
func NewSystemCollector(client *docker.Client, cfg *config.Config) *SystemCollector {
	return &SystemCollector{
		client:  client,
		timeout: cfg.Collection.Timeout,
	}
}

// Describe sends all system metric descriptors.
func (c *SystemCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range metrics.AllSystemDescs() {
		ch <- d
	}
}

// Collect fetches Docker system info and emits metrics.
func (c *SystemCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	var scrapeErrors int64

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	info, err := c.client.GetSystemInfo(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to get Docker system info")
		scrapeErrors++
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterUp, prometheus.GaugeValue, 0))
	} else {
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterUp, prometheus.GaugeValue, 1))

		// Container counts by state
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerContainersTotal, prometheus.GaugeValue, float64(info.ContainersRunning), "running"))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerContainersTotal, prometheus.GaugeValue, float64(info.ContainersPaused), "paused"))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerContainersTotal, prometheus.GaugeValue, float64(info.ContainersStopped), "stopped"))

		// Resource counts
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerImagesTotal, prometheus.GaugeValue, float64(info.Images)))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerVolumesTotal, prometheus.GaugeValue, float64(info.Volumes)))
		metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.DockerNetworksTotal, prometheus.GaugeValue, float64(info.Networks)))
	}

	// Build info (always emitted)
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(
		metrics.ExporterBuildInfo, prometheus.GaugeValue, 1,
		Version, Commit, BuildDate, runtime.Version(),
	))

	// Scrape self-metrics
	duration := time.Since(start).Seconds()
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterScrapeDuration, prometheus.GaugeValue, duration, "system"))
	metrics.SendSafe(ch, metrics.SafeNewConstMetric(metrics.ExporterScrapeErrors, prometheus.CounterValue, float64(scrapeErrors), "system"))
}
