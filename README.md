# Stromboli 🌋

> Container orchestration API for spawning and managing isolated Claude Code agents in Podman containers

Stromboli is a REST API that provides secure, isolated execution of Claude AI in containerized environments. It's designed for automating development workflows, CI/CD pipelines, code analysis, and AI-assisted development tasks.

## Features

- **Isolated Execution**: Each Claude instance runs in its own Podman container with resource limits
- **Full Claude CLI Support**: All 40+ Claude headless mode options exposed via REST API
- **Multiple Execution Modes**:
  - Synchronous execution with immediate response
  - Asynchronous execution with job tracking
  - Server-Sent Events (SSE) for real-time streaming output
- **Session Management**: Persistent conversation sessions across multiple API calls
- **Webhook Notifications**: Get notified when async jobs complete
- **Resource Controls**: CPU, memory, and timeout limits per container
- **Security**: Token-based authentication, JWT with refresh tokens, workspace allowlisting, secret management
- **Observability**: Prometheus metrics, OpenTelemetry distributed tracing, structured logging, request tracking

## Quick Start

### Prerequisites

- [Podman](https://podman.io/) v5+ installed and configured
- [Go](https://golang.org/) 1.22+ (for building from source)
- Claude API access token (from [Claude.ai](https://claude.ai))

### 1. Build the Agent Image

```bash
make build-agent-image
```

This creates a `stromboli-agent:latest` image with Claude CLI installed.

### 2. Configure Claude Token

Extract your Claude token from existing credentials:

```bash
make claude-setup
```

Or manually create `.claude-secrets`:

```bash
echo "your-claude-api-token" > .claude-secrets
chmod 600 .claude-secrets
```

### 3. Build and Run

```bash
# Build the binary
make build

# Run the server
./bin/stromboli
```

The API server will start on `http://localhost:8080`.

### 4. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Run Claude synchronously
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "claude": {
      "dangerously_skip_permissions": true
    }
  }'
```

## API Endpoints

### Execution

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/run` | POST | Execute Claude synchronously (returns output immediately) |
| `/run/async` | POST | Execute Claude asynchronously (returns job ID) |
| `/run/stream` | GET | Stream Claude output in real-time via SSE |

### Job Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/jobs` | GET | List all async jobs |
| `/jobs/{id}` | GET | Get job status and result |
| `/jobs/{id}` | DELETE | Cancel a pending/running job |

### Session Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/sessions` | GET | List all sessions |
| `/sessions/{id}` | DELETE | Destroy session and all its data |

### Authentication

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/token` | POST | Generate JWT access and refresh tokens |
| `/auth/refresh` | POST | Refresh an expiring access token |
| `/auth/validate` | GET | Validate a token and return claims |
| `/auth/logout` | POST | Invalidate a token (logout) |

### System

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check with component status |
| `/metrics` | GET | Prometheus metrics |
| `/claude/status` | GET | Check Claude configuration |

## Usage Examples

### Synchronous Execution

Execute Claude and get immediate response:

```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Analyze this Go code for bugs",
    "workspace": "/home/user/myproject",
    "claude": {
      "model": "sonnet",
      "dangerously_skip_permissions": true
    },
    "podman": {
      "timeout": "5m",
      "memory": "1g",
      "cpus": "2"
    }
  }'
```

### Asynchronous Execution with Webhook

Start a long-running task and get notified when complete:

```bash
# Start async job
curl -X POST http://localhost:8080/run/async \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Refactor this entire codebase for better maintainability",
    "workspace": "/home/user/large-project",
    "webhook_url": "https://myapp.com/webhook",
    "claude": {
      "model": "opus",
      "dangerously_skip_permissions": true
    }
  }'

# Response: {"job_id": "job-abc123def456"}

# Poll for status
curl http://localhost:8080/jobs/job-abc123def456

# Cancel if needed
curl -X DELETE http://localhost:8080/jobs/job-abc123def456
```

### Streaming Output

Get real-time output via Server-Sent Events:

```bash
curl -N http://localhost:8080/run/stream?prompt=Write%20a%20REST%20API%20in%20Go
```

### Session Continuation

Continue a conversation across multiple API calls:

```bash
# First call (creates session)
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Create a new Go project structure",
    "workspace": "/home/user/newproject"
  }'

# Response includes: "session_id": "sess-abc123"

