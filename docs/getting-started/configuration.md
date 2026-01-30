# Configuration

Stromboli is configured via environment variables or a YAML config file. Environment variables take precedence over config file values.

## Quick Start

For most users, the default configuration works out of the box. You only need:

1. Claude credentials at `~/.claude/.credentials.json` (created by `claude` CLI)
2. Podman socket enabled: `systemctl --user enable --now podman.socket`

## Environment Variables

All environment variables use the `STROMBOLI_` prefix.

### Server Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_SERVER_ADDRESS` | `:8080` | HTTP server listen address |

### Agent Container Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_IMAGE` | `ghcr.io/tomblancdev/stromboli-agent` | Default base container image |
| `STROMBOLI_AGENT_IMAGE_TAG` | `latest` | Default base image tag |
| `STROMBOLI_AGENT_CREDENTIALS_FILE` | `~/.claude/.credentials.json` | Path to Claude credentials |
| `STROMBOLI_AGENT_SESSIONS_DIR` | `.stromboli/sessions` | Session storage directory |

### Claude CLI Image Settings

The Claude CLI is mounted into containers at runtime. This allows using any glibc-based image (Python, Node, Go, etc.) with Claude.

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_MOUNT_CLAUDE_CLI` | `true` | Mount Claude CLI into containers |
| `STROMBOLI_AGENT_CLI_IMAGE` | `ghcr.io/tomblancdev/stromboli-agent` | Image containing Claude CLI |
| `STROMBOLI_AGENT_CLI_IMAGE_TAG` | `latest` | Claude CLI image tag |
| `STROMBOLI_AGENT_AUTO_PULL_CLI` | `true` | Auto-pull CLI image on startup if missing |

### Dynamic Images

Allow users to specify custom container images in API requests.

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS` | (empty) | Comma-separated allowed image patterns (glob syntax) |

Example patterns:
```bash
STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*,ubuntu:*,debian:*"
```

!!! warning "Image Compatibility"
    Alpine and musl-based images are **not supported**. Use glibc-based images:

    - ✅ `python:3.12` (Debian-based)
    - ✅ `node:20` (Debian-based)
    - ✅ `golang:1.22` (Debian-based)
    - ❌ `python:3.12-alpine`
    - ❌ `node:20-alpine`

### Resource Limits

Default limits for agent containers. Can be overridden per-request.

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_RESOURCES_MEMORY` | `512m` | Memory limit (e.g., `512m`, `2g`) |
| `STROMBOLI_RESOURCES_CPUS` | `1` | CPU limit (e.g., `0.5`, `2`) |
| `STROMBOLI_RESOURCES_TIMEOUT` | `30m` | Execution timeout (e.g., `5m`, `1h`) |

### Token Cache

Reduce file reads by caching credentials in memory.

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_TOKEN_CACHE_ENABLED` | `true` | Enable token caching |
| `STROMBOLI_TOKEN_CACHE_TTL` | `5m` | Cache time-to-live |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AUTH_ENABLED` | `false` | Enable authentication |
| `STROMBOLI_API_TOKENS` | (none) | Comma-separated static API tokens |

### JWT Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_JWT_SECRET` | (none) | JWT signing secret (generate with `openssl rand -base64 32`) |
| `STROMBOLI_JWT_EXPIRY` | `24h` | Access token lifetime |
| `STROMBOLI_JWT_REFRESH_EXPIRY` | `168h` | Refresh token lifetime (7 days) |

### Rate Limiting

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `STROMBOLI_RATE_LIMIT_RPS` | `10` | Requests per second |
| `STROMBOLI_RATE_LIMIT_BURST` | `20` | Burst allowance |

### Job Management

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_JOBS_CLEANUP_TTL` | `1h` | How long to keep completed jobs |
| `STROMBOLI_JOBS_CLEANUP_INTERVAL` | `5m` | Cleanup check interval |

### Observability

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `STROMBOLI_TRACING_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint |
| `STROMBOLI_TRACING_SERVICE_NAME` | `stromboli` | Service name in traces |
| `STROMBOLI_TRACING_INSECURE` | `true` | Use insecure connection (no TLS) |

