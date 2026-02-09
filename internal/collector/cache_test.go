package collector

import (
	"sync"
	"testing"
	"time"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestStatsCache_GetSet(t *testing.T) {
	c := NewStatsCache(5*time.Second, true)

	stats := &docker.Stats{ContainerID: "abc123", MemoryUsage: 1024}
	c.Set("abc123", stats)

	got, ok := c.Get("abc123")
	assert.True(t, ok)
	assert.Equal(t, uint64(1024), got.MemoryUsage)
}

func TestStatsCache_Miss(t *testing.T) {
	c := NewStatsCache(5*time.Second, true)

	_, ok := c.Get("nonexistent")
	assert.False(t, ok)
	assert.Equal(t, int64(1), c.Misses())
}

func TestStatsCache_TTLExpiry(t *testing.T) {
	c := NewStatsCache(10*time.Millisecond, true)

	c.Set("abc", &docker.Stats{ContainerID: "abc"})
	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get("abc")
	assert.False(t, ok, "expired entry should not be returned")
}

func TestStatsCache_Evict(t *testing.T) {
	c := NewStatsCache(5*time.Second, true)

	c.Set("abc", &docker.Stats{ContainerID: "abc"})
	c.Evict("abc")

	_, ok := c.Get("abc")
	assert.False(t, ok)
}

func TestStatsCache_EvictStale(t *testing.T) {
	c := NewStatsCache(10*time.Millisecond, true)

	c.Set("old", &docker.Stats{ContainerID: "old"})
	time.Sleep(20 * time.Millisecond)
	c.Set("new", &docker.Stats{ContainerID: "new"})

	c.EvictStale()

	_, okOld := c.Get("old")
	assert.False(t, okOld)

	_, okNew := c.Get("new")
	assert.True(t, okNew)
}

func TestStatsCache_Disabled(t *testing.T) {
	c := NewStatsCache(5*time.Second, false)

	c.Set("abc", &docker.Stats{ContainerID: "abc"})
	_, ok := c.Get("abc")
	assert.False(t, ok, "disabled cache should always miss")
}

func TestStatsCache_HitMissCounters(t *testing.T) {
	c := NewStatsCache(5*time.Second, true)

	c.Set("abc", &docker.Stats{ContainerID: "abc"})
	c.Get("abc")  // hit
	c.Get("abc")  // hit
	c.Get("miss") // miss

	assert.Equal(t, int64(2), c.Hits())
	assert.Equal(t, int64(1), c.Misses())
}

func TestStatsCache_ConcurrentAccess(t *testing.T) {
	c := NewStatsCache(5*time.Second, true)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		id := "container-" + string(rune('A'+i%26))
		go func() {
			defer wg.Done()
			c.Set(id, &docker.Stats{ContainerID: id})
		}()
		go func() {
			defer wg.Done()
			c.Get(id)
		}()
	}
	wg.Wait()
	// No race condition panics = pass
}
