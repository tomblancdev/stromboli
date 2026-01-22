package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	activeContainers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_containers",
			Help: "Number of active containers",
		},
	)
)

// RecordRequest increments the HTTP request counter
func RecordRequest(method, path string, status int) {
	httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
}

// RecordDuration records the HTTP request duration
func RecordDuration(method, path string, durationSeconds float64) {
	httpDuration.WithLabelValues(method, path).Observe(durationSeconds)
}

// SetActiveContainers sets the number of active containers
func SetActiveContainers(count int) {
	activeContainers.Set(float64(count))
}

// IncActiveContainers increments the active containers counter
func IncActiveContainers() {
	activeContainers.Inc()
}

// DecActiveContainers decrements the active containers counter
func DecActiveContainers() {
	activeContainers.Dec()
}
