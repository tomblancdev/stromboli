# Contributing

Thank you for considering contributing to Stromboli!

## Development Setup

### Prerequisites

- Go 1.22+
- Podman 4.0+
- Make
- Docker (for running tests in containers)

### Clone & Build

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
make build
```

### Run Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# With coverage
make test-coverage
```

### Run Locally

```bash
# Start Stromboli
make run

# Or with hot reload
make dev
```

## Code Style

### Go Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Run `make lint` before committing

### Package Structure

```
stromboli/
├── cmd/stromboli/       # Entry point only
├── internal/            # Private code
│   ├── api/            # HTTP handlers
│   ├── runner/         # Container execution
│   ├── secrets/        # Credentials management
│   └── ...
├── pkg/                # Public libraries (if any)
└── docs/               # Documentation
```

### Naming Conventions

- **Packages**: lowercase, short (`runner`, not `runner_manager`)
- **Files**: lowercase with underscores (`agent_test.go`)
- **Exported**: PascalCase (`SpawnAgent`)
- **Unexported**: camelCase (`createContainer`)

## Making Changes

### 1. Create a Branch

```bash
git checkout -b feat/my-feature
# or
git checkout -b fix/my-bugfix
```

### 2. Write Tests First (TDD)

```bash
# Write failing test
make test  # Should fail

# Implement feature
make test  # Should pass
```

### 3. Commit with Conventional Commits

```bash
git commit -m "feat: add new feature"
git commit -m "fix: resolve bug in X"
git commit -m "docs: update API documentation"
git commit -m "test: add tests for Y"
git commit -m "refactor: simplify Z"
```

### 4. Create Pull Request

```bash
git push origin feat/my-feature
gh pr create
```

## Pull Request Guidelines

### Checklist

- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] New code has tests
- [ ] Documentation updated if needed
- [ ] Commit messages follow conventions

### PR Template

```markdown
## Summary
Brief description of changes.

## Changes
- Added X
- Fixed Y
- Updated Z

## Test Plan
- [ ] Unit tests pass
- [ ] Manual testing done

## Related Issues
Closes #123
```

## Testing

### Unit Tests

Place tests next to the code:
```
internal/runner/runner.go
internal/runner/runner_test.go
```

### Integration Tests

```go
// internal/runner/integration_test.go
//go:build integration

func TestRunnerIntegration(t *testing.T) {
    // ...
}
```

Run with:
```bash
go test -tags=integration ./...
```

### Test Helpers

Use the mock executor for testing:

```go
func TestMyFeature(t *testing.T) {
    mockExecutor := runner.NewMockExecutor()
    mockExecutor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
        return []byte("ok"), nil
    }
    // ...
}
```

## Documentation

### Code Comments

```go
// Good: Explains why
// RetryWithBackoff handles transient network failures common with Podman socket

// Bad: Explains what (obvious from code)
// This function retries the operation
```

### API Documentation

Update Swagger annotations when changing API:

```go
// @Summary Run Claude
// @Description Executes Claude in an isolated container
// @Tags execution
// @Accept json
// @Produce json
// @Param request body RunRequest true "Run request"
// @Success 200 {object} RunResponse
// @Router /run [post]
func (s *Server) runClaude(c *gin.Context) {
```

## Release Process

Releases are automated via GitHub Actions when a tag is pushed:

```bash
git tag v0.1.6-alpha
git push origin v0.1.6-alpha
```

This triggers:
1. Build binaries for all platforms
2. Build and push Docker images
3. Create GitHub release
4. Update documentation

## Getting Help

- [GitHub Issues](https://github.com/tomblancdev/stromboli/issues) - Bug reports
- [GitHub Discussions](https://github.com/tomblancdev/stromboli/discussions) - Questions

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
