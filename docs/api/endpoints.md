# API Endpoints

Complete reference for all Stromboli API endpoints.

## POST /run

Run Claude synchronously.

### Request

```json
{
  "prompt": "Your prompt here",
  "workspace": "/path/to/workspace",
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
    "cpu_shares": 512
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
