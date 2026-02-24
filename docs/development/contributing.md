# Contributing

## Quick start

```bash
git clone https://github.com/tomblancdev/stromboli
cd stromboli
go mod download
make test
make build
./stromboli
```

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.24+ |
| Podman | 4.0+ |
| Make | Any |

## Workflow

1. Create a branch: `git checkout -b feat/my-feature`
2. Write a failing test
3. Write the minimal code to pass it
4. Run `make test && make lint`
5. Commit with [conventional commits](#commit-format)
6. Push and open a PR

## Commit format

```
type(scope): description

feat(runner): add image validation
fix(auth): handle expired tokens
test(runner): add timeout tests
docs: update security guide
refactor(api): simplify handler logic
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `ci`, `chore`

## Testing

```bash
make test              # all tests
make test-unit         # unit only
make test-integration  # requires Podman
make test-coverage     # with coverage report
```

Tests go next to the code: `runner.go` → `runner_test.go`. Use the `MockExecutor` for unit tests — see [testing guide](testing.md) for patterns.

## Code style

Follow [Effective Go](https://golang.org/doc/effective_go). Key conventions:

- Wrap errors with context: `fmt.Errorf("failed to create container: %w", err)`
- Small interfaces (1-3 methods), defined where used
- Context as first parameter
- `go fmt` and `golangci-lint` must pass

## Pull requests

Before submitting:

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] New code has tests
- [ ] Docs updated if needed

PR titles follow the same [commit format](#commit-format). Squash-merged to keep history clean.

## Releases

Tag and push to trigger the release pipeline:

```bash
git tag v0.3.0-alpha
git push origin v0.3.0-alpha
```

This builds binaries, pushes Docker images, creates a GitHub release, and deploys docs.

## Help

- [GitHub Issues](https://github.com/tomblancdev/stromboli/issues) — Bugs and feature requests
- [Architecture](architecture.md) — How the code is organized
- [Testing](testing.md) — Test patterns and helpers

MIT License — contributions are licensed under the same terms.
