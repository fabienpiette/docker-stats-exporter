package collector

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// mockDockerClient implements DockerClient for testing.
type mockDockerClient struct {
	containers []docker.Container
	stats      map[string]*docker.Stats
	listErr    error
	statsErr   map[string]error
}

func (m *mockDockerClient) ListContainers(_ context.Context) ([]docker.Container, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.containers, nil
}

func (m *mockDockerClient) GetContainerStats(_ context.Context, id string) (*docker.Stats, error) {
	if m.statsErr != nil {
		if err, ok := m.statsErr[id]; ok {
			return nil, err
		}
	}
	if s, ok := m.stats[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("container %s not found", id)
}

func newTestConfig() *config.Config {
	return &config.Config{
		Collection: config.CollectionConfig{
			Timeout: 5 * time.Second,
		},
		Performance: config.PerformanceConfig{
			MaxConcurrent: 4,
		},
	}
}

func newTestFilter() *docker.Filter {
	f, _ := docker.NewFilter(config.FiltersConfig{})
	return f
}

func collectMetrics(c prometheus.Collector) []prometheus.Metric {
	ch := make(chan prometheus.Metric, 100)
	go func() {
		c.Collect(ch)
		close(ch)
	}()
	var collected []prometheus.Metric
	for m := range ch {
		collected = append(collected, m)
	}
	return collected
}

func findMetric(metrics []prometheus.Metric, name string) []prometheus.Metric {
	var found []prometheus.Metric
	for _, m := range metrics {
		d := &dto.Metric{}
		_ = m.Write(d)
		desc := m.Desc().String()
		if strings.Contains(desc, "\""+name+"\"") {
			found = append(found, m)
		}
	}
	return found
}

func TestCollect_RunningContainer(t *testing.T) {
	mock := &mockDockerClient{
		containers: []docker.Container{
			{
				ID:    "abc123def456abc123def456",
				Name:  "web",
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					"com.docker.compose.service": "web",
					"com.docker.compose.project": "myapp",
				},
				StartedAt: time.Now().Add(-1 * time.Hour),
			},
		},
		stats: map[string]*docker.Stats{
			"abc123def456abc123def456": {
				MemoryUsage:      104857600,
				MemoryLimit:      536870912,
				MemoryCache:      20971520,
				MemoryRSS:        73400320,
				MemoryWorkingSet: 99614720,
				CPUUsageTotal:    500000000000,
				CPUUsageSystem:   100000000000,
				CPUUsageUser:     400000000000,
				PIDsCurrent:      25,
				Networks: map[string]docker.NetworkStats{
					"eth0": {RxBytes: 1024, TxBytes: 2048},
				},
				BlockIO: map[string]docker.BlockIOStats{
					"8:0": {ReadBytes: 4096, WriteBytes: 8192},
				},
			},
		},
	}

	cache := NewStatsCache(30*time.Second, false)
	collector := NewContainerCollector(mock, newTestFilter(), cache, newTestConfig())
	metrics := collectMetrics(collector)

	// Should emit memory, CPU, network, block I/O, PIDs, and state metrics
	assert.NotEmpty(t, metrics)

	memUsage := findMetric(metrics, "container_memory_usage_bytes")
	assert.Len(t, memUsage, 1, "expected one memory usage metric")

	cpuTotal := findMetric(metrics, "container_cpu_usage_seconds_total")
	assert.Len(t, cpuTotal, 1, "expected one CPU total metric")

	pids := findMetric(metrics, "container_pids_current")
	assert.Len(t, pids, 1, "expected one PIDs metric")

	netRx := findMetric(metrics, "container_network_receive_bytes_total")
	assert.Len(t, netRx, 1, "expected one network RX metric (one interface)")

	fsRead := findMetric(metrics, "container_fs_reads_bytes_total")
	assert.Len(t, fsRead, 1, "expected one block I/O read metric")

	uptime := findMetric(metrics, "container_uptime_seconds")
	assert.Len(t, uptime, 1, "expected one uptime metric")
}

