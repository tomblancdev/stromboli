# Stromboli - Claude Guidelines 🎭

## Project Overview

Stromboli is a container orchestration API for spawning and managing isolated Claude Code agents in Podman containers. It's the Podman-based alternative to Pinocchio.

## Development Philosophy

### 1. Test-Driven Development (TDD) 🧪

**Always write tests FIRST.**

```
RED    → Write a failing test
GREEN  → Write minimal code to pass
REFACTOR → Clean up while keeping tests green
```

- Unit tests go alongside code: `agent.go` → `agent_test.go`
- Integration tests: `internal/*/integration_test.go`
- E2E tests: `tests/e2e/`
- Run tests before committing: `make test`
- Target coverage: 80%+ for all packages

### 2. Keep It Simple (KISS) 🎯

- **No premature optimization** - Make it work, then make it fast
- **No over-engineering** - Solve today's problem, not tomorrow's
- **No unnecessary abstractions** - If you only need it once, don't abstract it
- **Prefer standard library** - Only add dependencies when truly needed
- **Small functions** - Each function does one thing well
- **Flat is better than nested** - Avoid deep nesting

### 3. Code Organization 📁

```
stromboli/
├── cmd/stromboli/       # Entry point only - no business logic
├── internal/            # Private application code
│   ├── api/            # HTTP handlers, middleware, routes
│   ├── agent/          # Agent domain logic
│   ├── container/      # Podman integration
│   ├── auth/           # OAuth/JWT handling
│   ├── config/         # Configuration loading
│   └── store/          # Database layer
├── pkg/                # Public libraries (if any)
├── tests/e2e/          # End-to-end tests
├── api/                # Generated OpenAPI specs
├── configs/            # Config file templates
└── deployments/        # Docker/Podman compose files
```

**Rules:**
- `internal/` is private - don't expose implementation details
- Each package has a single responsibility
- Avoid circular dependencies
- Keep `cmd/` thin - just wire up and start

### 4. Documentation 📝

**Code should be self-documenting, but:**

- Add godoc comments to all exported functions/types
- Explain "why", not "what" in comments
- Keep README.md updated
- Document non-obvious decisions in code

```go
// Good: Explains why
// RetryWithBackoff handles transient network failures common with Podman socket

// Bad: Explains what (obvious from code)
// This function retries the operation
```

## Tech Stack

| Component | Technology | Notes |
|-----------|------------|-------|
| Language | Go 1.22+ | Use latest stable |
| Web Framework | Gin | Simple, popular |
| Podman | v5+ bindings | `github.com/containers/podman/v5/pkg/bindings` |
| Testing | testify + testcontainers | Mock interfaces, not implementations |
| Config | Viper | Env vars + config files |
| Logging | slog (stdlib) | Structured logging |
| Auth | OAuth2/JWT | Via Authentik |
| Database | SQLite → PostgreSQL | Start simple, migrate later |

## Coding Standards

### Error Handling

```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create container: %w", err)
}

// Use custom error types for domain errors
var ErrAgentNotFound = errors.New("agent not found")
```

### Interfaces

```go
// Define interfaces where they're used, not where they're implemented
// Keep interfaces small (1-3 methods)

type ContainerManager interface {
    Create(ctx context.Context, spec ContainerSpec) (string, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string) error
}
```

### Context

```go
// Always pass context as first parameter
// Use context for cancellation and timeouts
func (s *Service) SpawnAgent(ctx context.Context, req SpawnRequest) (*Agent, error)
```

### Naming

- **Packages**: lowercase, short, no underscores (`container`, not `container_manager`)
- **Files**: lowercase with underscores (`agent_test.go`)
- **Exported**: PascalCase (`SpawnAgent`)
- **Unexported**: camelCase (`createContainer`)
- **Interfaces**: `-er` suffix when possible (`Reader`, `Manager`)

## Development Workflow

### Before Writing Code

1. Check existing tests
2. Read related code
3. Write failing test first
4. Plan minimal implementation

### Commit Guidelines

```
feat: add agent continue endpoint
fix: handle container timeout gracefully
test: add integration tests for spawn flow
docs: update API documentation
refactor: extract container spec builder
```

### Pull Request Checklist

- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] New code has tests
- [ ] Documentation updated if needed
- [ ] No hardcoded values (use config)

## Container Development

All development happens in containers. Never install directly on the host.

```bash
# Start development environment
make dev

# Run tests in container
make test

# Build production image
make build
```

## Useful Commands

```bash
make test          # Run all tests
make test-unit     # Run unit tests only
make test-e2e      # Run E2E tests
make lint          # Run linter
make swagger       # Generate OpenAPI spec
make dev           # Start dev environment
make build         # Build production image
```

## Project Conventions for Claude Sessions

- **Branch flow**: never commit directly to `main`. Use `feat/`, `fix/`, `docs/`, `refactor/`, `test/`, `ci/` prefixes. PRs only. The `block-main-commit` hook enforces this.
- **Audit issues**: GitHub issues labelled `audit` (with `critical` / `high` / `medium` / `low`) are the security/infra triage backlog. Treat severity as priority.
- **Agent worktrees**: `.stromboli-worktrees/` contains disposable copies the runtime spawns. Treat them as build artefacts — never edit, never commit, safe to clean.
- **Go toolchain**: not on the host. Everything runs through `golang:1.24` via Podman (see Makefile `GO_RUN`). The first hook-triggered run will pull ~500MB; pre-pull with `podman pull golang:1.24` to avoid surprise.

## Claude Code Setup

This project ships with hooks and slash commands under `.claude/`:

| File | Purpose |
|------|---------|
| `.claude/settings.json` | Permissions allowlist + hooks wiring (committed). |
| `.claude/hooks/gofmt-on-edit.sh` | PostToolUse: gofmt on any `*.go` edit. |
| `.claude/hooks/block-main-commit.sh` | PreToolUse: rejects `git commit` while on `main`. |
| `.claude/hooks/session-end-check.sh` | Stop: runs `go vet` on changed packages + reminds to test. |
| `.claude/commands/run-tests.md` | `/run-tests [unit\|integration\|e2e\|all\|coverage]` |
| `.claude/commands/sweep-deps.md` | `/sweep-deps` — triage Dependabot PRs. |

## References

- [Architecture Doc](docs/ARCHITECTURE.md)
- [API Documentation](docs/API.md)
- [Podman Go Bindings](https://github.com/containers/podman/blob/main/pkg/bindings/README.md)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
