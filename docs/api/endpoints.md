# API Endpoints

Complete reference for all Stromboli API endpoints.

## POST /run

Run Claude synchronously.

### Request

```json
{
  "prompt": "Your prompt here",
  "workdir": "/workspace",
  "webhook_url": "https://...",
  "claude": {
    "session_id": "uuid",
    "resume": false,
    "continue": false,
    "fork_session": false,
    "no_persistence": false,
    "model": "sonnet",
    "fallback_model": "haiku",
    "system_prompt": "...",
    "append_system_prompt": "...",
    "tools": ["Bash", "Read"],
    "allowed_tools": ["Bash(git:*)"],
    "disallowed_tools": ["Write"],
    "permission_mode": "default",
    "dangerously_skip_permissions": false,
    "input_format": "text",
    "output_format": "json",
    "max_budget_usd": 5.00,
    "verbose": false
  },
  "podman": {
    "image": "python:3.12",
    "volumes": ["/data:/data:ro"],
    "secrets_env": {"GH_TOKEN": "github-token"},
    "timeout": "5m",
    "memory": "512m",
    "cpus": "1",
    "cpu_shares": 512,
    "lifecycle": {
      "on_create_command": ["pip install -r requirements.txt"],
      "post_create": ["npm run build"],
      "post_start": ["redis-server --daemonize yes"],
      "hooks_timeout": "5m"
    },
    "environment": {
      "type": "compose",
      "path": "/path/to/docker-compose.yml",
      "service": "dev",
      "build_timeout": "15m"
    }
  }
}
```

### Response

```json
{
  "id": "run-abc123def456",
  "status": "completed",
  "output": "Claude's response...",
  "session_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Errors

| Status | Error |
|--------|-------|
| 400 | Invalid request body |
| 400 | Workspace validation failed |
| 400 | Image validation failed |
| 503 | Claude not configured |
| 500 | Execution failed |

---

## GET /run/stream

Run Claude with Server-Sent Events streaming.

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `prompt` | string | Required. The prompt |
| `workspace` | string | Workspace path |
| `session_id` | string | Session to resume |
| `resume` | bool | Resume session |

### Response

```
Content-Type: text/event-stream