# Continue the conversation
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Now add unit tests for the main package",
    "workspace": "/home/user/newproject",
    "claude": {
      "session_id": "sess-abc123",
      "resume": true
    }
  }'
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STROMBOLI_AUTH_ENABLED` | `false` | Enable token-based authentication |
| `STROMBOLI_API_TOKENS` | - | Comma-separated list of valid API tokens |
| `STROMBOLI_JWT_SECRET` | - | Secret key for JWT signing (enables JWT auth) |
| `STROMBOLI_JWT_EXPIRY` | `24h` | JWT access token lifetime |
| `STROMBOLI_RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `STROMBOLI_RATE_LIMIT_RPS` | `10` | Requests per second limit |
| `STROMBOLI_RATE_LIMIT_BURST` | `20` | Maximum burst size |
| `STROMBOLI_TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `STROMBOLI_TRACING_ENDPOINT` | `localhost:4317` | OTLP collector endpoint |

### Authentication

When `STROMBOLI_AUTH_ENABLED=true`, include a Bearer token:

```bash
curl -X POST http://localhost:8080/run \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello"}'
```

#### JWT Authentication (Recommended)

For production, use JWT tokens with automatic expiration and refresh:

```bash
# Set JWT secret to enable JWT auth
export STROMBOLI_JWT_SECRET="your-256-bit-secret"

# Generate JWT tokens using API token
curl -X POST http://localhost:8080/auth/token \
  -H "Authorization: Bearer your-api-token" \
  -H "Content-Type: application/json" \
  -d '{"client_id": "my-app"}'

# Response: {"access_token": "eyJ...", "refresh_token": "eyJ...", "expires_in": 86400}

# Use JWT for requests
curl -H "Authorization: Bearer eyJ..." http://localhost:8080/run ...

# Logout (invalidate token)
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer eyJ..."
```

See [docs/AUTHENTICATION.md](docs/AUTHENTICATION.md) for complete JWT setup.

### Rate Limiting

Protect your API from abuse by enabling rate limiting:

```bash
export STROMBOLI_RATE_LIMIT_ENABLED=true
export STROMBOLI_RATE_LIMIT_RPS=10     # 10 requests per second
export STROMBOLI_RATE_LIMIT_BURST=20   # Allow burst of 20

./bin/stromboli
```

Rate limits are applied per IP address. When exceeded, the API returns `429 Too Many Requests`. See [docs/RATE_LIMITING.md](docs/RATE_LIMITING.md) for details.

### Distributed Tracing

Enable OpenTelemetry tracing for observability:

```bash
export STROMBOLI_TRACING_ENABLED=true
export STROMBOLI_TRACING_ENDPOINT="jaeger:4317"
export STROMBOLI_TRACING_SERVICE_NAME="stromboli-prod"

./bin/stromboli
```

Traces include HTTP requests, runner execution, and internal spans. Compatible with Jaeger, Grafana Tempo, and any OTLP-compatible backend. See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for details.

### Resource Limits

Control container resources via `podman` options:

```json
{
  "prompt": "...",
  "podman": {
    "timeout": "10m",      // Container timeout (e.g., "5m", "1h")
    "memory": "2g",        // Memory limit (e.g., "512m", "1g")
    "cpus": "2",           // CPU limit (e.g., "0.5", "2")
    "cpu_shares": 512,     // CPU shares (relative weight, default 1024)
    "volumes": [           // Additional volume mounts
      "/data:/data:ro"
    ]
  }
}
```

### Claude Options

Stromboli exposes all Claude CLI options. See [docs/API.md](docs/API.md) for complete reference. Common options:

```json
{
  "claude": {
    "model": "sonnet",                         // sonnet, opus, haiku
    "session_id": "my-session-id",             // Session ID for persistence
    "resume": true,                            // Resume existing session
    "continue": true,                          // Continue latest conversation
    "dangerously_skip_permissions": true,      // Skip permission prompts (sandboxed only)
    "permission_mode": "bypassPermissions",    // Permission mode
    "system_prompt": "You are a senior dev",   // Custom system prompt
    "max_budget_usd": 10.0,                    // Budget limit
    "timeout": "30m"                           // Claude timeout
  }
}
```

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP/REST
       ▼
┌─────────────────────────────────┐
│      Stromboli API Server       │
│  ┌────────────────────────────┐ │
│  │  Gin HTTP Handlers         │ │
│  └───────────┬────────────────┘ │
│              ▼                   │
│  ┌────────────────────────────┐ │
│  │  Runner (Orchestration)    │ │
│  └───────────┬────────────────┘ │
│              ▼                   │
│  ┌────────────────────────────┐ │
│  │  Podman Command Builder    │ │
│  └───────────┬────────────────┘ │
└──────────────┼──────────────────┘
               │ podman run
               ▼
       ┌──────────────────┐
       │ Podman Container │
       │  (isolated env)  │
       │                  │
       │  ┌────────────┐  │
       │  │ Claude CLI │  │
       │  └────────────┘  │
       │                  │
       │  /workspace      │
       │  (mounted)       │
       └──────────────────┘
```

