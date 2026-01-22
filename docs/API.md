# Stromboli API Documentation

> REST API for executing Claude Code in isolated Podman containers

## Base URL

```
Development: http://localhost:8080
Production:  https://stromboli.example.com
```

## Authentication

Authentication is **optional** and disabled by default. When enabled via environment variables, all protected endpoints require Bearer token authentication.

```bash
# With authentication enabled
curl -H "Authorization: Bearer <token>" http://localhost:8080/run
```

### Configuration

| Environment Variable | Description |
|---------------------|-------------|
| `STROMBOLI_AUTH_ENABLED` | Set to `true` to enable authentication (default: `false`) |
| `STROMBOLI_API_TOKENS` | Comma-separated list of valid Bearer tokens |

**Example:**
```bash
export STROMBOLI_AUTH_ENABLED=true
export STROMBOLI_API_TOKENS="token1,token2,token3"
```

---

## API Endpoints

### System

#### `GET /health`

Health check (no authentication required).

**Response:** `200 OK`
```json
{
  "status": "ok",
  "name": "stromboli"
}
```

#### `GET /metrics`

Prometheus metrics endpoint (no authentication required).

**Response:** `200 OK`
```
# HELP stromboli_api_requests_total Total API requests
# TYPE stromboli_api_requests_total counter
stromboli_api_requests_total{endpoint="/run",method="POST",status="200"} 42
...
```

#### `GET /claude/status`

Check if Claude API credentials are configured.

**Response:** `200 OK`
```json
{
  "configured": true,
  "message": "Claude is configured"
}
```

**When not configured:**
```json
{
  "configured": false,
  "message": "Run 'make claude-setup' to configure"
}
```

---

### Execution

#### `POST /run`

Execute Claude Code synchronously and return the output immediately.

**Request Body:**
```json
{
  "prompt": "Write a function to calculate factorial",
  "workspace": "/home/user/myproject",
  "webhook_url": "https://example.com/webhook",
  "claude": {
    "model": "sonnet",
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "resume": false,
    "continue": false,
    "dangerously_skip_permissions": true,
    "system_prompt": "You are a senior Go developer",
    "max_budget_usd": 5.0,
    "timeout": "10m"
  },
  "podman": {
    "timeout": "5m",
    "memory": "1g",
    "cpus": "2",
    "cpu_shares": 512,
    "volumes": ["/data:/data:ro"]
  }
}
```

**Response:** `200 OK`
```json
{
  "id": "run-abc123def456",
  "status": "completed",
  "output": "Here's a function to calculate factorial:\n\nfunc Factorial(n int) int {\n  if n <= 1 {\n    return 1\n  }\n  return n * Factorial(n-1)\n}\n",
  "session_id": "sess-abc123def456"
}
```

**Error Response:** `400 Bad Request`, `500 Internal Server Error`, `503 Service Unavailable`
```json
{
  "status": "error",
  "error": "Invalid request: prompt is required"
}
```

#### `POST /run/async`

Execute Claude Code asynchronously and return a job ID immediately.

**Request Body:** Same as `/run`

**Response:** `202 Accepted`
```json
{
  "job_id": "job-abc123def456"
}
```

Use the job ID to check status via `/jobs/{id}` endpoint.

#### `GET /run/stream`

Execute Claude Code and stream output in real-time using Server-Sent Events (SSE).

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `prompt` | string | ✅ | The prompt to send to Claude |
| `workspace` | string | - | Workspace path to mount |
| `session_id` | string | - | Session ID for conversation continuation |

**Example:**
```bash
curl -N "http://localhost:8080/run/stream?prompt=Write%20a%20hello%20world%20program&workspace=/home/user/project"
```

**Response:** `200 OK` with `Content-Type: text/event-stream`
```
data: Analyzing your request...

data: Creating file main.go...

data: package main

data:

data: import "fmt"

event: done
data: {"id":"run-abc123","session_id":"sess-def456","status":"completed"}
```

**Event Types:**
- `data:` - Output line from Claude
- `event: done` - Execution completed successfully (includes metadata)
- `event: error` - Execution failed

**Error Events:**
```
event: error
data: execution failed: timeout exceeded
```

---

### Job Management

#### `GET /jobs`

