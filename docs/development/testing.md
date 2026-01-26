# Testing

Guide to testing Stromboli.

## Running Tests

```bash
# All tests
make test

# Specific package
go test ./internal/runner/...

# With verbose output
go test -v ./...

# With coverage
make test-coverage
```

## Test Structure

### Unit Tests

Located next to the code:
```
internal/runner/runner.go
internal/runner/runner_test.go
```

### Test Naming

```go
func TestFunctionName_Scenario(t *testing.T) {
    // ...
}

func TestFunctionName_Scenario_SubScenario(t *testing.T) {
    // ...
}
```

Examples:
```go
func TestValidateSecretsEnv_ValidCases(t *testing.T) { }
func TestValidateSecretsEnv_InvalidEnvVarNames(t *testing.T) { }
func TestListSecrets_PodmanError(t *testing.T) { }
```

## Mock Executor

Use `MockExecutor` to test without Podman:

```go
func TestRunClaude(t *testing.T) {
    mockExecutor := runner.NewMockExecutor()

    // Configure mock responses
    mockExecutor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
        if args[1] == "run" {
            return []byte("Claude output"), nil
        }
        return nil, errors.New("unknown command")
    }

    // Create runner with mock
    runner, err := runner.NewPodmanRunnerWithExecutor(
        "image",
        "/path/to/secrets",
        "/path/to/sessions",
        []string{"/allowed"},
        mockExecutor,
    )
    require.NoError(t, err)

    // Test
    result, err := runner.Run(ctx, request)
    assert.NoError(t, err)
    assert.Equal(t, "Claude output", result.Output)
}
```

## Table-Driven Tests

Use table-driven tests for comprehensive coverage:

```go
func TestValidateSecretsEnv(t *testing.T) {
    tests := []struct {
        name       string
        secretsEnv map[string]string
        wantErr    bool
        errContain string
    }{
        {
            name:       "valid single secret",
            secretsEnv: map[string]string{"GH_TOKEN": "github-token"},
            wantErr:    false,
        },
        {
            name:       "invalid env var name",
            secretsEnv: map[string]string{"GH-TOKEN": "secret"},
            wantErr:    true,
            errContain: "invalid environment variable name",
        },
        {
            name:       "dangerous env var",
            secretsEnv: map[string]string{"LD_PRELOAD": "malicious"},
            wantErr:    true,
            errContain: "not allowed",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateSecretsEnv(tt.secretsEnv)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContain)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## HTTP Handler Tests

Use `httptest` for API tests:

```go
func TestHealthCheck(t *testing.T) {
    server := newTestServer(t, nil, false)

    req, err := http.NewRequest(http.MethodGet, "/health", nil)
    require.NoError(t, err)

    rec := httptest.NewRecorder()
    server.router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)

    var response map[string]string
    err = json.Unmarshal(rec.Body.Bytes(), &response)
    require.NoError(t, err)

    assert.Equal(t, "ok", response["status"])
}
```

## Test Helpers

### Temporary Directories

```go
func TestWithTempDir(t *testing.T) {
    tmpDir := t.TempDir()  // Cleaned up automatically

    credFile := filepath.Join(tmpDir, ".credentials.json")
    require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0600))

    // Use credFile...
}
```

### Test Fixtures

```go
func loadTestFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", name))
    require.NoError(t, err)
    return data
}
```

## Integration Tests

Build tag for integration tests:

```go
//go:build integration

package runner

import "testing"

func TestRunnerIntegration(t *testing.T) {
    if os.Getenv("PODMAN_AVAILABLE") != "true" {
        t.Skip("Podman not available")
    }
    // Real Podman tests...
}
```

Run:
```bash
PODMAN_AVAILABLE=true go test -tags=integration ./...
```

## Coverage

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### View Coverage

```bash
go tool cover -func=coverage.out
```

Output:
```
stromboli/internal/runner/runner.go:132:    Run             85.0%
stromboli/internal/runner/validation.go:24: ValidateSecretsEnv  100.0%
total:                                      (statements)    82.3%
```

### Coverage Targets

| Package | Target |
|---------|--------|
| `internal/api` | 80% |
| `internal/runner` | 85% |
| `internal/secrets` | 90% |
| `internal/auth` | 90% |

## Continuous Integration

Tests run automatically on PR:

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: make test
      - run: make lint
```

## Debugging Tests

### Verbose Output

```bash
go test -v ./internal/runner/... 2>&1 | tee test.log
```

### Run Single Test

```bash
go test -v -run TestValidateSecretsEnv_DangerousEnvVars ./internal/runner/...
```

### Debug with Delve

```bash
dlv test ./internal/runner/ -- -test.run TestMyFunction
```
