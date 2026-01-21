# Stromboli Architecture 🎭

> The puppet master for Claude Code containers

## Overview

Stromboli is a container orchestration API for spawning and managing isolated Claude Code instances in Podman containers. Inspired by [Pinocchio](https://github.com/anthropics/pinocchio) (Docker-based), Stromboli provides a self-hosted, API-first alternative using Podman.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         External Clients                             │
│              (CLI, Web UI, MCP Servers, CI/CD, etc.)                │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │ HTTPS
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Authentik (OAuth2)                            │
│                    ┌─────────────────────────┐                      │
│                    │  - User Management      │                      │
│                    │  - OAuth2/OIDC Provider │                      │
│                    │  - Token Validation     │                      │
│                    │  - RBAC Policies        │                      │
│                    └─────────────────────────┘                      │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │ JWT Tokens
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Stromboli Core API (Go)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │   HTTP/REST  │  │   Podman     │  │   Agent      │              │
│  │   Handlers   │──│   Manager    │──│   Registry   │              │
│  │   (Gin)      │  │   (Bindings) │  │   (State)    │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│         │                  │                  │                     │
│         ▼                  ▼                  ▼                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │   Auth       │  │   Container  │  │   SQLite/    │              │
│  │   Middleware │  │   Lifecycle  │  │   PostgreSQL │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │ Unix Socket
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Podman Socket Service                          │
│                 /run/user/UID/podman/podman.sock                    │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
              ┌───────────────────┼───────────────────┐
              ▼                   ▼                   ▼
       ┌────────────┐      ┌────────────┐      ┌────────────┐
       │  Claude    │      │  Claude    │      │  Claude    │
       │  Agent #1  │      │  Agent #2  │      │  Agent #n  │
       │            │      │            │      │            │
       │ - Workspace│      │ - Workspace│      │ - Workspace│
       │ - Creds    │      │ - Creds    │      │ - Creds    │
       │ - Logs     │      │ - Logs     │      │ - Logs     │
       └────────────┘      └────────────┘      └────────────┘
```

## Core Components

### 1. Stromboli Core API (Go + Gin)

The main orchestration service written in Go.

**Responsibilities:**
- REST API endpoint handling
- OAuth2 token validation (via Authentik)
- Container lifecycle management (create, start, stop, remove)
- Agent state tracking and persistence
- Log streaming and aggregation
- Resource quota enforcement

**Key Packages:**
```
stromboli/
├── cmd/stromboli/          # Application entry point
├── internal/
│   ├── api/                # HTTP handlers and routes
│   │   ├── handlers/       # Request handlers
│   │   ├── middleware/     # Auth, logging, rate limiting
│   │   └── routes.go       # Route definitions
│   ├── container/          # Podman container management
│   │   ├── client.go       # Podman bindings wrapper
│   │   ├── lifecycle.go    # Container lifecycle operations
│   │   └── spec.go         # Container spec builders
│   ├── agent/              # Claude agent abstractions
│   │   ├── agent.go        # Agent model and state
│   │   ├── registry.go     # In-memory agent tracking
│   │   └── credentials.go  # Credential management
│   ├── auth/               # Authentication
│   │   ├── oauth.go        # OAuth2/OIDC integration
│   │   └── jwt.go          # JWT validation
│   ├── config/             # Configuration management
│   └── store/              # Persistence layer
│       ├── store.go        # Storage interface
│       └── sqlite.go       # SQLite implementation
├── api/                    # OpenAPI specs
├── configs/                # Configuration files
└── deployments/            # Docker/Podman compose files
```

### 2. Authentik (OAuth2/OIDC Provider)

External identity provider running as a separate pod.

**Features Used:**
- OAuth2 Authorization Code flow (for Web UI)
- Client Credentials flow (for service-to-service)
- JWT token issuance and validation
- User/group management
- RBAC for agent permissions

**Integration Points:**
- Stromboli validates JWTs on every request
- Users authenticate via Authentik login page
- API keys map to Authentik service accounts

### 3. Claude Agent Containers

Isolated Podman containers running Claude Code CLI.

**Container Configuration:**
- Base image: Custom devcontainer with Claude Code pre-installed
- Mounts: Workspace directory (configurable read/write)
- Environment: Claude API credentials, task context
- Network: Restricted egress (whitelist-only)
- Resources: CPU/memory limits per container

## Data Flow

### Agent Spawn Flow

```
1. Client → POST /api/v1/agents (with task, workspace config)
2. Stromboli validates OAuth token via Authentik
3. Stromboli creates container spec:
   - Generates unique agent ID
   - Prepares workspace mount
   - Injects credentials
   - Sets resource limits
4. Stromboli calls Podman bindings to create container
5. Container starts with Claude Code CLI
6. Stromboli stores agent state in database
7. Client receives agent ID and status
```

### Agent Status Flow

```
1. Client → GET /api/v1/agents/{id}
2. Stromboli checks in-memory registry
3. Stromboli queries Podman for container status
4. Stromboli returns merged state to client
```

### Log Streaming Flow

```
1. Client → GET /api/v1/agents/{id}/logs (SSE)
2. Stromboli validates permissions
3. Stromboli opens log stream from Podman
4. Logs streamed to client via Server-Sent Events
```

## Security Model

### Authentication Layers

| Layer | Mechanism | Purpose |
|-------|-----------|---------|
| Client → API | OAuth2 Bearer Token | User/service identity |
| API → Authentik | OIDC Discovery | Token validation |
| API → Podman | Unix Socket | Local IPC (no auth) |
| Container → Claude API | API Key | Claude authentication |

### Container Isolation

- **User Namespaces**: Containers run as unprivileged users
- **Network Policies**: Egress restricted to whitelist
- **Resource Limits**: CPU/memory caps per container
- **Read-only Rootfs**: Base filesystem is immutable
- **Seccomp Profiles**: Restricted syscalls

### Credential Management

```
┌─────────────────┐
│ Credential      │     Credentials never stored in Stromboli DB
│ Sources         │     - Passed at spawn time
│                 │     - Injected as env vars
│ - User provides │     - Optionally fetched from secrets manager
│ - Vault/Secrets │
│ - Authentik     │
└─────────────────┘
```

## Deployment Topology

### Development (Single Host)

```yaml
# podman-compose.yml
services:
  stromboli:
    build: .
    ports: ["8080:8080"]
    volumes:
      - /run/user/1000/podman/podman.sock:/run/podman/podman.sock

  authentik:
    image: ghcr.io/goauthentik/server
    # ... authentik config

  postgres:  # For Authentik
    image: postgres:15
```

### Production (Multi-Host)

- Stromboli API behind reverse proxy (Traefik/Caddy)
- Authentik as shared identity provider
- Podman on dedicated worker nodes
- PostgreSQL for persistence
- Redis for caching (optional)

## Future Extensions

### Phase 2: MCP Integration
- MCP server pod for tool extensions
- Filesystem, web, database tools
- Custom tool development SDK

### Phase 3: Web UI
- React/Next.js dashboard
- Real-time agent monitoring
- Task templates and scheduling
- Multi-user workspace management

### Phase 4: Clustering
- Multi-node Podman orchestration
- Agent scheduling and placement
- Distributed state management

## Configuration

### Environment Variables

```bash
# Core
STROMBOLI_PORT=8080
STROMBOLI_LOG_LEVEL=info

# Podman
STROMBOLI_PODMAN_SOCKET=/run/user/1000/podman/podman.sock
STROMBOLI_AGENT_IMAGE=ghcr.io/stromboli/claude-agent:latest

# Auth
STROMBOLI_OAUTH_ISSUER=https://auth.example.com
STROMBOLI_OAUTH_CLIENT_ID=stromboli
STROMBOLI_OAUTH_AUDIENCE=stromboli-api

# Database
STROMBOLI_DB_PATH=/data/stromboli.db
```

## Testing Strategy (TDD)

We follow **Test-Driven Development** throughout the project. Tests are written before implementation.

### Testing Pyramid

```
         ┌─────────────────┐
         │   E2E Tests     │  ← Podman integration, full API flows
         │   (few, slow)   │
         ├─────────────────┤
         │ Integration     │  ← Handler tests with mocked Podman
         │ Tests (medium)  │
         ├─────────────────┤
         │   Unit Tests    │  ← Pure logic, no dependencies
         │  (many, fast)   │
         └─────────────────┘
```

### Test Categories

| Category | Location | What it tests | Tools |
|----------|----------|---------------|-------|
| **Unit** | `*_test.go` alongside code | Business logic, validators, parsers | Go `testing`, `testify` |
| **Integration** | `internal/*/integration_test.go` | API handlers, DB queries | `httptest`, `testcontainers` |
| **E2E** | `tests/e2e/` | Full spawn→run→stop flows | `testcontainers-go`, real Podman |

### Testing Tools

```go
// Dependencies
"github.com/stretchr/testify/assert"    // Assertions
"github.com/stretchr/testify/mock"      // Mocking
"github.com/stretchr/testify/suite"     // Test suites
"github.com/testcontainers/testcontainers-go"  // Container testing
```

### TDD Workflow

```
1. Write failing test (RED)
   └─ Define expected behavior first

2. Write minimal code to pass (GREEN)
   └─ Simplest implementation that works

3. Refactor (REFACTOR)
   └─ Clean up while keeping tests green

4. Repeat
```

### Project Structure with Tests

```
stromboli/
├── internal/
│   ├── agent/
│   │   ├── agent.go
│   │   ├── agent_test.go          # Unit tests
│   │   └── integration_test.go    # Integration tests
│   ├── container/
│   │   ├── client.go
│   │   ├── client_test.go
│   │   └── mocks/
│   │       └── podman_mock.go     # Generated mocks
│   └── api/
│       ├── handlers/
│       │   ├── agents.go
│       │   └── agents_test.go
├── tests/
│   ├── e2e/                       # End-to-end tests
│   │   ├── spawn_test.go
│   │   └── lifecycle_test.go
│   └── fixtures/                  # Test data
│       └── sample_workspace/
└── Makefile                       # test, test-unit, test-integration, test-e2e
```

### Coverage Goals

| Package | Target Coverage |
|---------|-----------------|
| `agent/` | 90%+ |
| `container/` | 80%+ |
| `api/handlers/` | 85%+ |
| `auth/` | 90%+ |

### CI Pipeline

```yaml
# .github/workflows/test.yml
jobs:
  test:
    steps:
      - run: make test-unit        # Fast, no containers
      - run: make test-integration # With testcontainers
      - run: make test-e2e         # Full Podman (on self-hosted runner)
      - run: make coverage-report
```

### Mocking Strategy

- **Podman bindings**: Interface wrapper + mock for unit tests
- **HTTP clients**: `httptest.Server` for external services
- **Database**: In-memory SQLite for integration tests
- **Time/UUID**: Inject generators for deterministic tests

## Technology Choices Rationale

| Choice | Reasoning |
|--------|-----------|
| **Go** | Native Podman bindings, excellent for container tooling, single binary |
| **Gin** | Popular, simple, great for learning Go, extensive middleware |
| **Podman** | Rootless security, daemonless, OCI-compliant, K8s YAML support |
| **Authentik** | Modern, full-featured, lighter than Keycloak, good API |
| **SQLite** | Simple MVP persistence, upgrade to PostgreSQL later |

## References

- [Podman Go Bindings](https://github.com/containers/podman/blob/main/pkg/bindings/README.md)
- [Pinocchio MCP](https://github.com/anthropics/pinocchio) (Docker-based inspiration)
- [agentctl](https://github.com/jordanpartridge/agentctl) (Similar CLI tool)
- [Claude Code DevContainer](https://code.claude.com/docs/en/devcontainer)
- [Authentik Docs](https://docs.goauthentik.io/)
