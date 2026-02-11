# Architecture

This document describes the high-level architecture of docker-stats-exporter.
If you want to familiarize yourself with the codebase, you are in the right
place.

## Bird's Eye View

The exporter is a Prometheus scrape target that translates Docker API responses
into Prometheus metrics. On each scrape:

```
Prometheus GET /metrics
        |
        v
┌─ ContainerCollector.Collect() ──────────────────────────┐
│  list containers -> filter -> fetch stats (bounded) ->  │
│  emit metrics                                           │
└─────────────────────────────────────────────────────────┘
        |
        v
  Prometheus response (text/plain)
```

Nothing runs between scrapes. There is no background goroutine collecting
stats, the Docker API is called synchronously during each Prometheus scrape,
with `stream=false` for one-shot snapshots.

The project follows standard Go layout: `cmd/` for the binary entry point,
`internal/` for private packages, `pkg/` for the reusable config package.

## Code Map

Packages are listed in dependency order (bottom-up). Lower packages never
import higher ones.

### `pkg/config/`

Three-tier configuration: hardcoded defaults -> YAML file -> environment
variables / CLI flags. Uses Viper with explicit env-var bindings (not
automatic prefix matching). Configuration is immutable after `Load()` returns.

Key file: `config.go`, all config structs, `Load()`, `Validate()`.

### `internal/metrics/`

All `prometheus.Desc` declarations live here, created once at package init
time. Also provides `SafeNewConstMetric` and `SendSafe`, wrappers that
catch label-count mismatches and log warnings instead of panicking.

Key files: `definitions.go` (every Desc), `helpers.go` (safe wrappers,
`NanosecondsToSeconds` constant).

**Architecture Invariant:** all metric creation must go through
`SafeNewConstMetric` -> `SendSafe`. Direct `prometheus.MustNewConstMetric`
calls risk panics on label mismatch.

### `internal/docker/`

Docker API wrapper. Encapsulates the Docker SDK, timeout handling, and the
double-inspect pattern (stats + inspect per container, because
`ContainerStats` doesn't include health/restart/exit data).

Key types and files:

- `client.go`, `Client` struct with `ListContainers`, `GetContainerStats`,
  `GetSystemInfo`. Thread-safe; no caching at this layer.
- `stats.go`, `Stats`, `NetworkStats`, `BlockIOStats` types and
  `ParseDockerStats()`. Handles cgroup v1 vs v2 differences
  (v1: `rss`/`cache`, v2: `anon`/`file`).
- `filter.go`, `Filter` with regex-compiled include/exclude rules.
  Patterns compiled once in `NewFilter()`, reused every scrape.
- `labels.go`, `ContainerLabels` extraction and `SanitizeLabelValue`.

**Architecture Invariant:** exclude rules always take precedence over include
rules. Even if a container matches every include pattern, one exclude match
blocks it.

**Architecture Invariant:** label order is fixed everywhere:
`["container_name", "compose_service", "compose_project", "image"]`. Network
metrics append `"interface"`, block I/O appends `"device"`. This must match
the Desc definitions in `internal/metrics/`.

### `internal/collector/`

Implements `prometheus.Collector` using the custom collector pattern, no
pre-registered metric vectors. `Describe()` returns static descriptors;
`Collect()` rebuilds every metric from scratch on each scrape.

Key files:

- `container.go`, `ContainerCollector`. Orchestrates the full scrape flow:
  list -> filter -> cache check -> bounded concurrent fetch -> emit. The bounded
  worker pool is a buffered-channel semaphore (`make(chan struct{}, maxConcurrent)`).
  Stopped containers emit state metrics only (no stats available).
- `system.go`, `SystemCollector`. Fetches daemon-level counts (containers,
  images, volumes, networks) and emits `exporter_build_info` and
  `exporter_up`.
- `cache.go`, `StatsCache`. TTL-based, thread-safe (`sync.RWMutex` + atomic
  hit/miss counters). Disabled mode is zero-overhead (all operations are
  no-ops).

**Architecture Invariant:** the custom collector pattern means metrics for
removed containers disappear automatically, no stale time series, no manual
cleanup.

**Architecture Invariant:** `ContainerCollector` depends on `DockerClient`
(an interface), not on `*docker.Client` directly. This enables mock-based
testing.

### `internal/server/`

HTTP server, middleware, and handlers. Middleware stack order:
recovery (outermost) -> logging -> basic auth (innermost).

Key files: `server.go` (lifecycle), `middleware.go` (three middleware
functions), `handlers.go` (`/health`, `/ready`, `/version`).

**Architecture Invariant:** basic auth uses `subtle.ConstantTimeCompare`
to prevent timing attacks.

### `cmd/exporter/main.go`

Entry point. Wires everything together: config -> logger -> Docker client ->
filter -> cache -> collectors -> HTTP server -> signal handling (SIGINT/SIGTERM
with 10s graceful shutdown). Injects build info (version/commit/date from
ldflags) into `collector.Version` / `collector.Commit` / `collector.BuildDate`.

## Invariants

Rules that are invisible in code and easy to violate accidentally:

- **CPU metrics are nanosecond counters.** They are converted to seconds
  (`× 1e-9`) only at emission time in the collector. `ParseDockerStats`
  returns raw nanoseconds.

- **Configuration is immutable after startup.** No runtime reloading, no
  mutexes around config access. All packages receive config values at
  construction time.

- **Bounded concurrency is mandatory.** The semaphore in
  `ContainerCollector.Collect()` prevents resource exhaustion when monitoring
  many containers. Removing it risks file descriptor starvation.

- **All errors are logged, never silently swallowed.** Metric creation
  failures go through `SafeNewConstMetric` which logs warnings. Per-container
  stats failures are logged and counted in `exporter_scrape_errors_total`.
  One container's failure never blocks others.

- **The Docker client adds method-level timeouts.** Each API call wraps the
  incoming context with its own `context.WithTimeout`. The caller's context
  controls cancellation, the client's timeout controls maximum wait.

## Cross-Cutting Concerns

**Error handling** follows three tiers: fatal at startup (config/client
creation), warn-and-continue during collection (per-container), and
panic-recovery in the HTTP middleware.

**Logging** uses logrus with JSON format (set before config loads to avoid
format mismatch on early log lines). DEBUG level logs HTTP requests and
raw memory stats; WARN level logs per-container failures.

**Concurrency** is minimal by design. The stats cache uses `sync.RWMutex`
with atomic counters. The worker pool uses a buffered channel as semaphore.
Config and metric descriptors are immutable, no synchronization needed.

**Testing** uses interface-based mocking. `ContainerCollector` accepts a
`DockerClient` interface; tests provide a `mockDockerClient` with
controllable responses and errors.

## A Typical Change

**Adding a new container metric** (e.g., `container_oom_kills_total`):

1. Add the `prometheus.Desc` in `internal/metrics/definitions.go` with the
   standard `containerLabelNames`.
2. Add the field to `docker.Stats` in `internal/docker/stats.go` and parse it
   in `ParseDockerStats()`.
3. Emit it in `internal/collector/container.go`, add a `SendSafe` call in the
   appropriate emit function (or create a new one).
4. Add a test case in `internal/docker/stats_test.go` with a fixture in
   `testdata/`.
5. Document the metric in `README.md` under the relevant table.
