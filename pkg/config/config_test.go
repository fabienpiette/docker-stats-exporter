package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "9200", cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Address)
	assert.Equal(t, "/metrics", cfg.Server.MetricsPath)
	assert.Equal(t, "unix:///var/run/docker.sock", cfg.Docker.Host)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, 10, cfg.Performance.MaxConcurrent)
	assert.Equal(t, 4, cfg.Performance.Workers)
	assert.True(t, cfg.Metrics.Cache.Enabled)
	assert.True(t, cfg.Collection.Collectors.Container)
	assert.True(t, cfg.Collection.Collectors.System)
}

func TestLoad_ConfigFile(t *testing.T) {
	content := `
server:
  port: "8080"
  address: "127.0.0.1"
docker:
  host: "tcp://localhost:2375"
logging:
  level: "debug"
  format: "text"
performance:
  max_concurrent: 20
  workers: 8
`
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(content), 0644))

	cfg, err := Load(cfgFile)
	require.NoError(t, err)

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Address)
	assert.Equal(t, "tcp://localhost:2375", cfg.Docker.Host)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "text", cfg.Logging.Format)
	assert.Equal(t, 20, cfg.Performance.MaxConcurrent)
	assert.Equal(t, 8, cfg.Performance.Workers)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("EXPORTER_PORT", "9999")
	t.Setenv("DOCKER_HOST", "tcp://remote:2375")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "9999", cfg.Server.Port)
	assert.Equal(t, "tcp://remote:2375", cfg.Docker.Host)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestValidate_AuthMissingCredentials(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: "9200",
			Auth: AuthConfig{Enabled: true},
		},
		Docker:      DockerConfig{Host: "unix:///var/run/docker.sock"},
		Performance: PerformanceConfig{MaxConcurrent: 1, Workers: 1},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_TLSMissingFiles(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: "9200",
			TLS:  TLSConfig{Enabled: true},
		},
		Docker:      DockerConfig{Host: "unix:///var/run/docker.sock"},
		Performance: PerformanceConfig{MaxConcurrent: 1, Workers: 1},
	}
	assert.Error(t, cfg.Validate())
}

func TestValidate_InvalidPerformance(t *testing.T) {
	cfg := &Config{
		Server:      ServerConfig{Port: "9200"},
		Docker:      DockerConfig{Host: "unix:///var/run/docker.sock"},
		Performance: PerformanceConfig{MaxConcurrent: 0, Workers: 1},
	}
	assert.Error(t, cfg.Validate())
}

func TestLoad_MissingConfigFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
}
