# Docker Stats Exporter

A lightweight Prometheus exporter for Docker container metrics. It collects memory, CPU, network, disk I/O, and container state metrics directly from the Docker API, without the overhead of cAdvisor.

---

<p align="center">
  <img src="docs/demo.gif" alt="goscribe demo" width="800">
</p>

## Quick Start

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

Metrics are available at `http://localhost:9200/metrics`:

```
# HELP container_memory_usage_bytes Current memory usage in bytes
# TYPE container_memory_usage_bytes gauge
container_memory_usage_bytes{container_name="nginx",compose_service="web",compose_project="myapp",image="nginx:latest"} 2.7262976e+07

# HELP container_cpu_usage_seconds_total Total CPU time consumed in seconds
# TYPE container_cpu_usage_seconds_total counter
container_cpu_usage_seconds_total{container_name="nginx",compose_service="web",compose_project="myapp",image="nginx:latest"} 15.230744

# HELP container_network_receive_bytes_total Network bytes received
# TYPE container_network_receive_bytes_total counter
container_network_receive_bytes_total{container_name="nginx",compose_service="web",compose_project="myapp",image="nginx:latest",interface="eth0"} 1.048576e+06
```

## Features

- **~12 MB binary, <20 MB Docker image** — runs comfortably on a Raspberry Pi or a small VPS
- **Zero stale series** — custom Prometheus collector rebuilds metrics each scrape; removed containers disappear automatically
- **cgroup v1 and v2** — handles both transparently, including Proxmox LXC environments
- **Compose-aware labels** — every metric carries `compose_service` and `compose_project` out of the box
- **Works behind a socket proxy** — only needs container and info API access, no host mounts
- **Zero-config defaults** — connects to the local Docker socket on port 9200; customize with env vars, CLI flags, or YAML

## Why not cAdvisor?

cAdvisor is a full host monitoring tool. If you only need Docker container metrics, this exporter is a lighter alternative.

| | Docker Stats Exporter | cAdvisor |
|---|---|---|
| Binary size | ~12 MB | ~200 MB+ |
| Runtime overhead | Low (API calls on scrape only) | High (continuous monitoring of host and containers) |
| Host mounts | Docker socket only | `/sys/fs/cgroup`, `/proc`, `/sys`, `/dev/disk`, Docker socket |
| Privileged mode | Not required | Required or multiple host mounts |
| Socket proxy | Works behind a restricted socket proxy | Needs direct access to host filesystems |
| Stale series | None (custom collector rebuilds metrics each scrape) | Can leave stale series for removed containers |
| Compose labels | Built-in `compose_service` and `compose_project` on every metric | Not extracted natively |
| Scope | Docker container metrics only | Host CPU, memory, disks, hardware topology, per-process stats |
| Container runtimes | Docker only | Docker, containerd, CRI-O |
| Kubernetes | Works via DaemonSet | Built into kubelet |

Use this exporter when you run Docker on a single host or a small cluster and want container metrics without the weight of full host monitoring. Use cAdvisor when you need host-level observability, per-process stats, or run Kubernetes with multiple container runtimes.

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
| `COLLECTION_TIMEOUT` | `30s` | Docker API call timeout |

### Filtering containers

Include or exclude containers by name, image, or label. Patterns for names and images are regular expressions. Exclude rules always take precedence over include rules.

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

### Process

| Metric | Type | Description |
|---|---|---|
| `container_pids_current` | gauge | Number of PIDs in the container |

### Container state

These are emitted for all containers, including stopped ones:

| Metric | Type | Description |
|---|---|---|
| `container_last_seen` | gauge | Unix timestamp of last observation |
| `container_start_time_seconds` | gauge | Start time as Unix timestamp |
| `container_uptime_seconds` | gauge | Uptime in seconds |
| `container_info` | gauge | Always 1; carries extra labels (container_id, status, health_status, started_at) |
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

## HTTP Endpoints

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

The included `docker-compose.yml` sets up the exporter with a socket proxy, Prometheus, and Grafana. A Grafana dashboard is auto-provisioned at startup. The Prometheus scrape config is in `prometheus.yml`.

```bash
docker compose up -d
```

- Exporter: http://localhost:9200/metrics
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin) — dashboard loads automatically

### Kubernetes

A DaemonSet manifest is provided in `deploy/kubernetes/daemonset.yml`. It runs one exporter pod per node with the Docker socket mounted read-only, includes Prometheus scrape annotations, and sets resource limits (50m CPU, 64Mi memory).

```bash
kubectl apply -f deploy/kubernetes/daemonset.yml
```

### Remote Docker host

Point `DOCKER_HOST` to a remote TCP address. Enable TLS in the config if the remote daemon requires it.

## Troubleshooting

### All memory metrics are 0 (Proxmox LXC)

When running Docker inside a Proxmox LXC container, memory metrics may report as 0 for all containers. This happens because Alpine (OpenRC) does not delegate cgroup v2 controllers to child cgroups by default. Docker can run containers but cannot read their memory stats.

Verify by checking inside the LXC container:

```bash
cat /sys/fs/cgroup/cgroup.subtree_control
```

If the output is empty or missing `memory`, the fix is to create an init script that moves processes to a child cgroup and enables the controllers:

```bash
cat > /etc/local.d/cgroup-delegate.start << 'SCRIPT'
#!/bin/sh
mkdir -p /sys/fs/cgroup/init
for pid in $(cat /sys/fs/cgroup/cgroup.procs 2>/dev/null); do
    echo "$pid" > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null
done
echo "+memory +cpu +io +pids" > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null
SCRIPT

chmod +x /etc/local.d/cgroup-delegate.start
rc-update add local default
```

Then stop and start the LXC container from the Proxmox host (a reboot inside the container is not sufficient):

```bash
pct stop <CTID> && pct start <CTID>
```

After the container comes back up, restart Docker so it recreates container cgroups with memory accounting:

```bash
service docker restart
```

This issue affects any Docker-in-LXC setup using Alpine or other OpenRC-based distributions. Systemd-based distributions handle this delegation automatically.

## Development

Requires Go 1.24 or later.

```bash
make build       # build the binary to bin/
make test        # run tests with race detector
make lint        # run golangci-lint
make docker      # build the Docker image
make run         # go run directly
make clean       # remove build artifacts
```

## Design Notes

The exporter uses a custom Prometheus collector (not pre-registered metric vectors). Metrics are built fresh on each scrape, so containers that disappear are automatically cleaned up without stale time series.

Stats are fetched with `stream=false` on the Docker API, giving a single point-in-time snapshot per container per scrape. A bounded worker pool limits concurrent Docker API calls (configurable via `max_concurrent`).

An optional in-memory cache with configurable TTL avoids redundant Docker API calls when Prometheus scrapes faster than the cache window.

## Acknowledgments

Thanks to all [contributors](https://github.com/fabienpiette/docker-stats-exporter/graphs/contributors).

<p align="center">
<a href="https://buymeacoffee.com/fabienpiette" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" height="60"></a>
</p>

## License

[AGPL-3.0](LICENSE)