Key components:

- **API Layer**: HTTP handlers, authentication, request validation
- **Runner Layer**: Orchestration logic, execution modes (sync/async/stream)
- **Command Builders**: Construct Podman and Claude CLI commands
- **Job Manager**: Track async jobs, cleanup, webhook notifications
- **Session Manager**: Persistent session state across calls
- **Secrets Manager**: Secure Claude token handling via Podman secrets

For detailed architecture, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Development

### Project Structure

```
stromboli/
├── cmd/stromboli/       # Application entry point
├── internal/            # Private application code
│   ├── api/            # HTTP handlers, middleware, routes
│   ├── runner/         # Orchestration logic
│   ├── podman/         # Podman command construction
│   ├── claude/         # Claude command construction
│   ├── job/            # Async job management
│   ├── session/        # Session lifecycle
│   ├── auth/           # Authentication
│   └── types/          # Shared types
├── docs/               # Documentation
├── deployments/        # Docker/Podman files
└── tests/              # Integration and E2E tests
```

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint

# Development with hot reload (requires air)
make dev
```

### Documentation Generation

```bash
# Generate all documentation
make docs

# Generate Swagger/OpenAPI spec
make docs-swagger

# Generate Go code documentation
make docs-godoc

# Serve Swagger UI locally
make docs-serve
```

### Testing

```bash
# Run all tests
make test

# Run with coverage report
make test-coverage
open coverage.html
```

## Production Deployment

### Docker/Podman Compose

```yaml
version: '3.8'
services:
  stromboli:
    build: .
    ports:
      - "8080:8080"
    environment:
      - STROMBOLI_AUTH_ENABLED=true
      - STROMBOLI_API_TOKENS=${API_TOKENS}
    volumes:
      - /var/run/podman/podman.sock:/var/run/podman/podman.sock
      - ./.claude-secrets:/app/.claude-secrets:ro
      - ./workspaces:/workspaces:rw
    restart: unless-stopped
```

### Security Considerations

1. **Enable Authentication**: Always set `STROMBOLI_AUTH_ENABLED=true` in production
2. **Workspace Allowlist**: Configure allowed workspace paths in `main.go`
3. **Resource Limits**: Set appropriate CPU/memory limits for containers
4. **Network Isolation**: Run Stromboli in isolated network
5. **Token Rotation**: Regularly rotate API tokens and Claude credentials
6. **Audit Logging**: Enable structured logging and monitor API usage

### Systemd Service

```ini
[Unit]
Description=Stromboli API Server
After=network.target podman.service

[Service]
Type=simple
User=stromboli
WorkingDirectory=/opt/stromboli
ExecStart=/opt/stromboli/bin/stromboli
Environment="STROMBOLI_AUTH_ENABLED=true"
Environment="STROMBOLI_API_TOKENS=token1,token2"
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## Documentation

- **[User Guide](docs/USER_GUIDE.md)** - Start here! Complete guide for users
- [API Reference](docs/API.md) - Complete API documentation with schemas
- [Examples](docs/EXAMPLES.md) - Practical usage examples
- [Architecture](docs/ARCHITECTURE.md) - System design and components
- [Configuration](docs/CONFIGURATION.md) - All configuration options
- [Authentication](docs/AUTHENTICATION.md) - JWT and token authentication
- [Testing](docs/TESTING.md) - Testing guide (unit, integration, E2E)
- [Vision](docs/VISION.md) - Project vision and roadmap

## Contributing

Contributions are welcome! This project follows Test-Driven Development (TDD):

1. Write failing tests first
2. Implement minimal code to pass
3. Refactor while keeping tests green

See [CLAUDE.md](CLAUDE.md) for development guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Implement the feature
5. Ensure tests pass (`make test`)
6. Run linter (`make lint`)
7. Commit your changes (`git commit -m 'Add amazing feature'`)
8. Push to the branch (`git push origin feature/amazing-feature`)
9. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/tomblanc/stromboli/issues)
- **Discussions**: [GitHub Discussions](https://github.com/tomblanc/stromboli/discussions)

## Related Projects

- [Pinocchio](https://github.com/tomblanc/pinocchio) - Docker-based alternative
- [Claude Code](https://claude.ai/claude-code) - Official Claude CLI tool
- [n8n](https://n8n.io/) - Workflow automation (see [VISION.md](docs/VISION.md) for integration)

## Acknowledgments

Built with:
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [Podman](https://podman.io/) - Container engine
- [Claude AI](https://claude.ai/) - AI assistant
- [Prometheus](https://prometheus.io/) - Monitoring and metrics

---

**Stromboli** - Because your CI/CD pipeline deserves an AI volcano 🌋
