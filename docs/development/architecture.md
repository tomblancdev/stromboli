# Architecture

Overview of Stromboli's internal architecture.

## High-Level Architecture

```mermaid
graph TB
    subgraph Client
        A[HTTP Client]
    end

    subgraph Stromboli
        B[Gin Router]
        C[API Handlers]
        D[Runner]
        E[Session Manager]
        F[Secrets Manager]
        G[Job Manager]
    end

    subgraph Podman
        H[Podman Socket]
        I[Agent Container]
        J[Claude CLI]
    end

    subgraph Storage
        K[Sessions Dir]
        L[Podman Secrets]
    end

    A -->|HTTP| B
    B --> C
    C --> D
    C --> G
    D --> E
    D --> F
    D -->|exec| H
    H --> I
    I --> J
    E --> K
    F --> L
```

## Package Structure

```
stromboli/
├── cmd/stromboli/           # Entry point
│   └── main.go              # Minimal - just wiring
│
├── internal/                # Private packages
│   ├── api/                 # HTTP layer
│   │   ├── server.go        # Router setup
│   │   ├── run.go           # Request/response types
│   │   ├── async.go         # Async job handlers
│   │   ├── health.go        # Health checks
│   │   ├── auth.go          # JWT authentication
│   │   └── middleware.go    # Rate limiting, logging
│   │
│   ├── runner/              # Container execution
│   │   ├── runner.go        # PodmanRunner
│   │   ├── executor.go      # Command execution interface
│   │   ├── image.go         # Image validation
│   │   └── validation.go    # Input validation
│   │
│   ├── podman/              # Podman command building
│   │   └── builder.go       # Fluent command builder
│   │
│   ├── claude/              # Claude CLI integration
│   │   └── builder.go       # CLI argument builder
│   │
│   ├── secrets/             # Credentials management
│   │   └── secrets.go       # Podman secrets sync
│   │
│   ├── session/             # Session storage
│   │   └── manager.go       # Create/destroy sessions
│   │
│   ├── job/                 # Async job management
│   │   └── manager.go       # Job lifecycle
│   │
│   ├── workspace/           # Workspace validation
│   │   └── validator.go     # Path allowlist
│   │
│   ├── auth/                # Authentication
│   │   ├── jwt.go           # Token generation
│   │   └── middleware.go    # Auth middleware
│   │
│   ├── config/              # Configuration
│   │   └── config.go        # Viper config loading
│   │
│   └── types/               # Shared types
│       └── options.go       # ClaudeOptions, PodmanOptions
│
├── docs/                    # Documentation (MkDocs)
├── deployments/             # Docker/Compose files
└── api/                     # OpenAPI specs
```

## Request Flow

### Synchronous Execution

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Handler
    participant R as Runner
    participant P as Podman
    participant S as Storage

    C->>A: POST /run
    A->>A: Validate request
    A->>R: Run(ctx, request)
    R->>R: Sync credentials
    R->>R: Validate workspace
    R->>R: Build Podman command
    R->>P: podman run ...
    P->>P: Execute Claude
    P->>S: Write session data
    P-->>R: Output
    R-->>A: Result
    A-->>C: JSON response
```

### Async Execution

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Handler
    participant J as Job Manager
    participant R as Runner

    C->>A: POST /run/async
    A->>J: CreateJob()
    J-->>A: job_id
    A-->>C: {job_id, status: pending}

    J->>R: Run(ctx, request)
    Note over R: Execution happens async

    C->>A: GET /jobs/{id}
    A->>J: GetJob(id)
    J-->>A: Job status
    A-->>C: {status: running/completed}
```

## Key Interfaces

### Runner

```go
type Runner interface {
    Run(ctx context.Context, req Request) (*Result, error)
    RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)
    RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
    DestroySession(sessionID string) error
    ListSessions() ([]string, error)
}
```

### Executor

```go
type Executor interface {
    Run(ctx context.Context, args []string) ([]byte, error)
    RunStream(ctx context.Context, args []string) (stdout, stderr io.ReadCloser, start func() error, wait func() error, err error)
}
```

## Security Model

### Container Isolation

Each agent runs in an isolated Podman container with:
- Separate network namespace
- Resource limits (CPU, memory)
- Read-only root filesystem
- Non-root user
- No privileged capabilities

### Credentials

```
Host                          Container
~/.claude/.credentials.json → Podman Secret → /home/user/.claude/.credentials.json
```

Credentials never pass through the API - they're injected via Podman secrets.

### Workspace Security

```
API Request          Validation           Container
workspace=/foo → Allowlist check → Mount at /workspace
```

Only directories in the allowlist can be mounted.

## Configuration Flow

```mermaid
graph LR
    A[Environment Variables] --> D[Viper]
    B[Config File] --> D
    C[Defaults] --> D
    D --> E[Config Struct]
    E --> F[Server]
    E --> G[Runner]
```

## Error Handling

Errors are wrapped with context at each layer:

```go
// runner/runner.go
return nil, fmt.Errorf("failed to create container: %w", err)

// api/server.go
return nil, fmt.Errorf("execution failed: %w", err)
```

API returns structured errors:
```json
{
  "error": "execution failed: failed to create container: image not found",
  "status": "error"
}
```
