package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// Client wraps the Docker API client with timeout and convenience methods.
type Client struct {
	cli     *client.Client
	timeout time.Duration
}

// NewClient creates a Docker client from configuration.
func NewClient(cfg config.DockerConfig, timeout time.Duration) (*Client, error) {
	opts := []client.Opt{
		client.WithHost(cfg.Host),
		client.WithAPIVersionNegotiation(),
	}

	if cfg.APIVersion != "" {
		opts = append(opts, client.WithVersion(cfg.APIVersion))
	}

	if cfg.TLS.Enabled {
		opts = append(opts, client.WithTLSClientConfig(cfg.TLS.CACert, cfg.TLS.Cert, cfg.TLS.Key))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	return &Client{cli: cli, timeout: timeout}, nil
}

// ListContainers returns all containers (running and stopped).
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	raw, err := c.cli.ContainerList(ctx, containertypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	containers := make([]Container, 0, len(raw))
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = trimLeadingSlash(r.Names[0])
		}

		ctr := Container{
			ID:     r.ID,
			Name:   name,
			Image:  r.Image,
			Labels: r.Labels,
			Status: r.Status,
			State:  r.State,
		}

		// Fetch inspect data for health, restart count, exit code, started_at
		inspect, err := c.cli.ContainerInspect(ctx, r.ID)
		if err == nil {
			ctr.RestartCount = inspect.RestartCount
			ctr.ExitCode = inspect.State.ExitCode
			if inspect.State.Health != nil {
				ctr.Health = inspect.State.Health.Status
			}
			if inspect.State.StartedAt != "" {
				if t, parseErr := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); parseErr == nil {
					ctr.StartedAt = t
				}
			}
		}

		containers = append(containers, ctr)
	}

	return containers, nil
}

// GetContainerStats fetches a one-shot stats snapshot for a container.
func (c *Client) GetContainerStats(ctx context.Context, id string) (*Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// stream=false: returns a single JSON object and closes
	resp, err := c.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("getting stats for %s: %w", id, err)
	}
	defer resp.Body.Close()

	var statsJSON types.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&statsJSON); err != nil {
		return nil, fmt.Errorf("decoding stats for %s: %w", id, err)
	}

	// We also need inspect data for full parsing
	inspect, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s: %w", id, err)
	}

	return ParseDockerStats(&statsJSON, &inspect), nil
}

// GetSystemInfo returns Docker daemon information.
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting system info: %w", err)
	}

	// Count images
	imgList, err := c.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing images: %w", err)
	}

	// Count volumes
	volResp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing volumes: %w", err)
	}

	// Count networks
	netList, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing networks: %w", err)
	}

	return &SystemInfo{
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            len(imgList),
		Volumes:           len(volResp.Volumes),
		Networks:          len(netList),
		ServerVersion:     info.ServerVersion,
	}, nil
}

// Ping checks connectivity to the Docker daemon.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.cli.Ping(ctx)
	return err
}

// Close releases the Docker client resources.
func (c *Client) Close() error {
	return c.cli.Close()
}