data: {"type":"output","content":"Hello..."}
data: {"type":"output","content":"I found..."}
data: {"type":"done","session_id":"...","id":"run-..."}
```

---

## POST /run/async

Run Claude asynchronously.

### Request

Same as `/run`.

### Response

```json
{
  "job_id": "job-abc123",
  "status": "pending"
}
```

---

## GET /jobs

List all jobs.

### Response

```json
{
  "jobs": [
    {
      "id": "job-abc123",
      "status": "completed",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

## GET /jobs/:id

Get job details.

### Response

```json
{
  "id": "job-abc123",
  "status": "completed",
  "output": "...",
  "session_id": "...",
  "created_at": "...",
  "completed_at": "..."
}
```

### Job Status Values

| Status | Description |
|--------|-------------|
| `pending` | Queued, not started |
| `running` | Currently executing |
| `completed` | Finished successfully |
| `failed` | Finished with error |
| `cancelled` | Cancelled by user |

---

## DELETE /jobs/:id

Cancel a running or pending job.

### Response

```json
{
  "id": "job-abc123",
  "status": "cancelled"
}
```

---

## GET /sessions

List all session IDs.

### Response

```json
{
  "sessions": [
    "550e8400-e29b-41d4-a716-446655440000",
    "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
  ]
}
```

---

## DELETE /sessions/:id

Delete a session and its data.

### Response

```json
{
  "success": true,
  "session_id": "550e8400..."
}
```

---

## GET /sessions/:id/messages

Get conversation messages from a session.

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Max messages |
| `offset` | int | 0 | Skip messages |

### Response

```json
{
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    },
    {
      "role": "assistant",
      "content": "Hi there!"
    }
  ],
  "total": 2
}
```

---

## GET /health

Health check with component status.

### Response

```json
{
  "status": "ok",
  "name": "stromboli",
  "version": "0.1.5-alpha",
  "components": [
    {"name": "podman", "status": "ok"},
    {"name": "claude-credentials-file", "status": "ok"},
    {"name": "claude-credentials-secret", "status": "ok"}
  ]
}
```

---

## GET /secrets

List available Podman secrets.

### Response

```json
{
  "secrets": [
    "claude-credentials",
    "github-token",
    "gitlab-token"
  ]
}
```

---

## GET /claude/status

Check if Claude credentials are configured.

### Response

```json
{
  "configured": true,
  "message": "Claude is configured"
}
```

---

## GET /metrics

Prometheus metrics endpoint.

### Response

```
# HELP stromboli_active_containers Current number of running containers
# TYPE stromboli_active_containers gauge
stromboli_active_containers 2
...
```

---

## Image Discovery API

The Image Discovery API allows you to list, inspect, search, and pull container images.

---

## GET /images

List all local container images sorted by compatibility.

### Response

```json
{
  "images": [
    {
      "id": "sha256:abc123...",
      "repository": "python",
      "tag": "3.12",
      "size": 1073741824,
      "created": "2024-01-15T10:30:00Z",
      "compatibility_rank": 3,
      "compatible": true,
      "tools": ["python", "pip"],
      "has_claude_cli": false,
      "description": "Python 3.12 runtime"
    }
  ]
}
```

### Compatibility Ranks

| Rank | Description | Compatible |
|------|-------------|------------|
| 1 | Official Stromboli agent image | Yes |
| 2 | Verified compatible (has Claude CLI) | Yes |
| 3 | Standard glibc-based image | Yes |
| 4 | Alpine/musl-based image | No |

---

## GET /images/:name

Get detailed information about a specific image.

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | path | Image name with optional tag (e.g., `python:3.12-slim`) |

### Response

```json
{
  "id": "sha256:abc123...",
  "repository": "python",
  "tag": "3.12-slim",
  "size": 536870912,
  "created": "2024-01-15T10:30:00Z",
  "labels": {
    "org.opencontainers.image.source": "https://github.com/...",
    "maintainer": "..."
  },
  "compatibility_rank": 3,
  "rank_description": "Standard glibc-based image (compatible)",
  "compatible": true,
  "tools": ["python3", "pip"],
  "has_claude_cli": false,
  "description": "Python 3.12 slim runtime"
}
```

### Errors

| Status | Error |
|--------|-------|
| 400 | Invalid image name |
| 404 | Image not found |
| 500 | Internal server error |

---

## GET /images/search

Search container registries for images.

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | Required | Search query (e.g., `python`, `node`) |
| `limit` | int | 25 | Maximum results (max 100) |
| `no_trunc` | bool | false | Show full descriptions |

### Response

```json
{
  "results": [
    {
      "index": "docker.io",
      "name": "python",
      "description": "Python is an interpreted, interactive, object-oriented...",
      "stars": 9500,
      "official": true,
      "automated": false
    },
    {
      "index": "docker.io",
      "name": "pypy",
      "description": "PyPy is a fast, compliant Python implementation...",
      "stars": 850,
      "official": true,
      "automated": false
    }
  ]
}
```

### Errors

| Status | Error |
|--------|-------|
| 400 | Missing query parameter 'q' |
| 400 | Invalid limit parameter |
| 502 | Registry search failed |

---

## POST /images/pull

Pull an image from a container registry.

### Request

```json
{
  "image": "python:3.12",
  "quiet": false,
  "platform": "linux/amd64"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `image` | string | Yes | Image name with optional tag |
| `quiet` | bool | No | Suppress pull output |
| `platform` | string | No | Target platform (e.g., `linux/amd64`, `linux/arm64`) |

### Response

```json
{
  "success": true,
  "image_id": "sha256:abc123def456...",
  "image": "python:3.12"
}
```

### Errors

| Status | Error |
|--------|-------|
| 400 | Invalid request |
| 400 | Invalid image name |
| 500 | Failed to pull image |
