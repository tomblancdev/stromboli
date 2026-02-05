# API Overview

Stromboli exposes a REST API for managing Claude agents.

## Base URL

```
http://localhost:8080
```

## Authentication

Authentication is optional. When enabled, use JWT tokens:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/run
```

See [Authentication](authentication.md) for details.

## Endpoints Summary

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check with component status |
| GET | `/metrics` | Prometheus metrics |
| GET | `/claude/status` | Claude credentials status |
| GET | `/secrets` | List available Podman secrets |

### Execution

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/run` | Run Claude synchronously |
| GET | `/run/stream` | Run with streaming output |
| POST | `/run/async` | Run asynchronously |

### Jobs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/jobs` | List all jobs |
| GET | `/jobs/:id` | Get job status |
| DELETE | `/jobs/:id` | Cancel a job |

### Sessions

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/sessions` | List all sessions |
| DELETE | `/sessions/:id` | Delete a session |
| GET | `/sessions/:id/messages` | Get session messages |

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/token` | Generate JWT tokens |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/validate` | Validate a token |
| POST | `/auth/logout` | Invalidate a token |

### Images

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/images` | List local images (sorted by compatibility) |
| GET | `/images/:name` | Inspect a specific image |
| GET | `/images/search` | Search container registries |
| POST | `/images/pull` | Pull an image from a registry |

## Request Format

All POST endpoints accept JSON:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello",
    "workdir": "/workspace",
    "claude": { ... },
    "podman": { ... }
  }'
```

## Response Format

Successful responses:
```json
{
  "id": "run-abc123",
  "status": "completed",
  "output": "...",
  "session_id": "..."
}
```

Error responses:
```json
{
  "error": "Error message",
  "status": "error"
}
```

## HTTP Status Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Bad request (invalid input) |
| 401 | Unauthorized (auth required) |
| 404 | Not found |
| 429 | Rate limited |
| 500 | Internal server error |
| 503 | Service unavailable (Claude not configured) |

## Rate Limiting

When enabled, rate limits are applied per IP:

- Default: 10 requests/second
- Burst: 20 requests

Rate limit headers:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640000000
```

## OpenAPI Spec

Interactive API documentation and downloadable specs:

- **[OpenAPI Reference](openapi.md)** - Swagger UI, ReDoc, and downloadable YAML/JSON

When running locally, the spec is also served at:
```
http://localhost:8080/swagger/doc.json
```