func TestCollect_StoppedContainer(t *testing.T) {
	mock := &mockDockerClient{
		containers: []docker.Container{
			{
				ID:    "stopped1aabbccddeeff",
				Name:  "old-app",
				Image: "redis:7",
				State: "exited",
				Labels: map[string]string{},
				ExitCode: 137,
			},
		},
		stats: map[string]*docker.Stats{},
	}

	cache := NewStatsCache(30*time.Second, false)
	collector := NewContainerCollector(mock, newTestFilter(), cache, newTestConfig())
	metrics := collectMetrics(collector)

	// Stopped containers emit state metrics but no resource metrics
	assert.NotEmpty(t, metrics)

	lastSeen := findMetric(metrics, "container_last_seen")
	assert.Len(t, lastSeen, 1, "expected last_seen for stopped container")

	exitCode := findMetric(metrics, "container_exit_code")
	assert.Len(t, exitCode, 1, "expected exit_code for stopped container")

	// Should NOT emit resource metrics
	memUsage := findMetric(metrics, "container_memory_usage_bytes")
	assert.Empty(t, memUsage, "stopped containers should not emit memory metrics")

	cpuTotal := findMetric(metrics, "container_cpu_usage_seconds_total")
	assert.Empty(t, cpuTotal, "stopped containers should not emit CPU metrics")
}

func TestCollect_ListError(t *testing.T) {
	mock := &mockDockerClient{
		listErr: fmt.Errorf("connection refused"),
	}

	cache := NewStatsCache(30*time.Second, false)
	collector := NewContainerCollector(mock, newTestFilter(), cache, newTestConfig())
	metrics := collectMetrics(collector)

	// Should only emit self-metrics (scrape duration + errors)
	scrapeErrors := findMetric(metrics, "exporter_scrape_errors_total")
	assert.Len(t, scrapeErrors, 1, "expected scrape errors metric on list failure")
}

func TestCollect_StatsError(t *testing.T) {
	mock := &mockDockerClient{
		containers: []docker.Container{
			{ID: "fail1aabbccddeeff0011", Name: "fail-app", Image: "app:latest", State: "running", Labels: map[string]string{}},
		},
		stats:    map[string]*docker.Stats{},
		statsErr: map[string]error{"fail1aabbccddeeff0011": fmt.Errorf("timeout")},
	}

	cache := NewStatsCache(30*time.Second, false)
	collector := NewContainerCollector(mock, newTestFilter(), cache, newTestConfig())
	metrics := collectMetrics(collector)

	// Should still emit self-metrics even when stats fail
	assert.NotEmpty(t, metrics)

	// No resource metrics for the failed container
	memUsage := findMetric(metrics, "container_memory_usage_bytes")
	assert.Empty(t, memUsage)
}

func TestCollect_CacheHit(t *testing.T) {
	cachedStats := &docker.Stats{
		MemoryUsage: 999,
		MemoryLimit: 1000,
		Networks:    map[string]docker.NetworkStats{},
		BlockIO:     map[string]docker.BlockIOStats{},
	}

	mock := &mockDockerClient{
		containers: []docker.Container{
			{ID: "cached1aabbccddeeff00", Name: "cached-app", Image: "app:latest", State: "running", Labels: map[string]string{}, StartedAt: time.Now()},
		},
		stats: map[string]*docker.Stats{},
	}

	cache := NewStatsCache(30*time.Second, true)
	cache.Set("cached1aabbccddeeff00", cachedStats)

	collector := NewContainerCollector(mock, newTestFilter(), cache, newTestConfig())
	metrics := collectMetrics(collector)

	// Should use cached stats — no call to GetContainerStats needed
	memUsage := findMetric(metrics, "container_memory_usage_bytes")
	assert.Len(t, memUsage, 1, "expected memory metric from cached stats")

	require.Equal(t, int64(1), cache.Hits(), "expected one cache hit")
}

func TestCollect_FilterExcludes(t *testing.T) {
	mock := &mockDockerClient{
		containers: []docker.Container{
			{ID: "incl1aabbccddeeff0011", Name: "web", Image: "nginx:latest", State: "running", Labels: map[string]string{}, StartedAt: time.Now()},
			{ID: "excl1aabbccddeeff0011", Name: "internal-proxy", Image: "envoy:latest", State: "running", Labels: map[string]string{}, StartedAt: time.Now()},
		},
		stats: map[string]*docker.Stats{
			"incl1aabbccddeeff0011": {MemoryUsage: 100, Networks: map[string]docker.NetworkStats{}, BlockIO: map[string]docker.BlockIOStats{}},
			"excl1aabbccddeeff0011": {MemoryUsage: 200, Networks: map[string]docker.NetworkStats{}, BlockIO: map[string]docker.BlockIOStats{}},
		},
	}

	filter, err := docker.NewFilter(config.FiltersConfig{
		Exclude: config.FilterSet{Names: []string{"internal-.*"}},
	})
	require.NoError(t, err)

	cache := NewStatsCache(30*time.Second, false)
	collector := NewContainerCollector(mock, filter, cache, newTestConfig())
	metrics := collectMetrics(collector)

	memUsage := findMetric(metrics, "container_memory_usage_bytes")
	assert.Len(t, memUsage, 1, "expected only one container after filter exclusion")
}
