# Production Hardening

A checklist-driven guide to deploying Stromboli safely in production.

!!! info "Upgrading from v0.4.x?"
    v0.5.0-alpha changed several defaults to the secure side (auth on, tracing TLS on, metrics on a separate localhost listener, JSON logs). Read the [v0.5.0 upgrade guide](../getting-started/upgrade-to-0.5.0.md) before bouncing the server — a few of those defaults can fail-fast at startup if you're not ready.

## Required checklist

These are non-negotiable for any production deployment:

- [ ] **Set a strong JWT secret** — `STROMBOLI_JWT_SECRET=$(openssl rand -base64 32)`. Auth is on by default in v0.5.0+; the server fails fast at startup without a real secret.
- [ ] **Enable TLS** — Use a reverse proxy with HTTPS (see below)
- [ ] **Enable rate limiting** — `STROMBOLI_RATE_LIMIT_ENABLED=true`
- [ ] **Configure trusted proxies** — `STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES=10.0.0.0/8` (or your actual proxy CIDR). Without this, rate-limit identity can be spoofed via `X-Forwarded-For` from the public internet.
- [ ] **Set volume allowlist** — `STROMBOLI_AGENT_ALLOWED_VOLUMES=/path1,/path2`
- [ ] **Use rootless Podman** — `systemctl --user enable --now podman.socket`
- [ ] **Set image allowlist** — `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS=python:*,node:*`
- [ ] **Sign outgoing webhooks** — `STROMBOLI_WEBHOOK_SIGNING_SECRET=$(openssl rand -base64 32)` if any deployment uses `webhook_url`. Receivers MUST verify or you have a forgery vulnerability.
- [ ] **Verify metrics binding** — `/metrics` defaults to `127.0.0.1:9090` on its own listener. Don't override to `0.0.0.0:9090` without a NetworkPolicy.
- [ ] **Verify tracing TLS** — `STROMBOLI_TRACING_INSECURE` defaults to `false`. Only flip to `true` for localhost collectors.

!!! danger "Volume security"
    **Never** set `STROMBOLI_AGENT_ALLOW_ALL_VOLUMES=true` in production. This disables all volume validation.

!!! danger "Auth opt-out"
    **Never** set `STROMBOLI_AUTH_ENABLED=false` in production. The fail-fast JWT-secret check is the gate that keeps a misconfigured deploy from accidentally exposing an unauthenticated API.

## Recommended checklist

- [ ] Set resource limits (memory, CPU, timeout)
- [ ] Enable monitoring (Prometheus metrics on the localhost listener, JSON logs)
- [ ] Set up alerting (error rates, rate limit hits, blacklist backend failures returning 503)
- [ ] Pin image versions (avoid `:latest` in production)
- [ ] Back up session data **and** the bolt blacklist file (if used)
- [ ] Configure compose security (all `allow_*: false`)
- [ ] Rotate JWT secrets periodically — on rotation, every active session is invalidated
- [ ] Pick a token blacklist backend deliberately (memory vs. bolt — see below)
- [ ] Tail JSON logs into your aggregator with the `STROMBOLI_*` field structure

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
    image: ghcr.io/tomblancdev/stromboli:v0.5.0-alpha
```

```bash
docker compose pull && docker compose up -d --no-deps stromboli
```

When upgrading across a minor version (e.g. 0.4.x → 0.5.0), read the matching upgrade guide first — defaults sometimes flip in ways that fail-fast at startup. The [v0.5.0 upgrade guide](../getting-started/upgrade-to-0.5.0.md) is the playbook for the most recent jump.

## Token blacklist: choose a backend

Logout (`POST /auth/logout`) adds the token's JTI to a blacklist so the JWT is rejected on subsequent requests. Stromboli ships two storage backends; pick deliberately:

| Backend | Survives restart? | Multi-process safe? | Best for |
|---|---|---|---|
| `memory` (default) | ❌ | ❌ | Single-instance deployments with short access-token TTLs (e.g. 1h) — the practical impact of "logout doesn't survive restart" is bounded. |
| `bolt` | ✅ | ❌ | Single-instance deployments where logout MUST survive restart. Backed by a single file (`STROMBOLI_AUTH_BLACKLIST_BOLT_PATH`). Back this file up alongside session data. |

Configure via:

```bash
export STROMBOLI_AUTH_BLACKLIST_BACKEND=bolt
export STROMBOLI_AUTH_BLACKLIST_BOLT_PATH=/var/lib/stromboli/blacklist.db
export STROMBOLI_AUTH_BLACKLIST_CLEANUP_INTERVAL=1h
```

Neither backend is multi-process safe yet — if you horizontally scale Stromboli (multiple replicas behind a load balancer), each replica has its own blacklist and a logout on one won't be visible to peers. For now, either accept that limitation (a logged-out token continues working until natural expiry on other replicas) or pin per-tenant traffic via session affinity. A shared backend (Redis) is the next planned iteration.

The auth middleware **fails closed** on backend errors: a bolt I/O failure returns `503 auth backend unavailable` rather than admitting the request. Alert on this status code — sustained 503s mean the blacklist file is unreachable.

## Webhook signing

If your deployment uses `webhook_url` on `/run/async`, configure signing:

```bash
export STROMBOLI_WEBHOOK_SIGNING_SECRET="$(openssl rand -base64 32)"
```

Every outgoing callback then carries `X-Stromboli-Signature` (`sha256=<hex>`) and `X-Stromboli-Timestamp` headers. Receivers verify with HMAC-SHA256 over `timestamp + "." + body` and a freshness window. See the [webhook security guide](../guides/webhook-security.md) for receiver-side code in Go / Python / Node.

If you forget to set the secret, Stromboli logs a loud `WARN` on startup so the missing config is visible. **Don't ignore it in production** — unsigned webhooks are forgeable by anyone who can guess the receiver URL.

## Rate limiting and X-Forwarded-For

Stromboli's rate limiter buckets per client IP. If you sit Stromboli behind a reverse proxy (Caddy, Nginx, an LB) without configuring trusted proxies, every request looks like it came from the proxy's IP — your per-IP rate limit becomes a per-cluster rate limit.

Configure the trusted-proxy CIDR(s):

```bash
# Single proxy
export STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES="10.0.0.5/32"

# Whole private network
export STROMBOLI_RATE_LIMIT_TRUSTED_PROXIES="10.0.0.0/8,172.16.0.0/12"
```

When the request's immediate peer is in the allowlist, the leftmost `X-Forwarded-For` entry becomes the bucket key. From any other source, forwarding headers are ignored (the immediate peer is used).

!!! danger "Don't allow 0.0.0.0/0"
    Listing `0.0.0.0/0` in `TRUSTED_PROXIES` is the same as having no allowlist at all — anyone can spoof their bucket via a crafted `X-Forwarded-For` header. List only the actual CIDRs your proxy sits in.

## Operational security checklist

- [ ] Rotate JWT secrets periodically
- [ ] Rotate Claude API credentials as needed
- [ ] Monitor for failed authentication attempts (`401`)
- [ ] Monitor for blacklist backend failures (`503 auth backend unavailable`)
- [ ] Monitor for sustained `429` rate-limit responses (potential abuse OR misconfigured trusted proxies)
- [ ] Review container images for vulnerabilities
- [ ] Keep Podman and host OS updated
- [ ] Implement log retention policy on the JSON log stream
- [ ] Set up alerting for anomalous activity
