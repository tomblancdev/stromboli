# Production Hardening

A checklist-driven guide to deploying Stromboli safely in production.

## Required checklist

These are non-negotiable for any production deployment:

- [ ] **Enable authentication** — `STROMBOLI_AUTH_ENABLED=true`
- [ ] **Set a strong JWT secret** — `STROMBOLI_JWT_SECRET=$(openssl rand -base64 32)`
- [ ] **Enable TLS** — Use a reverse proxy with HTTPS (see below)
- [ ] **Enable rate limiting** — `STROMBOLI_RATE_LIMIT_ENABLED=true`
- [ ] **Set volume allowlist** — `STROMBOLI_AGENT_ALLOWED_VOLUMES=/path1,/path2`
- [ ] **Use rootless Podman** — `systemctl --user enable --now podman.socket`
- [ ] **Set image allowlist** — `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS=python:*,node:*`

!!! danger "Volume security"
    **Never** set `STROMBOLI_AGENT_ALLOW_ALL_VOLUMES=true` in production. This disables all volume validation.

## Recommended checklist

- [ ] Set resource limits (memory, CPU, timeout)
- [ ] Enable monitoring (Prometheus metrics, structured logging)
- [ ] Set up alerting (error rates, rate limit hits)
- [ ] Pin image versions (avoid `:latest` in production)
- [ ] Back up session data
- [ ] Restrict network access to `/metrics` and `/health`
- [ ] Configure compose security (all `allow_*: false`)
- [ ] Rotate JWT secrets periodically

## TLS setup

Always terminate TLS at a reverse proxy. Stromboli itself doesn't handle TLS.

=== "Caddy"

    The simplest option — automatic HTTPS with Let's Encrypt:

    ```
    stromboli.example.com {
        reverse_proxy localhost:8080
    }
    ```

=== "Traefik"

    ```yaml
    services:
      traefik:
        image: traefik:v2.10
        command:
          - "--providers.docker=true"
          - "--entrypoints.websecure.address=:443"
          - "--certificatesresolvers.le.acme.tlschallenge=true"
          - "--certificatesresolvers.le.acme.email=you@example.com"
          - "--certificatesresolvers.le.acme.storage=/letsencrypt/acme.json"
        ports:
          - "443:443"
        volumes:
          - /var/run/docker.sock:/var/run/docker.sock:ro
          - ./letsencrypt:/letsencrypt

      stromboli:
        image: ghcr.io/tomblancdev/stromboli:latest
        labels:
          - "traefik.enable=true"
          - "traefik.http.routers.stromboli.rule=Host(`stromboli.example.com`)"
          - "traefik.http.routers.stromboli.tls.certresolver=le"
    ```

=== "Nginx"

    ```nginx
    server {
        listen 443 ssl http2;
        server_name stromboli.example.com;

        ssl_certificate /etc/letsencrypt/live/stromboli.example.com/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/stromboli.example.com/privkey.pem;
        ssl_protocols TLSv1.2 TLSv1.3;

        add_header Strict-Transport-Security "max-age=63072000" always;
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-Frame-Options "DENY" always;

        location / {
            proxy_pass http://localhost:8080;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
    ```

## Monitoring

### Prometheus

Stromboli exposes metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'stromboli'
    static_configs:
      - targets: ['stromboli:8080']
```

### Tracing

Enable OpenTelemetry for request tracing:

```bash
STROMBOLI_TRACING_ENABLED=true
STROMBOLI_TRACING_ENDPOINT=jaeger:4317
STROMBOLI_TRACING_SERVICE_NAME=stromboli-prod
```

### Logging

Stromboli outputs structured JSON logs. Configure log rotation in your container runtime:

```yaml
services:
  stromboli:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## High availability

Stromboli is stateless — scale horizontally behind a load balancer:

1. Use shared storage (NFS or distributed filesystem) for session persistence
2. Put multiple Stromboli instances behind HAProxy, Nginx, or a cloud load balancer
3. Each instance needs access to the same Podman socket and session directory

## Backups

### Sessions

```bash
tar -czf sessions-backup-$(date +%Y%m%d).tar.gz /path/to/sessions
```

### Secrets

```bash
podman secret ls --format "{{.Name}}" | while read name; do
  podman secret inspect "$name" > "secrets/$name.json"
done
```

## Updates

Pin to specific versions and use rolling updates:

```yaml
services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:v0.3.0-alpha
```

```bash
docker compose pull && docker compose up -d --no-deps stromboli
```

## Operational security checklist

- [ ] Rotate JWT secrets periodically
- [ ] Rotate Claude API credentials as needed
- [ ] Monitor for failed authentication attempts
- [ ] Review container images for vulnerabilities
- [ ] Keep Podman and host OS updated
- [ ] Implement log retention policy
- [ ] Set up alerting for anomalous activity
