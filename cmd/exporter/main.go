package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/fabienpiette/docker-stats-exporter/internal/collector"
	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/fabienpiette/docker-stats-exporter/internal/server"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	// Use JSON formatter before config loads — matches the default config
	// format so there's no format mismatch in the log stream.
	log.SetFormatter(&log.JSONFormatter{TimestampFormat: time.RFC3339})
	log.SetLevel(log.InfoLevel)
}

func main() {
	// CLI flags
	configFile := pflag.String("config", "", "Path to config file")
	showVersion := pflag.Bool("version", false, "Show version information")

	// Register config-bound flags
	v := viper.New()
	config.RegisterFlags(pflag.CommandLine, v)
	pflag.Parse()

	if *showVersion {
		fmt.Printf("docker-stats-exporter %s (commit: %s, built: %s, go: %s)\n",
			version, commit, buildDate, runtime.Version())
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Configure logger
	initLogger(cfg.Logging)

	// Set build info for system collector
	collector.Version = version
	collector.Commit = commit
	collector.BuildDate = buildDate

	// Create Docker client
	dockerClient, err := docker.NewClient(cfg.Docker, cfg.Collection.Timeout)
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer dockerClient.Close()

	// Verify Docker connectivity
	if err := dockerClient.Ping(context.Background()); err != nil {
		log.Warnf("Docker daemon not reachable at startup: %v", err)
	} else {
		log.Info("Successfully connected to Docker daemon")
	}

	// Create filter
	filter, err := docker.NewFilter(cfg.Collection.Filters)
	if err != nil {
		log.Fatalf("Failed to create container filter: %v", err)
	}

	// Create cache
	cache := collector.NewStatsCache(cfg.Metrics.Cache.TTL, cfg.Metrics.Cache.Enabled)

	// Create Prometheus registry and register collectors
	registry := prometheus.NewRegistry()

	if cfg.Collection.Collectors.Container {
		cc := collector.NewContainerCollector(dockerClient, filter, cache, cfg)
		registry.MustRegister(cc)
		log.Info("Container collector registered")
	}

	if cfg.Collection.Collectors.System {
		sc := collector.NewSystemCollector(dockerClient, cfg)
		registry.MustRegister(sc)
		log.Info("System collector registered")
	}

	// Start HTTP server
	srv := server.NewServer(cfg.Server, registry, dockerClient)

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.WithField("addr", fmt.Sprintf("%s:%s", cfg.Server.Address, cfg.Server.Port)).Info("Docker Stats Exporter started")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.WithField("signal", sig.String()).Info("Received shutdown signal")

	// Graceful shutdown with 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server shutdown error")
	}

	log.Info("Docker Stats Exporter stopped")
}

func initLogger(cfg config.LoggingConfig) {
	// Set level
	level, err := log.ParseLevel(cfg.Level)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	// Set format
	switch cfg.Format {
	case "json":
		log.SetFormatter(&log.JSONFormatter{TimestampFormat: time.RFC3339})
	default:
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.RFC3339, DisableColors: true})
	}

	// Set output
	switch cfg.Output {
	case "stderr":
		log.SetOutput(os.Stderr)
	default:
		log.SetOutput(os.Stdout)
	}
}
