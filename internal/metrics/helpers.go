package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// NanosecondsToSeconds converts nanosecond counters to seconds for Prometheus.
const NanosecondsToSeconds = 1e-9

// HealthStatusToFloat maps Docker health status strings to numeric values.
func HealthStatusToFloat(status string) float64 {
	switch status {
	case "starting":
		return 1
	case "healthy":
		return 2
	case "unhealthy":
		return 3
	default:
		return 0 // none / no healthcheck
	}
}

// SafeNewConstMetric creates a const metric, recovering from panics on bad label counts.
// Returns nil if metric creation fails.
func SafeNewConstMetric(desc *prometheus.Desc, valueType prometheus.ValueType, value float64, labelValues ...string) prometheus.Metric {
	m, err := prometheus.NewConstMetric(desc, valueType, value, labelValues...)
	if err != nil {
		log.WithError(err).WithField("desc", desc.String()).Warn("Failed to create metric")
		return nil
	}
	return m
}

// SendSafe sends a metric to the channel if it is non-nil.
func SendSafe(ch chan<- prometheus.Metric, m prometheus.Metric) {
	if m != nil {
		ch <- m
	}
}
