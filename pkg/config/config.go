package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for the exporter.
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Docker      DockerConfig      `mapstructure:"docker"`
	Collection  CollectionConfig  `mapstructure:"collection"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Performance PerformanceConfig `mapstructure:"performance"`
}

type ServerConfig struct {
	Port        string     `mapstructure:"port"`
	Address     string     `mapstructure:"address"`
	MetricsPath string     `mapstructure:"metrics_path"`
	HealthPath  string     `mapstructure:"health_path"`
	ReadyPath   string     `mapstructure:"ready_path"`
	TLS         TLSConfig  `mapstructure:"tls"`
	Auth        AuthConfig `mapstructure:"auth"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type AuthConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type DockerConfig struct {
	Host       string          `mapstructure:"host"`
	APIVersion string          `mapstructure:"api_version"`
	TLS        DockerTLSConfig `mapstructure:"tls"`
}

type DockerTLSConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	CACert  string `mapstructure:"ca_cert"`
	Cert    string `mapstructure:"cert"`
	Key     string `mapstructure:"key"`
	Verify  bool   `mapstructure:"verify"`
}

type CollectionConfig struct {
	Interval   time.Duration    `mapstructure:"interval"`
	Timeout    time.Duration    `mapstructure:"timeout"`
	Collectors CollectorsConfig `mapstructure:"collectors"`
	Filters    FiltersConfig    `mapstructure:"filters"`
}

type CollectorsConfig struct {
	Container bool `mapstructure:"container"`
	System    bool `mapstructure:"system"`
}

type FiltersConfig struct {
	Include FilterSet `mapstructure:"include"`
	Exclude FilterSet `mapstructure:"exclude"`
}

type FilterSet struct {
	Labels []string `mapstructure:"labels"`
	Names  []string `mapstructure:"names"`
	Images []string `mapstructure:"images"`
}

type MetricsConfig struct {
	Namespace    string            `mapstructure:"namespace"`
	GlobalLabels map[string]string `mapstructure:"global_labels"`
	Cache        CacheConfig       `mapstructure:"cache"`
}

type CacheConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type PerformanceConfig struct {
	MaxConcurrent int  `mapstructure:"max_concurrent"`
	Workers       int  `mapstructure:"workers"`
	PprofEnabled  bool `mapstructure:"pprof_enabled"`
}

// setDefaults configures default values in viper.
func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", "9200")
	v.SetDefault("server.address", "0.0.0.0")
	v.SetDefault("server.metrics_path", "/metrics")
	v.SetDefault("server.health_path", "/health")
	v.SetDefault("server.ready_path", "/ready")
	v.SetDefault("server.tls.enabled", false)
	v.SetDefault("server.auth.enabled", false)

	// Docker
	v.SetDefault("docker.host", "unix:///var/run/docker.sock")
	v.SetDefault("docker.api_version", "")
	v.SetDefault("docker.tls.enabled", false)
	v.SetDefault("docker.tls.verify", true)

	// Collection
	v.SetDefault("collection.interval", 0)
	v.SetDefault("collection.timeout", "30s")
	v.SetDefault("collection.collectors.container", true)
	v.SetDefault("collection.collectors.system", true)

	// Metrics
	v.SetDefault("metrics.namespace", "")
	v.SetDefault("metrics.cache.enabled", true)
	v.SetDefault("metrics.cache.ttl", "30s")

	// Logging
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")

	// Performance
	v.SetDefault("performance.max_concurrent", 10)
	v.SetDefault("performance.workers", 4)
	v.SetDefault("performance.pprof_enabled", false)
}

// bindEnvVars maps environment variables to config keys.
func bindEnvVars(v *viper.Viper) {
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicit bindings for commonly used env vars
	bindings := map[string]string{
		"server.port":                "EXPORTER_PORT",
		"server.address":             "EXPORTER_ADDRESS",
		"server.metrics_path":        "EXPORTER_METRICS_PATH",
		"docker.host":                "DOCKER_HOST",
		"docker.api_version":         "DOCKER_API_VERSION",
		"collection.interval":        "COLLECTION_INTERVAL",
		"collection.timeout":         "COLLECTION_TIMEOUT",
		"logging.level":              "LOG_LEVEL",
		"logging.format":             "LOG_FORMAT",
		"performance.max_concurrent": "MAX_CONCURRENT",
		"performance.workers":        "WORKERS",
	}
	for key, env := range bindings {
		_ = v.BindEnv(key, env)
	}
}

// RegisterFlags registers CLI flags and binds them to viper.
func RegisterFlags(fs *pflag.FlagSet, v *viper.Viper) {
	fs.String("server.port", "", "Server port")
	fs.String("server.address", "", "Server bind address")
	fs.String("docker.host", "", "Docker host URL")
	fs.String("logging.level", "", "Log level (debug, info, warn, error)")
	fs.String("logging.format", "", "Log format (json, text)")
	_ = v.BindPFlags(fs)
}

// Load reads configuration from the given file path, env vars, and CLI flags.
func Load(configFile string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnvVars(v)

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the configuration for invalid values.
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server.port is required")
	}
	if c.Docker.Host == "" {
		return fmt.Errorf("docker.host is required")
	}
	if c.Server.Auth.Enabled {
		if c.Server.Auth.Username == "" || c.Server.Auth.Password == "" {
			return fmt.Errorf("auth username and password are required when auth is enabled")
		}
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" || c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS cert_file and key_file are required when TLS is enabled")
		}
	}
	if c.Performance.MaxConcurrent < 1 {
		return fmt.Errorf("performance.max_concurrent must be >= 1")
	}
	if c.Performance.Workers < 1 {
		return fmt.Errorf("performance.workers must be >= 1")
	}
	return nil
}
