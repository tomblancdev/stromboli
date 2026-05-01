# Configuration

Stromboli is configured via environment variables or a YAML config file. Environment variables take precedence.

## Essentials

For most setups, you only need two things:

1. Claude credentials at `~/.claude/.credentials.json` (created by `claude` CLI)
2. Podman socket enabled: `systemctl --user enable --now podman.socket`

Everything else has sensible defaults.

## Common settings

These are the settings you'll most likely want to change:

### Volume allowlist

Controls which host directories agents can mount. **Empty = all mounts denied** (secure default).

```bash
STROMBOLI_AGENT_ALLOWED_VOLUMES="/home/user/projects,/data/workspaces"
```

### Resource limits

Default limits for agent containers (overridable per-request):

```bash
STROMBOLI_RESOURCES_MEMORY=512m     # Memory limit
STROMBOLI_RESOURCES_CPUS=1          # CPU limit
STROMBOLI_RESOURCES_TIMEOUT=30m     # Execution timeout
```

### Custom images

Allow users to specify container images in API requests:

```bash
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*,ubuntu:*"
```

### Authentication

```bash
STROMBOLI_AUTH_ENABLED=true
STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
```

### Rate limiting

```bash
STROMBOLI_RATE_LIMIT_ENABLED=true
STROMBOLI_RATE_LIMIT_RPS=10
STROMBOLI_RATE_LIMIT_BURST=20
```

## All environment variables

### Server

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_SERVER_ADDRESS` | `:8080` | Listen address |

### Agent

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_AGENT_IMAGE` | `ghcr.io/tomblancdev/stromboli-agent` | Default base image |
| `STROMBOLI_AGENT_IMAGE_TAG` | `latest` | Default base image tag |
| `STROMBOLI_AGENT_CREDENTIALS_FILE` | `~/.claude/.credentials.json` | Claude credentials path |
| `STROMBOLI_AGENT_SESSIONS_DIR` | `.stromboli/sessions` | Session storage (internal) |
| `STROMBOLI_AGENT_SESSIONS_HOST_DIR` | (same as SESSIONS_DIR) | Session storage (host path, for containerized deployment) |
| `STROMBOLI_AGENT_ALLOWED_VOLUMES` | (empty) | Allowed volume host paths (comma-separated) |
| `STROMBOLI_AGENT_ALLOW_ALL_VOLUMES` | `false` | Allow all paths (DANGEROUS — dev only) |
| `STROMBOLI_AGENT_WORKDIR_AUTO_CREATE` | `true` | Auto-create workdir inside container |
| `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS` | (empty) | Allowed image patterns (glob, comma-separated) |

### CLI image

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_AGENT_MOUNT_CLAUDE_CLI` | `true` | Mount Claude CLI into containers |
| `STROMBOLI_AGENT_CLI_IMAGE` | `ghcr.io/tomblancdev/stromboli-agent` | CLI source image |
| `STROMBOLI_AGENT_CLI_IMAGE_TAG` | `latest` | CLI image tag |
| `STROMBOLI_AGENT_AUTO_PULL_CLI` | `true` | Auto-pull CLI image on startup |

### Resources

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_RESOURCES_MEMORY` | `512m` | Memory limit |
| `STROMBOLI_RESOURCES_CPUS` | `1` | CPU limit |
| `STROMBOLI_RESOURCES_TIMEOUT` | `30m` | Execution timeout |

### Authentication

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_AUTH_ENABLED` | `true` | Enable authentication. Server fails fast at startup if no JWT secret is configured (or it's a placeholder). Disable only for local dev. |
| `STROMBOLI_API_TOKENS` | (none) | Static API tokens (comma-separated) |
| `STROMBOLI_JWT_SECRET` | (none) | JWT signing secret (required when auth is enabled — minimum 32 chars, no placeholders) |
| `STROMBOLI_JWT_EXPIRY` | `24h` | Access token lifetime |
| `STROMBOLI_JWT_REFRESH_EXPIRY` | `168h` | Refresh token lifetime |

### Rate limiting

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `STROMBOLI_RATE_LIMIT_RPS` | `10` | Requests per second |
| `STROMBOLI_RATE_LIMIT_BURST` | `20` | Burst allowance |

### Jobs

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_JOBS_CLEANUP_TTL` | `1h` | Keep completed jobs for |
| `STROMBOLI_JOBS_CLEANUP_INTERVAL` | `5m` | Cleanup check interval |