List all async jobs.

**Response:** `200 OK`
```json
{
  "jobs": [
    {
      "id": "job-abc123def456",
      "status": "running",
      "output": "",
      "error": "",
      "session_id": "",
      "created_at": "2025-01-22T10:00:00Z",
      "updated_at": "2025-01-22T10:00:05Z"
    },
    {
      "id": "job-xyz789ghi012",
      "status": "completed",
      "output": "Task completed successfully",
      "session_id": "sess-abc123",
      "created_at": "2025-01-22T09:00:00Z",
      "updated_at": "2025-01-22T09:05:00Z"
    }
  ]
}
```

#### `GET /jobs/{id}`

Get the status and result of a specific async job.

**Path Parameters:**
- `id` - Job ID returned from `/run/async`

**Response:** `200 OK`
```json
{
  "id": "job-abc123def456",
  "status": "completed",
  "output": "Here is your code...",
  "error": "",
  "session_id": "sess-abc123",
  "created_at": "2025-01-22T10:00:00Z",
  "updated_at": "2025-01-22T10:05:00Z"
}
```

**Status Values:**
- `pending` - Job created but not started
- `running` - Execution in progress
- `completed` - Execution finished successfully
- `failed` - Execution failed (see `error` field)
- `cancelled` - Job was cancelled before completion

**Error Response:** `404 Not Found`
```json
{
  "status": "error",
  "error": "job not found"
}
```

#### `DELETE /jobs/{id}`

Cancel a pending or running async job.

**Path Parameters:**
- `id` - Job ID to cancel

**Response:** `200 OK`
```json
{
  "cancelled": true,
  "job_id": "job-abc123def456"
}
```

**Error Responses:**

`404 Not Found` - Job doesn't exist:
```json
{
  "status": "error",
  "error": "job not found"
}
```

`409 Conflict` - Job cannot be cancelled (already completed/failed):
```json
{
  "status": "error",
  "error": "cannot cancel job (already completed, failed, or cancelled)"
}
```

---

### Session Management

#### `GET /sessions`

List all existing Claude sessions stored on disk.

**Response:** `200 OK`
```json
{
  "sessions": [
    "sess-abc123def456",
    "sess-xyz789ghi012"
  ]
}
```

**Error Response:** `500 Internal Server Error`
```json
{
  "error": "failed to list sessions: ..."
}
```

#### `DELETE /sessions/{id}`

Destroy a session and remove all its stored data.

**Path Parameters:**
- `id` - Session ID to destroy

**Response:** `200 OK`
```json
{
  "success": true,
  "session_id": "sess-abc123def456"
}
```

**Error Responses:**

`400 Bad Request` - Invalid session ID:
```json
{
  "success": false,
  "error": "invalid session ID"
}
```

`404 Not Found` - Session doesn't exist:
```json
{
  "success": false,
  "error": "session not found: sess-abc123"
}
```

---

## Request/Response Schemas

### RunRequest

