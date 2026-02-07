# Docker Stats Exporter

A lightweight Prometheus exporter for Docker container metrics. It collects memory, CPU, network, disk I/O, and container state metrics directly from the Docker API, without the overhead of cAdvisor.

The binary is around 12 MB and the Docker image stays under 20 MB. It works with both cgroup v1 and v2.

## Quick start

The simplest way to run it is with the Docker socket mounted directly:

```yaml
services:
  docker-stats-exporter:
    image: docker-stats-exporter:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    ports:
      - "9200:9200"
```

Or build and run the binary:

```bash
make build
./bin/docker-stats-exporter
```

Metrics are available at `http://localhost:9200/metrics`.

## Building

Requires Go 1.21 or later.

```bash
make build       # build the binary to bin/
make test        # run tests with race detector
make docker      # build the Docker image
make clean       # remove build artifacts
make run         # go run directly
make lint        # run golangci-lint
```

## Configuration

The exporter works out of the box with zero configuration. It connects to the local Docker socket and listens on port 9200.

To customize, create a config file (see `config.yaml.example` for all options) and pass it with `--config`:

```bash
./bin/docker-stats-exporter --config /path/to/config.yaml
```

Settings can also be overridden with environment variables:

| Variable | Default | Description |
|---|---|---|
| `EXPORTER_PORT` | `9200` | Listen port |
| `EXPORTER_ADDRESS` | `0.0.0.0` | Bind address |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon address |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, text) |
| `MAX_CONCURRENT` | `10` | Max parallel stats requests |
| `COLLECTION_TIMEOUT` | `10s` | Docker API call timeout |

### Filtering containers

You can include or exclude containers by name, image, or label. Patterns for names and images are regular expressions. Exclude rules always take precedence over include rules.

```yaml
collection:
  filters:
    include:
      labels: ["monitoring=true"]
      names: ["^web-.*"]
      images: ["nginx:.*"]
    exclude:
      names: ["^test-.*"]
```

### Basic auth and TLS

Both are optional and disabled by default:

```yaml
server:
  auth:
    enabled: true
    username: "prometheus"
    password: "secret"
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

## Metrics

All container metrics carry these labels: `container_name`, `compose_service`, `compose_project`, `image`.

### Memory

| Metric | Type | Description |
|---|---|---|
| `container_memory_usage_bytes` | gauge | Current memory usage |
| `container_memory_limit_bytes` | gauge | Memory limit |
| `container_memory_cache_bytes` | gauge | Page cache usage |
| `container_memory_rss_bytes` | gauge | Resident set size |
| `container_memory_swap_bytes` | gauge | Swap usage |
| `container_memory_working_set_bytes` | gauge | Working set (usage minus inactive file) |
| `container_memory_failcnt` | gauge | OOM kill limit hit count |

### CPU

CPU metrics are counters in seconds. Use `rate()` in PromQL to get usage percentage:

```promql
rate(container_cpu_usage_seconds_total[5m]) * 100
```

| Metric | Type | Description |
|---|---|---|
| `container_cpu_usage_seconds_total` | counter | Total CPU time consumed |
| `container_cpu_system_seconds_total` | counter | Kernel mode CPU time |
| `container_cpu_user_seconds_total` | counter | User mode CPU time |
| `container_cpu_throttling_periods_total` | counter | Throttling period count |
| `container_cpu_throttled_seconds_total` | counter | Total throttled time |

### Network

Per-interface metrics (extra label: `interface`):

| Metric | Type | Description |
|---|---|---|
| `container_network_receive_bytes_total` | counter | Bytes received |
| `container_network_transmit_bytes_total` | counter | Bytes transmitted |
| `container_network_receive_packets_total` | counter | Packets received |
| `container_network_transmit_packets_total` | counter | Packets transmitted |
| `container_network_receive_errors_total` | counter | Receive errors |
| `container_network_transmit_errors_total` | counter | Transmit errors |
| `container_network_receive_dropped_total` | counter | Received packets dropped |
| `container_network_transmit_dropped_total` | counter | Transmitted packets dropped |

### Disk I/O

Per-device metrics (extra label: `device`):

| Metric | Type | Description |
|---|---|---|
| `container_fs_reads_bytes_total` | counter | Bytes read |
| `container_fs_writes_bytes_total` | counter | Bytes written |
| `container_fs_reads_total` | counter | Read operations |
| `container_fs_writes_total` | counter | Write operations |

### Container state

These are emitted for all containers, including stopped ones:

| Metric | Type | Description |
|---|---|---|
| `container_last_seen` | gauge | Unix timestamp of last observation |
| `container_start_time_seconds` | gauge | Start time as Unix timestamp |
| `container_uptime_seconds` | gauge | Uptime in seconds |
| `container_info` | gauge | Always 1; carries extra labels (container_id, status, health_status, created) |
| `container_health_status` | gauge | 0=none, 1=starting, 2=healthy, 3=unhealthy |
| `container_restart_count` | gauge | Restart count |
| `container_exit_code` | gauge | Last exit code |

### System

| Metric | Type | Description |
|---|---|---|
| `docker_containers_total` | gauge | Container count by state (running, paused, stopped) |
| `docker_images_total` | gauge | Total images |
| `docker_volumes_total` | gauge | Total volumes |
| `docker_networks_total` | gauge | Total networks |
| `exporter_build_info` | gauge | Build metadata (version, commit, build_date, go_version) |
| `exporter_up` | gauge | 1 if Docker daemon is reachable |
| `exporter_scrape_duration_seconds` | gauge | Scrape time per collector |
| `exporter_scrape_errors_total` | counter | Error count per collector |

## HTTP endpoints

| Path | Description |
|---|---|
| `/metrics` | Prometheus metrics |
| `/health` | Always returns 200. For liveness probes. |
| `/ready` | Returns 200 if Docker is reachable, 503 otherwise. For readiness probes. |
| `/version` | JSON with version, commit, build date, and Go version. |

## Deployment

### With a socket proxy (recommended for production)

Instead of mounting the Docker socket directly, use a socket proxy to limit the API surface:

```yaml
services:
  socket-proxy:
    image: tecnativa/docker-socket-proxy
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - CONTAINERS=1
      - INFO=1
      - IMAGES=1
      - NETWORKS=1
      - VOLUMES=1

  docker-stats-exporter:
    image: docker-stats-exporter:latest
    environment:
      - DOCKER_HOST=tcp://socket-proxy:2375
    ports:
      - "9200:9200"
    depends_on:
      - socket-proxy
```

### Full monitoring stack

The included `docker-compose.yml` sets up the exporter with a socket proxy, Prometheus, and Grafana. The Prometheus scrape config is in `prometheus.yml`.

```bash
docker compose up -d
```

- Exporter: http://localhost:9200/metrics
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### Remote Docker host

Point `DOCKER_HOST` to a remote TCP address. Enable TLS in the config if the remote daemon requires it.

## Design notes

The exporter uses a custom Prometheus collector (not pre-registered metric vectors). Metrics are built fresh on each scrape, which means containers that disappear are automatically cleaned up without stale time series.

Stats are fetched with `stream=false` on the Docker API, giving a single point-in-time snapshot per container per scrape. A bounded worker pool limits concurrent Docker API calls (configurable via `max_concurrent`).

An optional in-memory cache with configurable TTL avoids redundant Docker API calls when Prometheus scrapes faster than the cache window.

## License

AGPL-3.0
