# Contributing

Thank you for considering contributing to Stromboli! This guide will help you get started.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/tomblancdev/stromboli
cd stromboli

# Run tests
make test

# Build
make build

# Run locally
./stromboli
```

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.24+ | Language runtime |
| Podman | 4.0+ | Container runtime |
| Make | Any | Build automation |
| Git | Any | Version control |

### Clone & Build

```bash
# Clone
git clone https://github.com/tomblancdev/stromboli
cd stromboli

# Install dependencies
go mod download

# Build
make build

# Run
./stromboli
```

### Run Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Integration tests (requires Podman)
make test-integration

# With coverage report
make test-coverage
open coverage.html
```

### Development Server

```bash
# Start Stromboli locally
make run

# With hot reload (requires air)
make dev

# With debug logging
STROMBOLI_LOG_LEVEL=debug make run
```

---

## Code Architecture

### Project Structure

```
stromboli/
├── cmd/stromboli/           # Entry point (main.go only)
├── internal/                # Private application code
│   ├── api/                # HTTP handlers & middleware
│   │   ├── server.go      # Server setup & routes
│   │   ├── handlers.go    # Request handlers
│   │   ├── middleware.go  # Auth, rate limiting
│   │   └── health.go      # Health checks
│   ├── runner/             # Container execution
│   │   ├── runner.go      # PodmanRunner implementation
│   │   ├── executor.go    # Shell command execution
│   │   ├── image.go       # Image validation
│   │   └── cli_image.go   # CLI image management
│   ├── auth/               # Authentication
│   │   ├── jwt.go         # JWT token handling
│   │   ├── middleware.go  # Auth middleware
│   │   └── blacklist.go   # Token blacklist
│   ├── config/             # Configuration loading
│   ├── claude/             # Claude credentials
│   ├── job/                # Async job management
│   ├── secrets/            # Podman secrets
│   ├── tracing/            # OpenTelemetry
│   └── version/            # Version info
├── deployments/            # Docker/Compose files
│   └── docker/
│       ├── Dockerfile.server
│       └── Dockerfile.claude-cli
├── install/                # Installation files
├── docs/                   # MkDocs documentation
├── api/                    # OpenAPI specs
└── .github/workflows/      # CI/CD pipelines
```

### Key Components

#### 1. API Server (`internal/api/`)

The HTTP server handles all incoming requests:

```go
// server.go - Creates the server with all middleware
func NewServer(runner Runner, claude Claude, ...) *Server

// handlers.go - Request handlers
func (s *Server) runClaude(c *gin.Context)      // POST /run
func (s *Server) runClaudeAsync(c *gin.Context) // POST /run/async
func (s *Server) streamClaude(c *gin.Context)   // GET /run/stream
```

#### 2. Container Runner (`internal/runner/`)

Manages Podman container lifecycle:

```go
// runner.go - Main interface
type Runner interface {
    Run(ctx context.Context, req RunRequest) (*RunResponse, error)
}

// PodmanRunner - Implementation
type PodmanRunner struct {
    image           string
    credentialsFile string
    executor        Executor
    // ...
}
```

#### 3. Job Manager (`internal/job/`)

Handles async job execution:

```go
// manager.go
type Manager struct {
    jobs map[string]*Job
    mu   sync.RWMutex
}

func (m *Manager) Create(ctx context.Context, req Request) *Job
func (m *Manager) Get(id string) (*Job, bool)
func (m *Manager) StartCleanup(ttl, interval time.Duration)
```

### Request Flow

```
                    ┌─────────────────┐
                    │  HTTP Request   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │    Gin Router   │
                    └────────┬────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
     ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
     │  Recovery   │  │    Auth     │  │ Rate Limit  │
     │ Middleware  │  │ Middleware  │  │ Middleware  │
     └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
            │                │                │
            └────────────────┼────────────────┘
                             │
                    ┌────────▼────────┐
                    │    Handler      │
                    │  (runClaude)    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Validate &     │
                    │  Parse Request  │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  PodmanRunner   │
                    │     .Run()      │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │ Build Podman    │
                    │    Command      │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │   Execute       │
                    │   Container     │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Parse Output   │
                    │  Return JSON    │
                    └─────────────────┘
```