Main request object for Claude execution.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `prompt` | string | ✅ | Task/prompt for Claude |
| `workspace` | string | - | Host path to mount into container as `/workspace` |
| `webhook_url` | string | - | URL to POST job result when complete (async only) |
| `claude` | [ClaudeOptions](#claudeoptions) | - | Claude CLI configuration |
| `podman` | [PodmanOptions](#podmanoptions) | - | Container resource limits |

### ClaudeOptions

Complete configuration for Claude CLI (all 40+ options).

#### Session Management

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | Session UUID for persistence |
| `resume` | boolean | Resume existing session (requires `session_id`) |
| `continue` | boolean | Continue most recent conversation (ignores `session_id`) |
| `fork_session` | boolean | Create new session ID when resuming |
| `no_persistence` | boolean | Don't save session to disk |

#### Model Configuration

| Field | Type | Description |
|-------|------|-------------|
| `model` | string | Model alias: `sonnet`, `opus`, `haiku` or full name |
| `fallback_model` | string | Fallback when primary model overloaded |

#### System Prompt

| Field | Type | Description |
|-------|------|-------------|
| `system_prompt` | string | Replace default system prompt |
| `append_system_prompt` | string | Append to default system prompt |

#### Tools Configuration

| Field | Type | Description |
|-------|------|-------------|
| `tools` | []string | Built-in tools to enable (e.g., `["Bash", "Read", "Edit"]`) |
| `allowed_tools` | []string | Allowed tools with patterns (e.g., `["Bash(git:*)"]`) |
| `disallowed_tools` | []string | Tools to deny |

#### Permissions

| Field | Type | Description |
|-------|------|-------------|
| `permission_mode` | string | Mode: `acceptEdits`, `bypassPermissions`, `default`, `delegate`, `dontAsk`, `plan` |
| `dangerously_skip_permissions` | boolean | Bypass all permission checks (sandboxed environments only) |
| `allow_dangerously_skip_permissions` | boolean | Enable bypass as option without enabling by default |

#### Input/Output Format

| Field | Type | Description |
|-------|------|-------------|
| `input_format` | string | Input format: `text`, `stream-json` |
| `output_format` | string | Output format: `text`, `json`, `stream-json` |
| `include_partial_messages` | boolean | Include partial message chunks (stream-json only) |
| `replay_user_messages` | boolean | Re-emit user messages on stdout |

#### Structured Output

| Field | Type | Description |
|-------|------|-------------|
| `json_schema` | string | JSON Schema for structured output validation |

#### Budget Control

| Field | Type | Description |
|-------|------|-------------|
| `max_budget_usd` | float | Maximum dollar amount for API calls |

#### MCP Configuration

| Field | Type | Description |
|-------|------|-------------|
| `mcp_configs` | []string | MCP server config files or JSON strings |
| `strict_mcp_config` | boolean | Only use MCP servers from `mcp_configs` |

#### Agents

| Field | Type | Description |
|-------|------|-------------|
| `agent` | string | Agent for current session |
| `agents` | map[string]any | Custom agents definition (JSON object) |

#### Additional Resources

| Field | Type | Description |
|-------|------|-------------|
| `add_dirs` | []string | Additional directories for tool access |
| `plugin_dirs` | []string | Plugin directories |
| `files` | []string | File resources (format: `file_id:path`) |

#### Settings

| Field | Type | Description |
|-------|------|-------------|
| `settings` | string | Path to settings JSON file or JSON string |
| `setting_sources` | []string | Sources to load: `user`, `project`, `local` |

#### Beta Features

| Field | Type | Description |
|-------|------|-------------|
| `betas` | []string | Beta headers for API requests |

#### Miscellaneous

| Field | Type | Description |
|-------|------|-------------|
| `verbose` | boolean | Enable verbose mode |
| `debug` | string | Debug mode with optional category filter (e.g., `api,hooks`) |
| `disable_slash_commands` | boolean | Disable all slash commands/skills |

### PodmanOptions

Container resource configuration.

| Field | Type | Description |
|-------|------|-------------|
| `volumes` | []string | Volume mounts in `host:container` or `host:container:options` format |
| `timeout` | string | Container timeout (e.g., `5m`, `1h`, `30s`) |
| `memory` | string | Memory limit (e.g., `512m`, `1g`, `2g`) |
| `cpus` | string | CPU limit (e.g., `0.5`, `1`, `2`) |
| `cpu_shares` | integer | CPU shares (relative weight, default 1024) |

**Example:**
```json
{
  "volumes": [
    "/data:/data:ro",
    "/cache:/cache:rw"
  ],
  "timeout": "10m",
  "memory": "2g",
  "cpus": "2",
  "cpu_shares": 2048
}
```

---

## Webhooks

When you provide a `webhook_url` in an async execution request, Stromboli will POST the job result to that URL when execution completes.

### Webhook Payload

```json
{
  "job_id": "job-abc123def456",
  "status": "completed",
  "output": "Here is your code...",
  "error": "",
  "session_id": "sess-abc123"
}
```

**Fields:**
- `job_id` - The job ID
- `status` - Job status: `completed`, `failed`, or `cancelled`
- `output` - Claude's output (when successful)
- `error` - Error message (when failed)
- `session_id` - Session ID for conversation continuation

### Webhook Behavior

- Webhooks are sent in the background (non-blocking)
- Failed webhook deliveries are logged but don't affect job status
- No retry mechanism (single delivery attempt)
- Timeout: 30 seconds

**Example webhook handler:**
```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var payload struct {
        JobID     string `json:"job_id"`
        Status    string `json:"status"`
        Output    string `json:"output"`
        Error     string `json:"error"`
        SessionID string `json:"session_id"`
    }

    json.NewDecoder(r.Body).Decode(&payload)

    if payload.Status == "completed" {
        // Handle successful completion
    } else {
        // Handle failure
    }

    w.WriteHeader(http.StatusOK)
}
```

---

## Server-Sent Events (SSE)

The `/run/stream` endpoint uses Server-Sent Events for real-time output streaming.

### Event Format

**Data Events:**
```
data: <output line>
```

**Completion Event:**
```
event: done
data: {"id":"run-abc123","session_id":"sess-def456","status":"completed"}
```

**Error Event:**
```
event: error
data: <error message>
```

### Client Example (JavaScript)

```javascript
const eventSource = new EventSource(
  'http://localhost:8080/run/stream?prompt=Hello%20Claude'
);

eventSource.onmessage = (event) => {
  console.log('Output:', event.data);
};

eventSource.addEventListener('done', (event) => {
  const result = JSON.parse(event.data);
  console.log('Completed:', result);
  eventSource.close();
});

eventSource.addEventListener('error', (event) => {
  console.error('Error:', event.data);
  eventSource.close();
});
```

### Client Example (curl)

```bash
curl -N "http://localhost:8080/run/stream?prompt=Write%20hello%20world"
```

---

## Error Handling

All endpoints return consistent error responses.

### Error Response Format

```json
{
  "status": "error",
  "error": "Human-readable error message"
}
```

### HTTP Status Codes

| Code | Description |
|------|-------------|
| `200` | Success |
| `202` | Accepted (async job created) |
| `400` | Bad Request (invalid input) |
| `401` | Unauthorized (missing/invalid auth token) |
| `404` | Not Found (resource doesn't exist) |
| `409` | Conflict (invalid state transition) |
| `500` | Internal Server Error |
| `503` | Service Unavailable (Claude not configured) |

---

## Rate Limits

Default rate limits (configurable):

- **100 requests/minute** per API token
- **10 concurrent agents** per API token

Rate limit headers (when implemented):
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1737536400
```

---

## OpenAPI/Swagger Specification

Stromboli includes Swagger annotations for automatic OpenAPI spec generation.

### Generate Swagger Docs

```bash
make docs-swagger
```

This generates:
- `docs/swagger/swagger.json` - OpenAPI 3.0 specification
- `docs/swagger/swagger.yaml` - YAML format
- `docs/swagger/docs.go` - Go embedded docs

### View Swagger UI

```bash
make docs-serve
```

Opens Swagger UI at http://localhost:8081

---

## Best Practices

### Security

1. **Enable authentication in production**:
   ```bash
   export STROMBOLI_AUTH_ENABLED=true
   export STROMBOLI_API_TOKENS="secure-random-token"
   ```

2. **Use workspace allowlisting**: Configure allowed workspace paths in `main.go`

3. **Set resource limits**: Always specify `podman.timeout`, `podman.memory`, and `podman.cpus`

4. **Use secrets for tokens**: Never pass Claude tokens in API requests; configure via `.claude-secrets`

### Performance

1. **Use async execution for long tasks**: Use `/run/async` for tasks that take >30 seconds

2. **Stream for real-time feedback**: Use `/run/stream` when you need immediate output visibility

3. **Reuse sessions**: Pass `session_id` and `resume: true` to continue conversations efficiently

4. **Set appropriate timeouts**: Match `podman.timeout` to expected task duration

### Reliability

1. **Implement webhook handlers**: For async execution, always provide a `webhook_url`

2. **Poll job status**: If webhook fails, poll `/jobs/{id}` as fallback

3. **Clean up sessions**: Regularly delete unused sessions via `/sessions/{id}`

4. **Monitor metrics**: Use `/metrics` endpoint for observability

---

## Examples

See [EXAMPLES.md](EXAMPLES.md) for comprehensive usage examples including:
- Basic synchronous execution
- Async execution with polling
- Streaming output
- Session management
- Webhook integration
- Resource limits
- Error handling
