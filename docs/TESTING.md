# Testing Guide

This document describes how to run tests in Stromboli and the difference between unit and integration tests.

## Test Types

### Unit Tests

Unit tests are fast, isolated tests that don't require external dependencies like Podman. They test individual functions and logic in isolation.

**Run unit tests:**
```bash
make test
# or
go test -v ./...
```

**What's included:**
- Pure logic tests
- Struct initialization tests
- Helper function tests
- ID generation tests

**When to use:** During rapid development cycles, in CI/CD pipelines, or when Podman is not available.

---

### Integration Tests

Integration tests require Podman to be installed and running. They test real container operations, Podman API interactions, and secrets management.

**Run integration tests:**
```bash
make test-integration
# or
go test -v -tags=integration ./...
```

**What's included:**
- Container lifecycle tests (create, run, stop, remove)
- Podman secrets operations
- Runner execution with real containers
- Full end-to-end flows

**Prerequisites:**
1. **Podman installed**: `podman --version` must succeed
2. **Podman running**: Podman socket must be accessible
3. **Permissions**: User must have permission to create containers and secrets

**When to use:** Before committing, during integration testing, or when validating Podman-specific functionality.

---

### Run All Tests

To run both unit and integration tests:

```bash
make test-all
# or
go test -v ./... && go test -v -tags=integration ./...
```

---

## Test Organization

### Files with Build Tags

Tests that require Podman have the `//go:build integration` tag and are only run when explicitly requested:

- `internal/container/container_test.go` - Container lifecycle tests
- `internal/secrets/secrets_test.go` - Secrets management tests
- `internal/runner/runner_test.go` - Runner execution tests

### Unit Test Files

These files contain unit tests that run by default:

- `internal/container/manager_test.go` - Manager initialization
- `internal/secrets/manager_test.go` - Secrets manager initialization
- `internal/runner/id_test.go` - ID generation tests
- All other `*_test.go` files without build tags

---

## Test Coverage

Generate test coverage reports:

```bash
make test-coverage
```

This creates two files:
- `coverage.out` - Raw coverage data
- `coverage.html` - HTML coverage report (open in browser)

**Target coverage:** 80%+ for all packages

---

## CI/CD Integration

### GitHub Actions Example

```yaml
# Unit tests (no Podman required)
- name: Run unit tests
  run: make test

# Integration tests (Podman required)
- name: Setup Podman
  run: |
    sudo apt-get update
    sudo apt-get install -y podman

- name: Run integration tests
  run: make test-integration
```

### GitLab CI Example

```yaml
test:unit:
  script:
    - make test

test:integration:
  image: quay.io/podman/stable
  script:
    - make test-integration
```

---

## Writing Tests

### Unit Test Example

```go
package mypackage

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMyFunction(t *testing.T) {
    result := MyFunction()
    assert.NotNil(t, result)
}
```

### Integration Test Example

```go
//go:build integration

package mypackage

import (
    "testing"
    "os/exec"
    "github.com/stretchr/testify/assert"
)

func skipIfNoPodman(t *testing.T) {
    if _, err := exec.LookPath("podman"); err != nil {
        t.Skip("podman not available, skipping test")
    }
}

func TestPodmanOperation(t *testing.T) {
    skipIfNoPodman(t)

    // Test code that uses Podman...
}
```

---

## Troubleshooting

### "podman not available, skipping test"

**Problem:** Integration tests are being skipped.

**Solution:** Install Podman:
```bash
# Debian/Ubuntu
sudo apt-get install podman

# Fedora/RHEL
sudo dnf install podman

# macOS
brew install podman
podman machine init
podman machine start
```

### Permission Denied Errors

**Problem:** Tests fail with permission errors when creating containers or secrets.

**Solution:** Add your user to the Podman group or run rootless Podman:
```bash
# Enable rootless Podman
systemctl --user enable --now podman.socket

# Or add user to group (requires logout/login)
sudo usermod -aG podman $USER
```

### Tests Hanging

**Problem:** Integration tests hang indefinitely.

**Solution:** Check Podman service status:
```bash
systemctl --user status podman.socket
podman system info
```

---

## Best Practices

1. **Always write unit tests first** - They're faster and catch logic errors early
2. **Add integration tests for Podman features** - Validate real container behavior
3. **Use `t.Cleanup()` or defer** - Ensure test resources are cleaned up
4. **Unique names** - Use unique container/secret names to avoid conflicts
5. **Test isolation** - Each test should be independent and not rely on order
6. **Skip gracefully** - Use `skipIfNoPodman()` helper in integration tests

---

## Quick Reference

| Command | Description |
|---------|-------------|
| `make test` | Run unit tests only (fast, no Podman) |
| `make test-integration` | Run integration tests (requires Podman) |
| `make test-all` | Run all tests (unit + integration) |
| `make test-coverage` | Generate coverage reports |
| `go test ./internal/container` | Test specific package |
| `go test -v -run TestCreate` | Run specific test |
| `go test -tags=integration -v ./...` | Manual integration test run |
