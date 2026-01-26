# Configuration

Stromboli is configured via environment variables or a config file.

## Environment Variables

### Server Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_PORT` | `8080` | HTTP server port |
| `STROMBOLI_HOST` | `0.0.0.0` | Bind address |

### Agent Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_IMAGE` | `stromboli-agent:latest` | Default container image |
| `STROMBOLI_AGENT_SECRETS_FILE` | `~/.claude/.credentials.json` | Claude credentials path |
| `STROMBOLI_AGENT_SESSIONS_DIR` | `./sessions` | Session storage directory |
| `STROMBOLI_AGENT_ALLOWED_WORKSPACES` | (none) | Comma-separated allowed paths |

### Resource Limits

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_DEFAULT_MEMORY` | (none) | Default memory limit (e.g., `512m`) |
| `STROMBOLI_AGENT_DEFAULT_CPUS` | (none) | Default CPU limit (e.g., `1.0`) |
| `STROMBOLI_AGENT_DEFAULT_TIMEOUT` | (none) | Default timeout (e.g., `5m`) |

### Dynamic Images

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS` | (none) | Allowed image patterns (e.g., `python:*,golang:*`) |
| `STROMBOLI_AGENT_MOUNT_CLAUDE_CLI` | `false` | Mount Claude CLI into custom images |

### Authentication

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AUTH_ENABLED` | `false` | Enable JWT authentication |
| `STROMBOLI_AUTH_JWT_SECRET` | (none) | JWT signing secret |
| `STROMBOLI_AUTH_API_TOKEN` | (none) | API token for token generation |

### Rate Limiting

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_RATELIMIT_ENABLED` | `false` | Enable rate limiting |
| `STROMBOLI_RATELIMIT_REQUESTS_PER_SECOND` | `10` | Requests per second |
| `STROMBOLI_RATELIMIT_BURST` | `20` | Burst allowance |

### Observability

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `STROMBOLI_TRACING_ENDPOINT` | `localhost:4317` | OTLP endpoint |
| `STROMBOLI_TRACING_SERVICE_NAME` | `stromboli` | Service name for traces |

## Example Configurations

### Development

```bash
export STROMBOLI_PORT=8080
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/home/user/projects"
```

### Production

```bash
export STROMBOLI_PORT=8080
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_AUTH_JWT_SECRET="your-secure-secret"
export STROMBOLI_AUTH_API_TOKEN="your-api-token"
export STROMBOLI_RATELIMIT_ENABLED=true
export STROMBOLI_RATELIMIT_REQUESTS_PER_SECOND=5
export STROMBOLI_AGENT_DEFAULT_MEMORY="1g"
export STROMBOLI_AGENT_DEFAULT_TIMEOUT="10m"
export STROMBOLI_AGENT_ALLOWED_WORKSPACES="/data/projects"
```

### Docker Compose

```yaml
services:
  stromboli:
    image: stromboli:latest
    environment:
      - STROMBOLI_PORT=8080
      - STROMBOLI_AGENT_ALLOWED_WORKSPACES=/workspace
      - STROMBOLI_AGENT_DEFAULT_MEMORY=512m
      - STROMBOLI_AGENT_DEFAULT_TIMEOUT=5m
    volumes:
      - /run/podman/podman.sock:/run/podman/podman.sock
      - ~/.claude:/app/.claude:ro
      - ./sessions:/app/sessions
    ports:
      - "8080:8080"
```

## Config File

You can also use a YAML config file:

```yaml
# stromboli.yaml
server:
  port: 8080
  host: 0.0.0.0

agent:
  image: stromboli-agent:latest
  secrets_file: ~/.claude/.credentials.json
  sessions_dir: ./sessions
  allowed_workspaces:
    - /home/user/projects
    - /data/workspaces
  defaults:
    memory: 512m
    cpus: "1.0"
    timeout: 5m

auth:
  enabled: true
  jwt_secret: ${JWT_SECRET}
  api_token: ${API_TOKEN}

ratelimit:
  enabled: true
  requests_per_second: 10
  burst: 20
```

Load with:
```bash
stromboli --config stromboli.yaml
```
