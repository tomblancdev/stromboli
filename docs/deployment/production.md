# Production Deployment

Best practices for running Stromboli in production.

## Security Checklist

- [ ] Enable authentication
- [ ] Use HTTPS/TLS
- [ ] Enable rate limiting
- [ ] Configure workspace allowlist
- [ ] Set resource limits
- [ ] Use secrets management
- [ ] Enable logging/monitoring
- [ ] Regular security updates

## Authentication

Always enable authentication in production:

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_AUTH_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_AUTH_API_TOKEN="$(openssl rand -base64 32)"
```

## TLS/HTTPS

Use a reverse proxy with TLS:

=== "Traefik"

    ```yaml
    labels:
      - "traefik.http.routers.stromboli.tls=true"
      - "traefik.http.routers.stromboli.tls.certresolver=letsencrypt"
    ```

=== "Nginx"

    ```nginx
    server {
        listen 443 ssl;
        server_name stromboli.example.com;

        ssl_certificate /etc/ssl/certs/stromboli.crt;
        ssl_certificate_key /etc/ssl/private/stromboli.key;

        location / {
            proxy_pass http://localhost:8080;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
    }
    ```

=== "Caddy"

    ```
    stromboli.example.com {
        reverse_proxy localhost:8080
    }
    ```

## Rate Limiting

Protect against abuse:

```bash
export STROMBOLI_RATELIMIT_ENABLED=true
export STROMBOLI_RATELIMIT_REQUESTS_PER_SECOND=5
export STROMBOLI_RATELIMIT_BURST=10
```

## Resource Limits

Set default limits to prevent runaway containers:

```bash
export STROMBOLI_AGENT_DEFAULT_MEMORY=1g
export STROMBOLI_AGENT_DEFAULT_CPUS=2
export STROMBOLI_AGENT_DEFAULT_TIMEOUT=10m
```

## Workspace Security

Restrict accessible directories:

```bash
# Only allow specific paths
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/data/projects,/data/workspaces"
```

Never allow:
- `/` (root)
- `/etc`, `/var`, `/usr`
- Home directories with SSH keys
- Directories with credentials

## Monitoring

### Prometheus Metrics

Stromboli exposes metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'stromboli'
    static_configs:
      - targets: ['stromboli:8080']
```

Key metrics:
- `stromboli_active_containers` - Running containers
- `stromboli_requests_total` - Total requests
- `stromboli_request_duration_seconds` - Request latency

### Logging

Stromboli uses structured JSON logging. Forward to your log aggregator:

```yaml
services:
  stromboli:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### Tracing

Enable OpenTelemetry tracing:

```bash
export STROMBOLI_TRACING_ENABLED=true
export STROMBOLI_TRACING_ENDPOINT=jaeger:4317
```

## High Availability

For HA deployments:

1. **Stateless API**: Stromboli is stateless, scale horizontally
2. **Shared Storage**: Use NFS or distributed storage for sessions
3. **Load Balancer**: Use HAProxy, Nginx, or cloud LB

```yaml
services:
  stromboli-1:
    image: stromboli:latest
    volumes:
      - nfs-sessions:/app/sessions

  stromboli-2:
    image: stromboli:latest
    volumes:
      - nfs-sessions:/app/sessions

  haproxy:
    image: haproxy:latest
    ports:
      - "8080:8080"
    volumes:
      - ./haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg
```

## Backup

### Session Data

Regular backups of session storage:

```bash
# Backup
tar -czf sessions-backup-$(date +%Y%m%d).tar.gz ./sessions

# Restore
tar -xzf sessions-backup-20240101.tar.gz
```

### Podman Secrets

Export secrets for backup:

```bash
podman secret ls --format "{{.Name}}" | while read name; do
  podman secret inspect $name > secrets/$name.json
done
```

## Updates

### Rolling Updates

```bash
# Pull new image
docker compose pull

# Update with zero downtime
docker compose up -d --no-deps stromboli
```

### Version Pinning

Pin to specific versions in production:

```yaml
services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:v0.1.5-alpha
```

## Troubleshooting

### Health Check Failed

```bash
# Check container logs
docker compose logs stromboli

# Check health endpoint
curl http://localhost:8080/health
```

### High Memory Usage

1. Check for orphaned containers: `podman ps -a`
2. Reduce default memory limit
3. Implement session cleanup

### Slow Response Times

1. Check Podman performance: `podman info`
2. Check disk I/O for session storage
3. Enable tracing to identify bottlenecks