## Config File

Use a YAML config file for cleaner configuration:

```yaml
# stromboli.yaml

server:
  address: ":8080"

agent:
  # Base container image
  image: "ghcr.io/tomblancdev/stromboli-agent"
  image_tag: "latest"

  # Claude CLI mounting (for dynamic images)
  mount_claude_cli: true
  cli_image: "ghcr.io/tomblancdev/stromboli-agent"
  cli_image_tag: "latest"
  auto_pull_cli: true

  # Credentials and sessions
  credentials_file: "~/.claude/.credentials.json"
  sessions_dir: ".stromboli/sessions"

  # Dynamic images (glob patterns)
  allowed_image_patterns:
    - "python:*"
    - "node:*"
    - "golang:*"

  # Token cache
  token_cache:
    enabled: true
    ttl: "5m"

resources:
  memory: "512m"
  cpus: "1"
  timeout: "30m"

auth:
  enabled: false
  valid_tokens: []

jwt:
  secret: ""
  access_expiry: "24h"
  refresh_expiry: "168h"

rate_limit:
  enabled: false
  rate: 10
  burst: 20

jobs:
  cleanup_ttl: "1h"
  cleanup_interval: "5m"

tracing:
  enabled: false
  service_name: "stromboli"
  endpoint: "localhost:4317"
  insecure: true
```

Load with:
```bash
stromboli --config stromboli.yaml
```

## Example Configurations

### Development (Minimal)

```bash
# Just ensure Claude credentials exist
# ~/.claude/.credentials.json (created by 'claude' CLI)

# Start with defaults
stromboli
```

### Development with Dynamic Images

```bash
export STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*"
stromboli
```

### Production

```bash
# Authentication
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_JWT_EXPIRY=24h

# Rate limiting
export STROMBOLI_RATE_LIMIT_ENABLED=true
export STROMBOLI_RATE_LIMIT_RPS=50
export STROMBOLI_RATE_LIMIT_BURST=100

# Resources
export STROMBOLI_RESOURCES_MEMORY=2g
export STROMBOLI_RESOURCES_CPUS=2
export STROMBOLI_RESOURCES_TIMEOUT=1h

# Allowed images
export STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS="python:*,node:*,golang:*,ubuntu:*,debian:*"

stromboli
```

### Docker Compose

See the full example in `install/docker-compose.yml`:

```yaml
services:
  stromboli:
    image: ghcr.io/tomblancdev/stromboli:latest
    ports:
      - "8080:8080"
    volumes:
      # Podman socket
      - ${XDG_RUNTIME_DIR:-/run/user/${UID:-1000}}/podman/podman.sock:/run/podman/podman.sock
      # Claude credentials
      - ${HOME}/.claude:/home/stromboli/.claude:ro
      # Session persistence
      - stromboli-sessions:/app/sessions
    environment:
      STROMBOLI_SERVER_ADDRESS: ":8080"
      STROMBOLI_AGENT_IMAGE: "ghcr.io/tomblancdev/stromboli-agent"
      STROMBOLI_AGENT_MOUNT_CLAUDE_CLI: "true"
      STROMBOLI_AGENT_AUTO_PULL_CLI: "true"
    userns_mode: keep-id  # For rootless Podman

volumes:
  stromboli-sessions:
```

## Startup Behavior

On startup, Stromboli:

1. **Loads configuration** from environment variables and/or config file
2. **Checks Claude CLI image** (if `mount_claude_cli: true`):
   - If image exists locally → uses it
   - If missing and `auto_pull_cli: true` → pulls from registry
   - If missing and `auto_pull_cli: false` → logs warning
3. **Validates Claude credentials** → warns if not found
4. **Cleans up orphaned containers** from previous runs
5. **Starts HTTP server**

## Next Steps

- [Running Agents](../guide/running-agents.md) - Learn how to run agents
- [Authentication](../api/authentication.md) - Set up JWT auth for production
- [API Reference](../api/overview.md) - Full API documentation
