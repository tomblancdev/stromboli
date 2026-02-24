# Testing

## Running tests

```bash
make test              # all tests
make test-unit         # unit only
make test-integration  # requires Podman
make test-coverage     # with HTML report

# Specific package
go test ./internal/runner/...

# Single test
go test -v -run TestValidateSecretsEnv ./internal/runner/...
```

## Test structure

Tests live next to their code:

```
internal/runner/runner.go
internal/runner/runner_test.go
```

Naming convention: `TestFunctionName_Scenario`:

```go
func TestValidateSecretsEnv_ValidCases(t *testing.T) { }
func TestValidateSecretsEnv_InvalidEnvVarNames(t *testing.T) { }
func TestListSecrets_PodmanError(t *testing.T) { }
```

## Mock executor

Use `MockExecutor` to test without Podman:

```go
func TestRunClaude(t *testing.T) {
    mock := runner.NewMockExecutor()
    mock.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
        return []byte("Claude output"), nil
    }

    r, _ := runner.NewPodmanRunnerWithExecutor("image", "/secrets", "/sessions", []string{"/allowed"}, mock)
    result, err := r.Run(ctx, request)

    require.NoError(t, err)
    assert.Equal(t, "Claude output", result.Output)
}
```

## Table-driven tests

```go
func TestValidateSecretsEnv(t *testing.T) {
    tests := []struct {
        name       string
        secretsEnv map[string]string
        wantErr    bool
        errContain string
    }{
        {"valid", map[string]string{"GH_TOKEN": "github-token"}, false, ""},
        {"invalid name", map[string]string{"GH-TOKEN": "s"}, true, "invalid environment variable"},
        {"dangerous var", map[string]string{"LD_PRELOAD": "x"}, true, "not allowed"},
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

## HTTP handler tests

```go
func TestHealthCheck(t *testing.T) {
    server := newTestServer(t, nil, false)
    req, _ := http.NewRequest(http.MethodGet, "/health", nil)
    rec := httptest.NewRecorder()
    server.router.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)

    var resp map[string]string
    json.Unmarshal(rec.Body.Bytes(), &resp)
    assert.Equal(t, "ok", resp["status"])
}
```

## Integration tests

Use build tags to separate from unit tests:

```go
//go:build integration

func TestPodmanRunner_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // real Podman tests...
}
```

```bash
PODMAN_AVAILABLE=true go test -tags=integration ./...
```

## Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out
```

Targets:

| Package | Target |
|---|---|
| `internal/api` | 80% |
| `internal/runner` | 85% |
| `internal/secrets` | 90% |
| `internal/auth` | 90% |

## Debugging

```bash
# Verbose output
go test -v ./internal/runner/... 2>&1 | tee test.log

# With race detection
go test -race ./...

# With delve
dlv test ./internal/runner/ -- -test.run TestMyFunction
```
