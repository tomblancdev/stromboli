# Stromboli User Guide

> Complete guide to using Stromboli - Claude Code Container Orchestration API

**Version**: 1.4
**Last Updated**: January 23, 2026

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Core Concepts](#core-concepts)
3. [API Overview](#api-overview)
4. [Execution Modes](#execution-modes)
5. [Authentication](#authentication)
6. [Session Management](#session-management)
7. [Configuration](#configuration)
8. [Observability](#observability)
9. [Best Practices](#best-practices)
10. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Prerequisites

- Podman installed and running
- Claude API token (from [Anthropic Console](https://console.anthropic.com))

### Setup in 3 Steps

```bash
# 1. Configure Claude token
make claude-setup
# Paste your token when prompted

# 2. Build and run
make build
make run

# 3. Test it works
curl http://localhost:8080/health
```

### Your First Request

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "claude": {
      "dangerously_skip_permissions": true
    }
  }'
```

---

## Core Concepts

### What is Stromboli?

Stromboli is a REST API that orchestrates Claude Code instances in isolated Podman containers. It provides:

- **Isolation**: Each request runs in its own container
- **Security**: Workspace allowlisting, token management via Podman secrets
- **Flexibility**: Sync, async, and streaming execution modes
- **Sessions**: Multi-turn conversations with context preservation

### Architecture Overview

```
Client Request
     |
     v
+-------------+
|  Stromboli  | <-- REST API (Gin)
|    API      |
+-------------+
     |
     v
+-------------+
|   Podman    | <-- Container Runtime
+-------------+
     |
     v
+------------------+
| Claude Container | <-- Isolated Execution
| - Claude Code    |
| - Workspace      |
+------------------+
```

---

## API Overview

### Base URL

```
http://localhost:8080
```

### Endpoints Summary

| Category | Endpoint | Method | Description |
|----------|----------|--------|-------------|
| **Execution** | `/run` | POST | Synchronous execution |
| | `/run/async` | POST | Asynchronous execution |
| | `/run/stream` | GET | Streaming (SSE) |
| **Jobs** | `/jobs` | GET | List async jobs |
| | `/jobs/{id}` | GET | Get job status |
| | `/jobs/{id}` | DELETE | Cancel job |
| **Sessions** | `/sessions` | GET | List sessions |
| | `/sessions/{id}` | DELETE | Delete session |
| **Auth** | `/auth/token` | POST | Generate JWT tokens |
| | `/auth/refresh` | POST | Refresh access token |
| | `/auth/validate` | GET | Validate token |
| | `/auth/logout` | POST | Invalidate token |
| **System** | `/health` | GET | Health check |
| | `/metrics` | GET | Prometheus metrics |
| | `/claude/status` | GET | Claude config status |

### Request Format

All POST endpoints accept JSON:

```json
{
  "prompt": "Your task here",
  "workspace": "/path/to/project",
  "claude": {
    "model": "sonnet",
    "system_prompt": "Custom system prompt",
    "dangerously_skip_permissions": true
  },
  "podman": {
    "timeout": "10m",
    "memory": "2g",
    "cpus": "2"
  }
}
```

---

## Execution Modes

### 1. Synchronous (`/run`)

Wait for completion and get the full response:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a hello world in Python",
    "claude": {"dangerously_skip_permissions": true}
  }'
```

**Response:**
```json
{
  "id": "run-abc123",
  "status": "completed",
  "output": "Here is a hello world program...",
  "session_id": "sess-def456"
}
```

**Best for:** Quick tasks (<30 seconds)

### 2. Asynchronous (`/run/async`)

Submit a job and poll for results:

```bash
# Start job
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor this large codebase",
    "workspace": "/home/user/project",
    "webhook_url": "https://myapp.com/webhook"
  }'

# Response: {"job_id": "job-xyz789"}

# Poll status
curl http://localhost:8080/jobs/job-xyz789
```

**Best for:** Long-running tasks, background processing

### 3. Streaming (`/run/stream`)

Watch output in real-time via Server-Sent Events:

```bash
curl -N "http://localhost:8080/run/stream?prompt=Write%20a%20story&claude.dangerously_skip_permissions=true"
```

**Output:**
```
data: Once upon a time...
data: There was a brave knight...
event: done
data: {"id":"run-abc123","session_id":"sess-def456","status":"completed"}
```

**Best for:** Interactive UIs, real-time feedback

---

## Authentication

### Legacy Tokens (Simple)

Configure static API tokens:

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="token1,token2"
```

Use in requests:
```bash
curl -H "Authorization: Bearer token1" http://localhost:8080/run ...
```

### JWT Authentication (Recommended)

Enable JWT with automatic expiration and refresh:

```bash
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_JWT_EXPIRY="24h"
```

**Token Flow:**

```bash
# 1. Get JWT tokens using API token
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer your-api-token" \
  -d '{"client_id": "my-app"}'

# 2. Use access token for requests
curl -H "Authorization: Bearer eyJ..." http://localhost:8080/run ...

# 3. Refresh before expiry
curl -X POST http://localhost:8080/auth/refresh \
  -d '{"refresh_token": "eyJ..."}'

# 4. Logout when done
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer eyJ..."
```

---

## Session Management

Sessions preserve conversation context across multiple requests.

### Creating a Session

First request creates a new session automatically:

```bash
# First turn
curl -X POST http://localhost:8080/run \
  -d '{"prompt": "Create a Go REST API", "workspace": "/home/user/project"}'

# Response includes session_id
# {"output": "...", "session_id": "sess-550e8400..."}
```

### Continuing a Session

Use `session_id` and `resume: true`:

```bash
curl -X POST http://localhost:8080/run \
  -d '{
    "prompt": "Now add authentication",
    "workspace": "/home/user/project",
    "claude": {
      "session_id": "sess-550e8400...",
      "resume": true
    }
  }'
```

### Session Options

| Option | Description |
|--------|-------------|
| `session_id` | Resume specific session |
| `resume: true` | Continue from session state |
| `continue: true` | Continue most recent session |
| `fork_session: true` | Branch from session |

### Cleanup

```bash
# List sessions
curl http://localhost:8080/sessions

# Delete session
curl -X DELETE http://localhost:8080/sessions/sess-550e8400...
```

---

## Configuration

### Configuration Sources (Priority Order)

1. **Environment Variables** (highest)
2. **Config File** (`stromboli.yaml`)
3. **Defaults** (lowest)

### Essential Settings

| Setting | Env Variable | Default | Description |
|---------|-------------|---------|-------------|
| Server Address | `STROMBOLI_SERVER_ADDRESS` | `:8080` | Listen address |
| Agent Image | `STROMBOLI_AGENT_IMAGE` | `stromboli-agent` | Container image |
| Memory Limit | `STROMBOLI_DEFAULT_MEMORY` | `512m` | Default memory |
| CPU Limit | `STROMBOLI_DEFAULT_CPUS` | `1` | Default CPUs |
| Timeout | `STROMBOLI_DEFAULT_TIMEOUT` | `30m` | Default timeout |

### Config File Example

Create `stromboli.yaml`:

```yaml
server:
  address: ":8080"

agent:
  image: "stromboli-agent"
  image_tag: "v1.0.0"

resources:
  memory: "2g"
  cpus: "2"
  timeout: "1h"

auth:
  enabled: true

jwt:
  secret: "${STROMBOLI_JWT_SECRET}"
  access_expiry: "24h"
  refresh_expiry: "168h"

rate_limit:
  enabled: true
  rate: 100
  burst: 200

tracing:
  enabled: true
  endpoint: "jaeger:4317"
```

See [CONFIGURATION.md](CONFIGURATION.md) for all options.

---

## Observability

### Health Check

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "ok",
  "components": {
    "podman": "healthy",
    "secrets": "healthy"
  }
}
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

Available metrics:
- `stromboli_api_requests_total` - Request counts
- `stromboli_api_request_duration_seconds` - Latency histogram
- `stromboli_jobs_total` - Job counts by status
- `stromboli_containers_created_total` - Container counts

### Distributed Tracing (v1.4+)

Enable OpenTelemetry tracing:

```bash
export STROMBOLI_TRACING_ENABLED=true
export STROMBOLI_TRACING_ENDPOINT="jaeger:4317"
```

Traces include:
- HTTP request spans
- Runner execution spans
- Container lifecycle spans

Compatible with: Jaeger, Grafana Tempo, any OTLP collector

---

## Best Practices

### 1. Always Set Resource Limits

```json
{
  "podman": {
    "timeout": "10m",
    "memory": "2g",
    "cpus": "2"
  }
}
```

### 2. Use Sessions for Multi-Turn Tasks

```bash
# Save session_id from first request
SESSION_ID=$(curl ... | jq -r '.session_id')

# Continue with same session
curl ... -d '{"claude": {"session_id": "'$SESSION_ID'", "resume": true}}'
```

### 3. Use Async for Long Tasks

```bash
# Don't wait for slow tasks
curl -X POST http://localhost:8080/run/async \
  -d '{"prompt": "Long task...", "webhook_url": "https://myapp/webhook"}'
```

### 4. Clean Up Sessions

```bash
# Delete when done
curl -X DELETE http://localhost:8080/sessions/$SESSION_ID
```

### 5. Enable Security in Production

```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_JWT_SECRET="$(openssl rand -base64 32)"
export STROMBOLI_RATE_LIMIT_ENABLED=true
```

---

## Troubleshooting

### "Claude not configured"

**Problem:** Token file not found

**Solution:**
```bash
make claude-setup
# or manually create .claude-secrets with your token
```

### "503 Service Unavailable"

**Problem:** Podman not running

**Solution:**
```bash
systemctl --user start podman.socket
podman system info
```

### "429 Too Many Requests"

**Problem:** Rate limit exceeded

**Solution:** Wait or increase limits:
```bash
export STROMBOLI_RATE_LIMIT_RPS=100
export STROMBOLI_RATE_LIMIT_BURST=200
```

### "401 Unauthorized" or "token revoked"

**Problem:** Invalid or blacklisted token

**Solution:**
```bash
# Refresh your token
curl -X POST http://localhost:8080/auth/refresh \
  -d '{"refresh_token": "eyJ..."}'

# Or generate new tokens
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer your-api-token" \
  -d '{"client_id": "my-app"}'
```

### Container Timeout

**Problem:** Task exceeds timeout

**Solution:** Increase timeout:
```json
{
  "podman": {"timeout": "30m"}
}
```

---

## Related Documentation

- [API Reference](API.md) - Complete API documentation
- [Examples](EXAMPLES.md) - Practical usage examples
- [Configuration](CONFIGURATION.md) - All configuration options
- [Authentication](AUTHENTICATION.md) - JWT and token auth
- [Testing](TESTING.md) - Running tests
- [Architecture](ARCHITECTURE.md) - System design

---

*Need help? Open an issue at [github.com/tomblanc/stromboli](https://github.com/tomblanc/stromboli)*
