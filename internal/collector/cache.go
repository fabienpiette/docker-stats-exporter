package collector

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
)

type cacheEntry struct {
	stats     *docker.Stats
	timestamp time.Time
}

// StatsCache provides thread-safe caching of container stats with TTL-based expiry.
type StatsCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
	enabled bool

	hits   atomic.Int64
	misses atomic.Int64
}

// NewStatsCache creates a new cache. If enabled is false, Get always misses.
func NewStatsCache(ttl time.Duration, enabled bool) *StatsCache {
	return &StatsCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
		enabled: enabled,
	}
}

// Get returns cached stats for the container ID if they exist and haven't expired.
func (c *StatsCache) Get(id string) (*docker.Stats, bool) {
	if !c.enabled {
		c.misses.Add(1)
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()

	if !ok || time.Since(entry.timestamp) > c.ttl {
		c.misses.Add(1)
		return nil, false
	}

	c.hits.Add(1)
	return entry.stats, true
}

// Set stores stats in the cache.
func (c *StatsCache) Set(id string, stats *docker.Stats) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	c.entries[id] = cacheEntry{stats: stats, timestamp: time.Now()}
	c.mu.Unlock()
}

// Evict removes a specific container from the cache.
func (c *StatsCache) Evict(id string) {
	c.mu.Lock()
	delete(c.entries, id)
	c.mu.Unlock()
}

// EvictStale removes all entries older than the TTL.
func (c *StatsCache) EvictStale() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for id, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, id)
		}
	}
}

// Hits returns the total cache hit count.
func (c *StatsCache) Hits() int64 { return c.hits.Load() }

// Misses returns the total cache miss count.
func (c *StatsCache) Misses() int64 { return c.misses.Load() }
