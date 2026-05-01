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
    "max_turns": 30,
    "effort": "high",
    "dangerously_skip_permissions": false
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
  "structured_output": {"key": "value"},
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "usage": {
    "token_usage": {
      "input_tokens": 1234,
      "output_tokens": 567,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 8901
    },
    "estimated_cost_usd": 0.0142
  }
}
```

`structured_output` is populated when using `output_format: "json"` with a `json_schema`. It extracts the parsed JSON from Claude's response envelope.

`usage` reports the cumulative token totals for the session (read best-effort from the session JSONL) plus an `estimated_cost_usd` derived from the per-model price table. It is omitted when no assistant message has reported usage yet.

#### Notable request fields

| Field | Type | Description |
|---|---|---|
| `claude.effort` | string | Thinking/agentic complexity level. Per the upstream CLI: `low`, `medium`, `high`, `xhigh`, `max`. The accepted subset depends on the model — Stromboli passes through; Claude rejects unsupported levels at runtime. |
| `claude.prompt_caching_ttl` | string | `"5m"` enables `FORCE_PROMPT_CACHING_5M=1`, `"1h"` enables `ENABLE_PROMPT_CACHING_1H=1`. Useful for long-lived agent loops re-using the same context. |
| `claude.bedrock_service_tier` | string | `default` / `flex` / `priority` — sets `ANTHROPIC_BEDROCK_SERVICE_TIER`. Ignored when not on Bedrock. |
| `claude.enable_powershell_tool` | bool | Sets `CLAUDE_CODE_USE_POWERSHELL_TOOL=1`. No-op on non-Windows containers. |

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
| `workdir` | string | Working directory inside container |
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
{"job_id": "job-abc123", "session_id": "550e8400-e29b-41d4-a716-446655440000"}
```

The `session_id` is returned immediately so you can use it for tracking or session resume even before the job completes.

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
  "structured_output": {"key": "value"},
  "session_id": "...",
  "crash_info": null,
  "created_at": "...",
  "updated_at": "..."
}
```

Job statuses: `pending`, `running`, `completed`, `failed`, `crashed`, `cancelled`

When a job crashes (e.g. OOM, timeout), `status` is `crashed` and `crash_info` contains details:

```json
{
  "crash_info": {
    "exit_code": 137,
    "signal": "killed",
    "partial_output": "last output before crash...",
    "task_completed": false
  }
}
```

---

## DELETE /jobs/:id

Cancel a job.

```json
{"id": "job-abc123", "status": "cancelled"}
```

---

## GET /sessions

Returns each session's ID and (optional) human title. Titles are populated when an agent's `UserPromptSubmit` hook returns `hookSpecificOutput.sessionTitle` — Claude Code's interactive `/rename` does this automatically; headless callers can replicate it via a settings file.

```json
{
  "sessions": [
    {"id": "550e8400-...", "title": "Refactor billing service"},
    {"id": "6ba7b810-...", "title": ""}
  ]
}
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

## GET /sessions/:id/messages/:message_id

Fetch a single message from a session by its ID. Useful for retrieving large assistant outputs without re-reading the whole transcript.

```json
{"role": "assistant", "content": "...", "id": "msg-abc"}
```

| Error Status | Cause |
|---|---|
| 404 | Session or message not found |

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

---

## Persistent Agents

Long-lived Claude processes for event-driven workloads (sensor buses, on-call bots, anything that needs sub-second turn latency instead of the 2–3 s cold start of `/run`). One agent = one container = one Claude process running with stream-json I/O. Agents stay alive across many `/send` turns until DELETE or the idle timeout fires.

### POST /agents

Spawn a new agent.

```json
{
  "prompt": "You are an on-call assistant. Wait for incident reports.",
  "workdir": "/workspace",
  "idle_timeout_seconds": 1800,
  "claude": {
    "model": "sonnet",
    "allowed_tools": ["Read", "Bash"]
  }
}
```

| Field | Type | Description |
|---|---|---|
| `prompt` | string | Optional first turn. If omitted, the agent boots empty and waits for `/send`. |
| `workdir` | string | Working directory inside the container. |
| `idle_timeout_seconds` | int | Override the manager-wide default. `0` = use default; negatives are rejected. |
| `claude` | object | Free-form Claude CLI options (same shape as `RunRequest.claude`). |

```json
{
  "id": "agent-abc123",
  "session_id": "550e8400-...",
  "status": "starting",
  "created_at": "2026-05-01T08:00:00Z",
  "last_activity_at": "2026-05-01T08:00:00Z",
  "idle_timeout_seconds": 1800,
  "turns_completed": 0
}
```

| Status | Meaning |
|---|---|
| `starting` | Container booting; Claude hasn't reported readiness yet. |
| `idle` | Alive and ready to accept the next turn. |
| `generating` | A turn is in flight. Further `/send` returns `409`. |
| `exited` | The process or container has stopped (clean or crashed). Record persists briefly so callers can read `exit_error`. |

| Error Status | Cause |
|---|---|
| 400 | Invalid request body |
| 500 | Spawn failed |

### GET /agents

List every registered agent.

```json
[{"id": "agent-abc123", "status": "idle", "...": "..."}]
```

### GET /agents/:id

Inspect one agent. Returns the same `Snapshot` shape as `POST /agents`.

| Error Status | Cause |
|---|---|
| 404 | Agent not found |

### POST /agents/:id/send

Append a prompt to a running agent. Returns immediately; output streams via `/stream`.

```json
{"prompt": "What's the status of incident INC-417?"}
```

```json
{"turn_id": "turn-abc"}
```

| Error Status | Cause |
|---|---|
| 400 | Invalid request body |
| 404 | Agent not found |
| 409 | Turn already in progress (`ErrAgentBusy`) |
| 410 | Agent has exited (`ErrAgentExited`) |
| 503 | Agent not ready yet (`ErrAgentNotReady` — still in `starting`) |

### GET /agents/:id/stream

Server-Sent Events stream of agent output. Each event mirrors `agent.Event`:

```
event: turn_start
data: {"type":"turn_start","turn_id":"turn-abc","at":"..."}

event: output
data: {"type":"output","turn_id":"turn-abc","content":"Hello","at":"..."}

event: turn_end
data: {"type":"turn_end","turn_id":"turn-abc","at":"..."}

event: closed
data: {}
```

Event types: `turn_start`, `output`, `turn_end`, `error`, `exited`. The `closed` event fires when the subscription ends because the agent was stopped/deleted.

### DELETE /agents/:id

Stop the agent and free resources. Returns `204 No Content` on success, `404` if not found.
