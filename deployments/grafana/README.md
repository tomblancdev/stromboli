# Stromboli Grafana Dashboard

Pre-built Grafana dashboard for monitoring Stromboli API.

## Quick Start

### 1. Import Dashboard

In Grafana:
1. Go to **Dashboards** > **Import**
2. Upload `stromboli-dashboard.json` or paste its contents
3. Select your Prometheus datasource
4. Click **Import**

### 2. Configure Prometheus

Add Stromboli as a scrape target in `prometheus.yml`. Note that `/metrics`
is served on a separate listener (default `127.0.0.1:9090`) — point the
scraper there, not at the API port:

```yaml
scrape_configs:
  - job_name: 'stromboli'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'

rule_files:
  - 'prometheus-rules.yaml'
```

### 3. Load alerting rules

`prometheus-rules.yaml` ships alongside the dashboard with starter alerts
for API error rate, latency, memory pressure, goroutine leaks, and stuck
container cleanup. Tune the thresholds to your workload before paging
on them. For kube-prometheus-stack users, wrap the `groups:` block in a
`PrometheusRule` CR.

## Dashboard Panels

### Overview Row

| Panel | Description |
|-------|-------------|
| **Request Rate** | Requests per second (total, success, errors) |
| **Active Containers** | Gauge showing running Claude containers |
| **Error Rate** | Percentage of 5xx responses |

### Latency Row

| Panel | Description |
|-------|-------------|
| **Request Latency Percentiles** | p50, p95, p99 response times |
| **P95 Latency by Endpoint** | Slowest endpoints at 95th percentile |

### Endpoints Row

| Panel | Description |
|-------|-------------|
| **Requests by Endpoint** | Traffic distribution across API routes |
| **Requests by Status Code** | Response codes (2xx green, 4xx yellow, 5xx red) |

### Jobs Row

| Panel | Description |
|-------|-------------|
| **Container Activity Over Time** | Historical container usage |

## Metrics Reference

### `http_requests_total` (Counter)

Total HTTP requests processed.

**Labels:**
- `method` - HTTP method (GET, POST, DELETE)
- `path` - API endpoint (/run, /health, /jobs, etc.)
- `status` - HTTP status code (200, 400, 500, etc.)

**Example queries:**
```promql
# Request rate
rate(http_requests_total{job="stromboli"}[5m])

# Error rate percentage
100 * sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))

# Requests by endpoint
sum(rate(http_requests_total[5m])) by (path)
```

### `http_request_duration_seconds` (Histogram)

Request latency distribution.

**Labels:**
- `method` - HTTP method
- `path` - API endpoint

**Example queries:**
```promql
# P95 latency
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# Average latency
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# P99 by endpoint
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))
```

### `active_containers` (Gauge)

Number of currently running Claude containers.

**Example queries:**
```promql
# Current count
active_containers{job="stromboli"}

# Max over time range
max_over_time(active_containers[1h])
```

## Alerting Rules

Recommended alerts (add to Prometheus/Alertmanager):

```yaml
groups:
  - name: stromboli
    rules:
      - alert: HighErrorRate
        expr: |
          100 * sum(rate(http_requests_total{job="stromboli", status=~"5.."}[5m]))
          / sum(rate(http_requests_total{job="stromboli"}[5m])) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate on Stromboli API"
          description: "Error rate is {{ $value }}%"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{job="stromboli"}[5m])) by (le)) > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency on Stromboli API"
          description: "P95 latency is {{ $value }}s"

      - alert: TooManyContainers
        expr: active_containers{job="stromboli"} > 50
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Too many active containers"
          description: "{{ $value }} containers running"
```

## Customization

The dashboard uses a `${datasource}` variable - select your Prometheus instance from the dropdown at the top.

To customize:
1. Edit panels in Grafana
2. Export updated JSON via **Share** > **Export** > **Save to file**
3. Replace `stromboli-dashboard.json`
