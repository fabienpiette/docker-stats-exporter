package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
	"github.com/fabienpiette/docker-stats-exporter/pkg/config"
)

// Server is the HTTP server that serves metrics and health endpoints.
type Server struct {
	httpServer *http.Server
	cfg        config.ServerConfig
}

// NewServer creates a configured HTTP server.
func NewServer(cfg config.ServerConfig, registry *prometheus.Registry, dockerClient *docker.Client) *Server {
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	// Health, ready, version
	mux.Handle(cfg.HealthPath, healthHandler())
	mux.Handle(cfg.ReadyPath, readyHandler(dockerClient))
	mux.Handle("/version", versionHandler())

	// Apply middleware stack: recovery → logging → (optional auth) → routes
	var handler http.Handler = mux
	if cfg.Auth.Enabled {
		handler = basicAuthMiddleware(cfg.Auth.Username, cfg.Auth.Password, handler)
	}
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)

	addr := fmt.Sprintf("%s:%s", cfg.Address, cfg.Port)
	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start() error {
	log.WithField("addr", s.httpServer.Addr).Info("Starting HTTP server")

	if s.cfg.TLS.Enabled {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server with a timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("Shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}
