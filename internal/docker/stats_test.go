package docker

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestStatsJSON(t testing.TB) *types.StatsJSON {
	t.Helper()
	data, err := os.ReadFile("../../testdata/stats_response.json")
	require.NoError(t, err)

	var stats types.StatsJSON
	require.NoError(t, json.Unmarshal(data, &stats))
	return &stats
}

func testContainerJSON() *types.ContainerJSON {
	return &types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:           "abc123def456",
			Name:         "/test-container",
			RestartCount: 2,
			State: &types.ContainerState{
				Status:    "running",
				ExitCode:  0,
				StartedAt: "2024-01-15T09:00:00.000000000Z",
				Health: &types.Health{
					Status: "healthy",
				},
			},
		},
		Config: &containertypes.Config{
			Image: "nginx:latest",
			Labels: map[string]string{
				"com.docker.compose.service": "web",
				"com.docker.compose.project": "myapp",
			},
		},
	}
}

func TestParseDockerStats(t *testing.T) {
	statsJSON := loadTestStatsJSON(t)
	containerJSON := testContainerJSON()

	stats := ParseDockerStats(statsJSON, containerJSON)

	// Container identity
	assert.Equal(t, "abc123def456", stats.ContainerID)
	assert.Equal(t, "test-container", stats.Name) // leading slash trimmed
	assert.Equal(t, "nginx:latest", stats.Image)
	assert.Equal(t, "running", stats.Status)
	assert.Equal(t, "healthy", stats.Health)
	assert.Equal(t, 2, stats.RestartCount)
	assert.Equal(t, 0, stats.ExitCode)
	assert.False(t, stats.StartedAt.IsZero())

	// Memory
	assert.Equal(t, uint64(104857600), stats.MemoryUsage)
	assert.Equal(t, uint64(536870912), stats.MemoryLimit)
	assert.Equal(t, uint64(20971520), stats.MemoryCache)
	assert.Equal(t, uint64(73400320), stats.MemoryRSS)
	assert.Equal(t, uint64(1048576), stats.MemorySwap)
	assert.Equal(t, uint64(3), stats.MemoryFailcnt)
	// WorkingSet = usage (104857600) - inactive_file (5242880) = 99614720
	assert.Equal(t, uint64(99614720), stats.MemoryWorkingSet)

	// CPU
	assert.Equal(t, uint64(500000000000), stats.CPUUsageTotal)
	assert.Equal(t, uint64(100000000000), stats.CPUUsageSystem)
	assert.Equal(t, uint64(400000000000), stats.CPUUsageUser)
	assert.Equal(t, uint64(10), stats.CPUThrottledPeriods)
	assert.Equal(t, uint64(5000000000), stats.CPUThrottledTime)
	assert.Equal(t, uint32(2), stats.OnlineCPUs)

	// Network
	require.Contains(t, stats.Networks, "eth0")
	eth0 := stats.Networks["eth0"]
	assert.Equal(t, uint64(1073741824), eth0.RxBytes)
	assert.Equal(t, uint64(536870912), eth0.TxBytes)
	assert.Equal(t, uint64(500000), eth0.RxPackets)
	assert.Equal(t, uint64(250000), eth0.TxPackets)
	assert.Equal(t, uint64(10), eth0.RxErrors)
	assert.Equal(t, uint64(2), eth0.TxErrors)
	assert.Equal(t, uint64(5), eth0.RxDropped)
	assert.Equal(t, uint64(1), eth0.TxDropped)

	// PIDs
	assert.Equal(t, uint64(25), stats.PIDsCurrent)

	// Block I/O
	require.Contains(t, stats.BlockIO, "8:0")
	bio := stats.BlockIO["8:0"]
	assert.Equal(t, uint64(1048576), bio.ReadBytes)
	assert.Equal(t, uint64(2097152), bio.WriteBytes)
	assert.Equal(t, uint64(100), bio.ReadOps)
	assert.Equal(t, uint64(200), bio.WriteOps)
}

func TestParseDockerStats_NoHealth(t *testing.T) {
	statsJSON := loadTestStatsJSON(t)
	containerJSON := testContainerJSON()
	containerJSON.State.Health = nil

	stats := ParseDockerStats(statsJSON, containerJSON)
	assert.Equal(t, "", stats.Health)
}

func TestParseDockerStats_EmptyStartedAt(t *testing.T) {
	statsJSON := loadTestStatsJSON(t)
	containerJSON := testContainerJSON()
	containerJSON.State.StartedAt = ""

	stats := ParseDockerStats(statsJSON, containerJSON)
	assert.True(t, stats.StartedAt.IsZero())
}

func TestParseDockerStats_CgroupV2Memory(t *testing.T) {
	statsJSON := loadTestStatsJSON(t)
	containerJSON := testContainerJSON()

	// Simulate cgroup v2: "anon" instead of "rss", no "rss" key
	delete(statsJSON.MemoryStats.Stats, "rss")
	statsJSON.MemoryStats.Stats["anon"] = 65536000

	stats := ParseDockerStats(statsJSON, containerJSON)
	assert.Equal(t, uint64(65536000), stats.MemoryRSS)
}

func TestTrimLeadingSlash(t *testing.T) {
	assert.Equal(t, "container", trimLeadingSlash("/container"))
	assert.Equal(t, "container", trimLeadingSlash("container"))
	assert.Equal(t, "", trimLeadingSlash(""))
}

func TestDeviceKey(t *testing.T) {
	assert.Equal(t, "8:0", deviceKey(8, 0))
	assert.Equal(t, "253:1", deviceKey(253, 1))
}
