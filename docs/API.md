# Stromboli API Documentation 🎭

> REST API for managing Claude Code agents in Podman containers

## Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://stromboli.example.com/api/v1
```

## Authentication

All endpoints (except health checks) require OAuth2 Bearer token authentication via Authentik.

```bash
curl -H "Authorization: Bearer <token>" https://stromboli.example.com/api/v1/agents
```

### Scopes

| Scope | Description |
|-------|-------------|
| `agents:read` | List and view agents |
| `agents:write` | Create, modify, stop agents |
| `workspaces:read` | List workspaces |
| `workspaces:write` | Manage workspaces |

---

## Health Endpoints

### `GET /health`

Health check (no auth required).

**Response:**
```json
{
  "status": "ok",
  "version": "0.1.0",
  "uptime_seconds": 3600
}
```

### `GET /health/ready`

Readiness check - verifies Podman socket and database connectivity.

**Response:**
```json
{
  "ready": true,
  "checks": {
    "podman": true,
    "database": true,
    "auth": true
  }
}
```

---

## Agent Endpoints

### `GET /agents`

List all agents owned by the authenticated user.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | - | Filter by status (pending, running, completed, etc.) |
| `limit` | int | 50 | Max results (1-100) |
| `offset` | int | 0 | Pagination offset |

**Response:**
```json
{
  "agents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "fix-auth-tests",
      "status": "running",
      "task": "Fix the failing tests in src/auth.go",
      "created_at": "2025-01-21T10:00:00Z",
      "started_at": "2025-01-21T10:00:05Z"
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

---

### `POST /agents`

Spawn a new Claude Code agent.

**Request Body:**
```json
{
  "task": "Fix the failing tests in src/auth.go",
  "name": "fix-auth-tests",
  "workspace": {
    "git_url": "https://github.com/user/repo.git",
    "branch": "main",
    "writable": true
  },
  "resources": {
    "memory_mb": 2048,
    "cpu_cores": 1
  },
  "timeout_minutes": 120
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `task` | string | ✅ | Task for Claude Code (1-10000 chars) |
| `name` | string | - | Human-readable name (lowercase, hyphens) |
| `workspace` | object | - | Git repository config |
| `workspace.git_url` | string | - | Repository URL to clone |
| `workspace.branch` | string | - | Branch to checkout (default: "main") |
| `workspace.commit` | string | - | Specific commit SHA |
| `workspace.writable` | bool | - | Allow modifications (default: true) |
| `workspace.writable_paths` | []string | - | Specific writable paths |
| `resources` | object | - | Resource limits |
| `resources.memory_mb` | int | - | Memory limit (512-16384, default: 2048) |
| `resources.cpu_cores` | float | - | CPU cores (0.5-8, default: 1) |
| `resources.storage_gb` | int | - | Storage limit (1-100, default: 10) |
| `credentials` | object | - | Claude API credentials |
| `credentials.api_key` | string | - | Anthropic API key |
| `timeout_minutes` | int | - | Auto-stop timeout (1-1440, default: 120) |
| `environment` | map | - | Additional env vars |

**Response:** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "fix-auth-tests",
  "status": "pending",
  "task": "Fix the failing tests in src/auth.go",
  "workspace": { ... },
  "resources": { ... },
  "created_at": "2025-01-21T10:00:00Z"
}
```

**Errors:**
- `400 Bad Request` - Invalid configuration
- `429 Too Many Requests` - Agent limit reached

---

### `GET /agents/{id}`

Get detailed information about a specific agent.

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "fix-auth-tests",
  "status": "running",
  "task": "Fix the failing tests in src/auth.go",
  "workspace": {
    "git_url": "https://github.com/user/repo.git",
    "branch": "main"
  },
  "container_id": "abc123def456",
  "created_at": "2025-01-21T10:00:00Z",
  "started_at": "2025-01-21T10:00:05Z"
}
```

---

### `DELETE /agents/{id}`

Stop and remove an agent.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `force` | bool | false | Kill immediately without graceful shutdown |

**Response:** `204 No Content`

---

### `POST /agents/{id}/stop`

Gracefully stop an agent (keeps container for inspection).

**Request Body (optional):**
```json
{
  "timeout_seconds": 30
}
```

**Response:** Updated agent object with `status: "stopped"`

---

### `POST /agents/{id}/restart`

Restart an agent with the same configuration.

**Response:** Updated agent object with `status: "starting"`

---

### `POST /agents/{id}/continue`

Send a follow-up message to continue the conversation with the agent.

**Request Body:**
```json
{
  "message": "Great progress! Now also add unit tests for the new function.",
  "wait_for_idle": false,
  "timeout_seconds": 300
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message` | string | ✅ | Follow-up message (1-10000 chars) |
| `wait_for_idle` | bool | - | Wait for agent to be ready (default: false) |
| `timeout_seconds` | int | - | Max wait time if wait_for_idle (default: 300) |

**Use Cases:**
- Provide additional instructions
- Answer questions the agent asked
- Request changes to completed work
- Give feedback and ask for revisions

**Response:** Updated agent object

**Errors:**
- `400 Bad Request` - Agent not in running/idle state

---

### `GET /agents/{id}/logs`

Get agent container logs.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `stream` | bool | false | Enable SSE streaming |
| `tail` | int | 100 | Lines from end (0 = all) |
| `since` | datetime | - | Show logs since (RFC3339) |
| `timestamps` | bool | false | Include timestamps |

**Response (polling):**
```json
{
  "logs": "Starting Claude Code...\nAnalyzing task...\n...",
  "truncated": false
}
```

**Response (streaming):** `text/event-stream`
```
data: Starting Claude Code...
data: Analyzing task...
data: Reading file src/auth.go...
```

---

### `POST /agents/{id}/exec`

Execute a command inside the agent container.

**Request Body:**
```json
{
  "command": ["git", "status"],
  "working_dir": "/workspace",
  "timeout_seconds": 30
}
```

**Response:**
```json
{
  "exit_code": 0,
  "stdout": "On branch main\nnothing to commit, working tree clean\n",
  "stderr": ""
}
```

---

### `GET /agents/{id}/files`

List files modified by the agent.

**Response:**
```json
{
  "files": [
    {
      "path": "src/auth.go",
      "status": "modified",
      "size_bytes": 2048,
      "modified_at": "2025-01-21T10:15:00Z"
    },
    {
      "path": "src/auth_test.go",
      "status": "added",
      "size_bytes": 1024,
      "modified_at": "2025-01-21T10:16:00Z"
    }
  ]
}
```

---

### `GET /agents/{id}/files/{path}`

Download a specific file from the agent workspace.

**Response:** File content as `application/octet-stream` or `text/plain`

---

## Workspace Endpoints

### `GET /workspaces`

List registered workspaces.

**Response:**
```json
{
  "workspaces": [
    {
      "id": "ws-123",
      "name": "my-project",
      "git_url": "https://github.com/user/repo.git",
      "default_branch": "main",
      "created_at": "2025-01-20T09:00:00Z"
    }
  ]
}
```

---

### `POST /workspaces`

Register a new workspace.

**Request Body:**
```json
{
  "name": "my-project",
  "git_url": "https://github.com/user/repo.git",
  "default_branch": "main"
}
```

**Response:** `201 Created` with workspace object

---

## Agent Status Values

| Status | Description |
|--------|-------------|
| `pending` | Container being created |
| `starting` | Container starting up |
| `running` | Agent actively working |
| `idle` | Agent waiting for input |
| `stopping` | Graceful shutdown in progress |
| `stopped` | Container stopped |
| `failed` | Container crashed or error |
| `completed` | Task completed successfully |

---

## Error Responses

All errors follow this format:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "details": {
    "field": "workspace.git_url",
    "reason": "must be a valid git URL"
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `bad_request` | 400 | Invalid request body or parameters |
| `unauthorized` | 401 | Missing or invalid auth token |
| `forbidden` | 403 | Insufficient permissions |
| `not_found` | 404 | Agent or resource not found |
| `invalid_state` | 400 | Agent not in expected state |
| `limit_exceeded` | 429 | Too many concurrent agents |
| `internal_error` | 500 | Server error |

---

## Rate Limits

- **100 requests/minute** per user
- **10 concurrent agents** per user (configurable)

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705834800
```

---

## OpenAPI Generation

The OpenAPI 3.1 specification is auto-generated from Go code using [swaggo/swag](https://github.com/swaggo/swag).

```bash
# Generate OpenAPI spec
make swagger

# Output: api/openapi.yaml
```

See handler functions in `internal/api/handlers/` for swagger annotations.
