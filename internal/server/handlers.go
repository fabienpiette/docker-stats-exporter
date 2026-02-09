package server

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/fabienpiette/docker-stats-exporter/internal/collector"
	"github.com/fabienpiette/docker-stats-exporter/internal/docker"
)

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func readyHandler(client *docker.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := client.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("NOT READY"))
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("READY"))
	}
}

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

func versionHandler() http.HandlerFunc {
	info := versionInfo{
		Version:   collector.Version,
		Commit:    collector.Commit,
		BuildDate: collector.BuildDate,
		GoVersion: runtime.Version(),
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(info)
	}
}
