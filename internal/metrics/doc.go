// Package metrics provides Prometheus metrics for Stromboli monitoring.
//
// # Available Metrics
//
// HTTP Metrics:
//   - http_requests_total: Counter of HTTP requests by method, path, and status
//   - http_request_duration_seconds: Histogram of request durations
//
// Container Metrics:
//   - active_containers: Gauge of currently running containers
//
// # Usage
//
// Record HTTP request:
//
//	metrics.RecordRequest("POST", "/run", 200)
//	metrics.RecordDuration("POST", "/run", 1.5)
//
// Track containers:
//
//	metrics.IncActiveContainers()
//	defer metrics.DecActiveContainers()
//
// # Prometheus Endpoint
//
// Metrics are exposed at /metrics endpoint automatically via promhttp.Handler().
// Use Grafana dashboard in deployments/grafana/ for visualization.
package metrics
