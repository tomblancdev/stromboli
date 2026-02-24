# API Overview

Stromboli exposes a REST API for spawning and managing Claude agents.

## Base URL

```
http://localhost:8080
```

## Authentication

Optional. When enabled, use JWT tokens:

```bash
curl -H "Authorization: Bearer <token>" localhost:8080/run
```

See [authentication](authentication.md) for setup.

## Endpoints

### Execution

| Method | Endpoint | Description |
|---|---|---|
| POST | `/run` | Run Claude synchronously |
| GET | `/run/stream` | Stream output via SSE |
| POST | `/run/async` | Run asynchronously (returns job ID) |

### Jobs

| Method | Endpoint | Description |
|---|---|---|
| GET | `/jobs` | List all jobs |
| GET | `/jobs/:id` | Get job status and result |
| DELETE | `/jobs/:id` | Cancel a job |

### Sessions

| Method | Endpoint | Description |
|---|---|---|
| GET | `/sessions` | List session IDs |
| DELETE | `/sessions/:id` | Delete a session |
| GET | `/sessions/:id/messages` | Get conversation messages |

### Auth

| Method | Endpoint | Description |
|---|---|---|
| POST | `/auth/token` | Generate JWT tokens |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/validate` | Check token validity |
| POST | `/auth/logout` | Invalidate a token |

### Images

| Method | Endpoint | Description |
|---|---|---|
| GET | `/images` | List local images (sorted by compatibility) |
| GET | `/images/:name` | Inspect a specific image |
| GET | `/images/search` | Search container registries |
| POST | `/images/pull` | Pull an image |

### System

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Health check with component status |
| GET | `/metrics` | Prometheus metrics |
| GET | `/claude/status` | Claude credentials status |
| GET | `/secrets` | List available Podman secrets |

## Request format

All POST endpoints accept JSON:

```bash
curl -X POST localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello",
    "workdir": "/workspace",
    "claude": { ... },
    "podman": { ... }
  }'
```

## Response format

Success:

```json
{"id": "run-abc123", "status": "completed", "output": "...", "session_id": "..."}
```

Error:

```json
{"error": "Error message", "status": "error"}
```

## Status codes

| Code | Meaning |
|---|---|
| 200 | Success |
| 400 | Bad request |
| 401 | Unauthorized |
| 404 | Not found |
| 429 | Rate limited |
| 500 | Server error |
| 503 | Service unavailable |

## Rate limiting

When enabled, applied per-IP:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640000000
```

## OpenAPI spec

Interactive docs and downloadable specs at [OpenAPI reference](openapi.md). Local server: `localhost:8080/swagger/doc.json`
