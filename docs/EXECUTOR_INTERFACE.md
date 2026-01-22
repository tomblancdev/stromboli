# Executor Interface

> **Status**: Implemented (Roadmap Item #5)
> **Date**: January 22, 2026
> **Impact**: Enables unit testing of runner logic without Podman

## Overview

The Executor interface abstracts command execution in the runner package, allowing the PodmanRunner to be tested without requiring Podman to be installed. This separation of concerns makes the codebase more testable and maintainable.

## Architecture

### Interface Definition

```go
// Executor executes shell commands
type Executor interface {
    // Run executes a command and returns the combined output
    Run(ctx context.Context, args []string) ([]byte, error)

    // RunStream executes a command and returns pipes for streaming output
    RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
}
```

### Implementations

#### ShellExecutor (Production)

The `ShellExecutor` wraps Go's `os/exec` package and is used in production:

```go
executor := NewShellExecutor()
output, err := executor.Run(ctx, []string{"podman", "run", "..."})
```

**Features:**
- Direct execution via `exec.CommandContext`
- Respects context cancellation
- Handles both combined output (`Run`) and streaming (`RunStream`)

#### MockExecutor (Testing)

The `MockExecutor` provides a test double with configurable behavior:

```go
mock := NewMockExecutor()
mock.DefaultOutput = []byte("mocked output")
mock.DefaultError = nil

// Or use custom function
mock.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
    return []byte("custom response"), nil
}
```

**Features:**
- Records all command invocations
- Configurable output and errors
- Thread-safe for concurrent testing
- Supports both `Run` and `RunStream` methods

## Usage

### Production Code

By default, all constructors use `ShellExecutor`:

```go
// Standard constructor (uses ShellExecutor internally)
runner, err := NewPodmanRunner(image, secretsFile, sessionsDir, allowedWorkspaces)

// With defaults (uses ShellExecutor internally)
runner, err := NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir, allowedWorkspaces, defaults)
```

### Testing Code

For unit tests, inject a `MockExecutor`:

```go
// Create mock
mock := NewMockExecutor()
mock.DefaultOutput = []byte("test output")

// Create runner with mock
runner, err := NewPodmanRunnerWithExecutor(
    "test-image",
    secretsFile,
    sessionsDir,
    []string{},
    mock,
)

// Run tests
result, err := runner.Run(ctx, Request{Prompt: "test"})

// Verify calls
calls := mock.GetCalls()
assert.Len(t, calls, 1)
assert.Contains(t, calls[0], "podman")
```

## Benefits

### 1. Unit Testing Without Podman

Tests can now verify runner logic (command building, option handling, session management) without requiring Podman:

```go
func TestRunner_AppliesDefaults(t *testing.T) {
    mock := NewMockExecutor()
    runner, _ := NewPodmanRunnerWithExecutor("img", secrets, sessions, nil, mock)

    runner.Run(ctx, Request{Prompt: "test"})

    // Verify default limits were applied
    cmd := strings.Join(mock.GetCalls()[0], " ")
    assert.Contains(t, cmd, "--memory=512m")
}
```

### 2. Faster Test Execution

Unit tests run instantly without spawning containers:

- **Before**: 5-10s per test (Podman container overhead)
- **After**: <10ms per test (in-memory execution)

### 3. Better Test Coverage

Can now test edge cases that are difficult to reproduce with real containers:

- Context cancellation
- Command start failures
- Streaming errors
- Concurrent execution

### 4. Cleaner Architecture

Separates orchestration logic (PodmanRunner) from execution mechanism (Executor):

```
PodmanRunner (orchestration)
    ↓ uses
Executor (execution abstraction)
    ↓ implements
ShellExecutor (production) or MockExecutor (testing)
```

## Backward Compatibility

All existing code continues to work without changes:

- ✅ `NewPodmanRunner()` works as before
- ✅ `NewPodmanRunnerWithDefaults()` works as before
- ✅ All integration tests pass unchanged
- ✅ No changes needed to API handlers

## Files Modified

| File | Changes |
|------|---------|
| `internal/runner/executor.go` | New interface definition |
| `internal/runner/shell_executor.go` | Production implementation |
| `internal/runner/mock_executor.go` | Test implementation |
| `internal/runner/runner.go` | Updated to use Executor interface |
| `internal/runner/executor_test.go` | Tests for executor implementations |
| `internal/runner/runner_unit_test.go` | Unit tests using MockExecutor |

## Testing Strategy

### Unit Tests (No Podman Required)

Run these tests anywhere:

```bash
make test
```

Tests that use `MockExecutor`:
- Command building logic
- Default resource limit application
- Session management
- Option handling
- Error scenarios

### Integration Tests (Require Podman)

Run these tests in an environment with Podman:

```bash
make test-integration
```

Tests that use real Podman:
- Actual container execution
- Secret mounting
- Volume management
- Real Claude Code interaction

## Example Test Cases

### Testing Default Limits

```go
func TestAppliesDefaults(t *testing.T) {
    mock := NewMockExecutor()
    runner, _ := NewPodmanRunnerWithDefaultsAndExecutor(
        "img", secrets, sessions, nil,
        ResourceDefaults{Memory: "512m", CPUs: "1"},
        mock,
    )

    runner.Run(ctx, Request{Prompt: "test"})

    cmd := strings.Join(mock.GetCalls()[0], " ")
    assert.Contains(t, cmd, "--memory=512m")
    assert.Contains(t, cmd, "--cpus=1")
}
```

### Testing Error Handling

```go
func TestHandlesExecutionError(t *testing.T) {
    mock := NewMockExecutor()
    mock.DefaultError = errors.New("execution failed")

    runner, _ := NewPodmanRunnerWithExecutor("img", secrets, sessions, nil, mock)

    _, err := runner.Run(ctx, Request{Prompt: "test"})

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "execution failed")
}
```

### Testing Streaming

```go
func TestStreamsOutput(t *testing.T) {
    mock := NewMockExecutor()
    mock.StreamOutput = "line1\nline2\nline3"

    runner, _ := NewPodmanRunnerWithExecutor("img", secrets, sessions, nil, mock)

    output := make(chan string, 10)
    result, err := runner.RunStream(ctx, Request{Prompt: "test"}, output)

    require.NoError(t, err)

    var lines []string
    for line := range output {
        lines = append(lines, line)
    }

    assert.Len(t, lines, 3)
}
```

## Future Enhancements

Possible extensions to the Executor interface:

1. **Resource Monitoring**: Add methods to track CPU/memory usage
2. **Container Lifecycle**: Expose container ID for management
3. **Output Filtering**: Add hooks for processing output streams
4. **Metrics Collection**: Built-in execution time tracking
5. **Retry Logic**: Configurable retry strategies for transient failures

## Related Documentation

- [Testing Guide](TESTING.md) - Full testing strategy
- [Architecture](ARCHITECTURE.md) - System overview
- [Roadmap](ROADMAP.md) - Development progress