---

## Code Style

### Go Guidelines

Follow [Effective Go](https://golang.org/doc/effective_go) and these conventions:

#### Naming

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase, short | `runner`, `auth` |
| File | lowercase, underscores | `runner_test.go` |
| Exported | PascalCase | `RunRequest` |
| Unexported | camelCase | `buildCommand` |
| Constant | PascalCase or camelCase | `DefaultTimeout` |
| Interface | `-er` suffix when possible | `Executor`, `Runner` |

#### Error Handling

```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create container: %w", err)
}

// Use sentinel errors for domain errors
var ErrImageNotAllowed = errors.New("image not in allowed patterns")

// Check with errors.Is
if errors.Is(err, ErrImageNotAllowed) {
    // handle specifically
}
```

#### Interfaces

```go
// Define interfaces where used (not where implemented)
// Keep interfaces small (1-3 methods)
type Executor interface {
    Run(ctx context.Context, args []string) ([]byte, error)
}
```

#### Context

```go
// Always pass context as first parameter
func (r *Runner) Run(ctx context.Context, req Request) (*Response, error)

// Respect context cancellation
select {
case <-ctx.Done():
    return nil, ctx.Err()
case result := <-ch:
    return result, nil
}
```

### Formatting

```bash
# Auto-format code
go fmt ./...

# Run linter
make lint
# or
golangci-lint run
```

---

## Making Changes

### 1. Create a Branch

```bash
# Feature
git checkout -b feat/my-feature

# Bug fix
git checkout -b fix/my-bugfix

# Documentation
git checkout -b docs/update-readme
```

### 2. Write Tests First (TDD)

```bash
# 1. Write failing test
go test ./internal/runner -run TestMyFeature  # FAIL

# 2. Implement feature
# ... write code ...

# 3. Test passes
go test ./internal/runner -run TestMyFeature  # PASS

# 4. Refactor if needed
```

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run specific package
go test ./internal/runner/...

# Run with verbose output
go test -v ./internal/runner -run TestMyFeature

# Run with race detection
go test -race ./...
```

### 4. Commit with Conventional Commits

```bash
# Format: type(scope): description
git commit -m "feat(runner): add image validation"
git commit -m "fix(auth): handle expired tokens correctly"
git commit -m "docs: update API documentation"
git commit -m "test(runner): add tests for timeout handling"
git commit -m "refactor(api): simplify handler logic"
git commit -m "ci: add integration test workflow"
```

Types:
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation only
- `test` - Adding tests
- `refactor` - Code change that neither fixes a bug nor adds a feature
- `ci` - CI/CD changes
- `chore` - Other changes (build, deps, etc.)

### 5. Create Pull Request

```bash
# Push branch
git push origin feat/my-feature

# Create PR
gh pr create --fill
```

---

## Testing

### Test Organization

```
internal/runner/
├── runner.go           # Implementation
├── runner_test.go      # Unit tests
└── integration_test.go # Integration tests (build tag)
```

### Unit Tests

```go
// runner_test.go
func TestPodmanRunner_Run(t *testing.T) {
    // Setup mock executor
    mockExec := NewMockExecutor()
    mockExec.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
        return []byte(`{"output": "test"}`), nil
    }

    runner := &PodmanRunner{
        executor: mockExec,
        image:    "test-image",
    }

    // Test
    result, err := runner.Run(context.Background(), RunRequest{
        Prompt: "test",
    })

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "test", result.Output)
}
```

### Table-Driven Tests

```go
func TestImageValidator_IsAllowed(t *testing.T) {
    tests := []struct {
        name     string
        patterns []string
        image    string
        want     bool
    }{
        {
            name:     "exact match",
            patterns: []string{"python:3.12"},
            image:    "python:3.12",
            want:     true,
        },
        {
            name:     "wildcard match",
            patterns: []string{"python:*"},
            image:    "python:3.12",
            want:     true,
        },
        {
            name:     "no match",
            patterns: []string{"python:*"},
            image:    "node:20",
            want:     false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            v := NewImageValidator(tt.patterns, "default")
            got := v.IsAllowed(tt.image)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Integration Tests

```go
// integration_test.go
//go:build integration

package runner_test

import (
    "context"
    "testing"

    "stromboli/internal/runner"
)

func TestPodmanRunner_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    r, err := runner.NewPodmanRunner("python:3.12")
    require.NoError(t, err)

    result, err := r.Run(context.Background(), runner.RunRequest{
        Prompt: "print('hello')",
    })

    require.NoError(t, err)
    assert.Contains(t, result.Output, "hello")
}
```

Run integration tests:
```bash
go test -tags=integration ./...
```

### Mock Helpers

```go
// Use the provided mock executor
mockExec := runner.NewMockExecutor()

// Configure response
mockExec.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
    // Check args
    if args[0] == "run" {
        return []byte("success"), nil
    }
    return nil, errors.New("unknown command")
}

// Or return error
mockExec.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
    return nil, errors.New("podman not available")
}
```

---

## Documentation

### Code Comments

```go
// Good: Explains WHY
// RetryWithBackoff handles transient network failures common
// when Podman socket is temporarily unavailable during high load.
func RetryWithBackoff(...)

// Bad: Explains WHAT (obvious from code)
// This function retries the operation
func RetryWithBackoff(...)
```

### Godoc

All exported functions need documentation:

```go
// Run executes Claude in an isolated Podman container.
//
// The container is created with the specified image, resource limits,
// and workspace mounts. Output is captured and returned when complete.
//
// If the context is cancelled, the container is stopped and removed.
func (r *PodmanRunner) Run(ctx context.Context, req RunRequest) (*RunResponse, error) {
```

### API Documentation

Update Swagger annotations for API changes:

```go
// @Summary Run Claude synchronously
// @Description Executes Claude in an isolated container and returns the result
// @Tags execution
// @Accept json
// @Produce json
// @Param request body RunRequest true "Run request"
// @Success 200 {object} RunResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /run [post]
func (s *Server) runClaude(c *gin.Context) {
```

Regenerate docs:
```bash
make swagger
```

### MkDocs Documentation

Documentation lives in `docs/`:

```bash
# Preview locally
pip install mkdocs-material
mkdocs serve

# Build
mkdocs build
```

---

## Pull Request Guidelines

### Before Submitting

- [ ] Tests pass: `make test`
- [ ] Linter passes: `make lint`
- [ ] New code has tests
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] No unrelated changes included

### PR Title Format

```
feat(runner): add image validation
fix(auth): handle expired JWT tokens
docs: update security guide
```

### PR Description Template

```markdown
## Summary
Brief description of what this PR does.

## Changes
- Added image validation to runner
- Updated configuration docs
- Fixed timeout handling in async jobs

## Test Plan
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Screenshots (if applicable)

## Related Issues
Closes #123
```

### Review Process

1. **Automated checks** must pass (CI, linting, tests)
2. **Code review** by maintainer
3. **Approval** required before merge
4. **Squash merge** to keep history clean

---

## Release Process

Releases are automated via GitHub Actions.

### Creating a Release

```bash
# Create and push tag
git tag v0.2.0-alpha
git push origin v0.2.0-alpha
```

This triggers:
1. **Binary builds** for Linux, macOS, Windows (amd64, arm64)
2. **Docker image** pushed to `ghcr.io/tomblancdev/stromboli`
3. **GitHub Release** created with binaries
4. **Documentation** deployed to GitHub Pages

### Version Numbering

```
v{major}.{minor}.{patch}[-{prerelease}]

v0.1.0       # First stable minor
v0.2.0-alpha # Alpha release
v1.0.0       # First major stable
```

---

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/tomblancdev/stromboli/discussions)
- **Bugs**: [GitHub Issues](https://github.com/tomblancdev/stromboli/issues)
- **Security**: See [Security Guide](../guide/security.md)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
