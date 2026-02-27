# Architecture

How Stromboli is organized internally.

## High-level overview

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

## Package structure

```
stromboli/
├── cmd/stromboli/        # Entry point (main.go — just wiring)
├── internal/
│   ├── api/              # HTTP handlers, middleware, routes
│   ├── runner/           # Container execution, validation
│   ├── podman/           # Podman command builder
│   ├── claude/           # Claude CLI argument builder
│   ├── secrets/          # Podman secrets sync
│   ├── session/          # Session storage management
│   ├── job/              # Async job lifecycle
│   ├── workspace/        # Path allowlist validation
│   ├── auth/             # JWT tokens, auth middleware
│   ├── config/           # Viper config loading
│   ├── types/            # Shared types (ClaudeOptions, PodmanOptions)
│   ├── tracing/          # OpenTelemetry
│   └── version/          # Version info
├── docs/                 # MkDocs documentation
├── deployments/          # Docker/Compose files
└── api/                  # OpenAPI specs
```

## Request flow

### Synchronous

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Handler
    participant R as Runner
    participant P as Podman

    C->>A: POST /run
    A->>A: Validate request
    A->>R: Run(ctx, request)
    R->>R: Sync credentials
    R->>R: Validate volumes & image
    R->>R: Build Podman command
    R->>P: podman run ...
    P-->>R: Output
    R-->>A: Result
    A-->>C: JSON response
```

### Async

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Handler
    participant J as Job Manager
    participant R as Runner

    C->>A: POST /run/async
    A->>J: CreateJob()
    J-->>A: job_id
    A-->>C: {job_id, session_id}

    J->>R: Run(ctx, request)
    Note over R: Runs in background

    C->>A: GET /jobs/{id}
    A->>J: GetJob(id)
    A-->>C: {status: completed, output: ...}
```

## Key interfaces

```go
// Runner — executes Claude in containers
type Runner interface {
    Run(ctx context.Context, req Request) (*Result, error)
    RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)
    RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
    DestroySession(sessionID string) error
    ListSessions() ([]string, error)
}

// Executor — runs shell commands (mockable for tests)
type Executor interface {
    Run(ctx context.Context, args []string) ([]byte, error)
    RunStream(ctx context.Context, args []string) (stdout, stderr io.ReadCloser, start func() error, wait func() error, err error)
}
```

## Security model

Credentials flow through Podman secrets, never through the API:

```
Host: ~/.claude/.credentials.json → Podman Secret → Container: /home/user/.claude/.credentials.json
```

Volumes go through allowlist validation before any mount happens:

```
API Request: volumes=["/home/user/code:/workspace"] → Allowlist check → Mount
```

## Configuration flow

```mermaid
graph LR
    A[Environment Variables] --> D[Viper]
    B[Config File] --> D
    C[Defaults] --> D
    D --> E[Config Struct]
    E --> F[Server]
    E --> G[Runner]
```

## Error handling

Errors wrap with context at each layer:

```go
// runner layer
return nil, fmt.Errorf("failed to create container: %w", err)

// api layer
return nil, fmt.Errorf("execution failed: %w", err)
```

API returns structured JSON errors:

```json
{"error": "execution failed: image not found", "status": "error"}
```