### Observability

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_LOG_FORMAT` | `json` | Log encoder (`json` for log aggregators, `text` for humans) |
| `STROMBOLI_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `STROMBOLI_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `STROMBOLI_TRACING_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint |
| `STROMBOLI_TRACING_SERVICE_NAME` | `stromboli` | Service name in traces |
| `STROMBOLI_TRACING_INSECURE` | `false` | Disable TLS for the OTLP exporter (insecure — dev only) |
| `STROMBOLI_METRICS_ENABLED` | `true` | Expose Prometheus metrics on a separate listener |
| `STROMBOLI_METRICS_ADDRESS` | `127.0.0.1:9090` | Metrics listener bind address. Defaults to localhost so a forgotten port doesn't expose metrics publicly — front it with a sidecar or reverse proxy if you need cross-namespace scraping. |
| `STROMBOLI_TOKEN_CACHE_ENABLED` | `true` | Cache credentials in memory |
| `STROMBOLI_TOKEN_CACHE_TTL` | `5m` | Cache TTL |

### Compose environments

| Variable | Default | Description |
|---|---|---|
| `STROMBOLI_COMPOSE_ALLOW_PRIVILEGED` | `false` | Allow privileged containers |
| `STROMBOLI_COMPOSE_ALLOW_HOST_NETWORK` | `false` | Allow host network mode |
| `STROMBOLI_COMPOSE_ALLOW_HOST_VOLUMES` | `false` | Allow host volume mounts |
| `STROMBOLI_COMPOSE_BUILD_TIMEOUT` | `10m` | Max compose build/up time |
| `STROMBOLI_COMPOSE_HEALTH_TIMEOUT` | `2m` | Max health check wait time |
| `STROMBOLI_COMPOSE_STACK_TTL` | `1h` | Orphaned stack max age |

## YAML config file

For a cleaner setup, use a YAML file:

```yaml
# stromboli.yaml
server:
  address: ":8080"

agent:
  image: "ghcr.io/tomblancdev/stromboli-agent"
  image_tag: "latest"
  mount_claude_cli: true
  cli_image: "ghcr.io/tomblancdev/stromboli-agent"
  auto_pull_cli: true
  credentials_file: "~/.claude/.credentials.json"
  sessions_dir: ".stromboli/sessions"
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "golang:*"
  allowed_volumes:
    - "/home/user/projects"
  token_cache:
    enabled: true
    ttl: "5m"

resources:
  memory: "512m"
  cpus: "1"
  timeout: "30m"

auth:
  enabled: true   # default; server fails fast without a real JWT secret

jwt:
  secret: ""              # set via STROMBOLI_JWT_SECRET (≥32 chars, no placeholders)
  access_expiry: "24h"
  refresh_expiry: "168h"

rate_limit:
  enabled: false  # opt-in
  rate: 10
  burst: 20

tracing:
  enabled: false
  service_name: "stromboli"
  endpoint: "localhost:4317"
  insecure: false   # TLS by default — set to true only for local dev collectors

jobs:
  cleanup_ttl: "1h"
  cleanup_interval: "5m"

compose:
  allow_privileged: false
  allow_host_network: false
  allow_host_volumes: false
  build_timeout: "10m"
  health_timeout: "2m"
  stack_ttl: "1h"
```

Load with:

```bash
stromboli --config stromboli.yaml
```

## Example: production

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_RATE_LIMIT_ENABLED=true
export STROMBOLI_RATE_LIMIT_RPS=50
export STROMBOLI_RESOURCES_MEMORY=2g
export STROMBOLI_RESOURCES_CPUS=2
export STROMBOLI_RESOURCES_TIMEOUT=1h
export STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*"
export STROMBOLI_AGENT_ALLOWED_VOLUMES="/data/projects,/home/user/workspaces"
```

See [production hardening](../security/production.md) for the full checklist.

## Startup behavior

On startup, Stromboli:

1. Loads config from environment variables and/or config file
2. Checks for the CLI image (pulls if missing and `auto_pull_cli: true`)
3. Validates Claude credentials (warns if not found)
4. Cleans up orphaned containers from previous runs
5. Starts the HTTP server
