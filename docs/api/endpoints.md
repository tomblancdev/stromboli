# API Endpoints

Complete reference for all Stromboli endpoints.

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
    "system_prompt": "...",
    "append_system_prompt": "...",
    "allowed_tools": ["Read", "Grep"],
    "disallowed_tools": ["Write"],
    "output_format": "json",
    "max_budget_usd": 5.00,
    "dangerously_skip_permissions": false
  },
  "podman": {
    "image": "python:3.12",
    "volumes": ["/data:/data:ro"],
    "secrets_env": {"GH_TOKEN": "github-token"},
    "timeout": "5m",
    "memory": "512m",
    "cpus": "1",
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

| Error Status | Cause |
|---|---|
| 400 | Invalid request, volume/image validation failed |
| 503 | Claude not configured |
| 500 | Execution failed |

---

## GET /run/stream

Stream Claude output via Server-Sent Events.

### Query parameters

| Parameter | Type | Description |
|---|---|---|
| `prompt` | string | Required |
| `workspace` | string | Workspace path |
| `session_id` | string | Session to resume |
| `resume` | bool | Resume session |

### Response

```
Content-Type: text/event-stream

data: {"type":"output","content":"Hello..."}
data: {"type":"done","session_id":"...","id":"run-..."}
```

---

## POST /run/async

Run Claude asynchronously. Same request body as `/run`.

### Response

```json
{"job_id": "job-abc123", "status": "pending"}
```

---

## GET /jobs

List all jobs.

```json
{
  "jobs": [
    {"id": "job-abc123", "status": "completed", "created_at": "2024-01-01T00:00:00Z"}
  ]
}
```

---

## GET /jobs/:id

Get job details.

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

Job statuses: `pending`, `running`, `completed`, `failed`, `cancelled`

---

## DELETE /jobs/:id

Cancel a job.

```json
{"id": "job-abc123", "status": "cancelled"}
```

---

## GET /sessions

```json
{"sessions": ["550e8400-...", "6ba7b810-..."]}
```

---

## DELETE /sessions/:id

```json
{"success": true, "session_id": "550e8400..."}
```

---

## GET /sessions/:id/messages

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | int | 50 | Max messages |
| `offset` | int | 0 | Skip messages |

```json
{
  "messages": [
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi there!"}
  ],
  "total": 2
}
```

---

## GET /health

```json
{
  "status": "ok",
  "name": "stromboli",
  "version": "0.3.0-alpha",
  "components": [
    {"name": "podman", "status": "ok"},
    {"name": "claude-credentials-file", "status": "ok"},
    {"name": "claude-credentials-secret", "status": "ok"}
  ]
}
```

---

## GET /secrets

```json
{"secrets": ["claude-credentials", "github-token", "gitlab-token"]}
```

---

## GET /claude/status

```json
{"configured": true, "message": "Claude is configured"}
```

---

## GET /metrics

Prometheus-format metrics.

---

## GET /images

List local images sorted by compatibility.

```json
{
  "images": [
    {
      "repository": "python",
      "tag": "3.12",
      "size": 1073741824,
      "compatibility_rank": 3,
      "compatible": true,
      "tools": ["python", "pip"]
    }
  ]
}
```

Compatibility ranks: 1 = official agent image, 2 = verified compatible, 3 = standard glibc, 4 = Alpine/musl (incompatible).

---

## GET /images/:name

Inspect a specific image (e.g., `python:3.12-slim`).

```json
{
  "repository": "python",
  "tag": "3.12-slim",
  "compatibility_rank": 3,
  "compatible": true,
  "tools": ["python3", "pip"]
}
```

---

## GET /images/search

| Parameter | Type | Default | Description |
|---|---|---|---|
| `q` | string | Required | Search query |
| `limit` | int | 25 | Max results (max 100) |

```json
{
  "results": [
    {"name": "python", "description": "Python runtime", "stars": 9500, "official": true}
  ]
}
```

---

## POST /images/pull

```json
{"image": "python:3.12", "quiet": false, "platform": "linux/amd64"}
```

```json
{"success": true, "image_id": "sha256:abc123...", "image": "python:3.12"}
```
