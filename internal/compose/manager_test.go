package compose

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	strerrors "stromboli/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockExecutor implements Executor for testing
type MockExecutor struct {
	mu sync.Mutex

	// Calls tracks all command invocations
	Calls [][]string

	// RunFunc is called when Run is invoked, if set
	RunFunc func(ctx context.Context, args []string) ([]byte, error)

	// RunStreamFunc is called when RunStream is invoked, if set
	RunStreamFunc func(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)

	// DefaultOutput is returned if RunFunc is not set
	DefaultOutput []byte

	// DefaultError is returned if RunFunc is not set
	DefaultError error
}

// NewMockExecutor creates a new MockExecutor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Calls: make([][]string, 0),
	}
}

// Run executes the mock command
func (m *MockExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, args)
	m.mu.Unlock()

	if m.RunFunc != nil {
		return m.RunFunc(ctx, args)
	}

	return m.DefaultOutput, m.DefaultError
}

// RunStream executes the mock command with streaming
func (m *MockExecutor) RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, args)
	m.mu.Unlock()

	if m.RunStreamFunc != nil {
		return m.RunStreamFunc(ctx, args)
	}

	// Return empty streams
	stdoutReader := io.NopCloser(strings.NewReader(""))
	stderrReader := io.NopCloser(strings.NewReader(""))

	return stdoutReader, stderrReader, func() error { return nil }, func() error { return nil }, nil
}

// GetCalls returns a copy of all recorded command calls
// Returns a copy to prevent race conditions when iterating
func (m *MockExecutor) GetCalls() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return a copy to prevent race conditions
	calls := make([][]string, len(m.Calls))
	for i, call := range m.Calls {
		calls[i] = make([]string, len(call))
		copy(calls[i], call)
	}
	return calls
}

// Reset clears all recorded calls
func (m *MockExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = make([][]string, 0)
}

// Helper to create a valid compose file
func createTestComposeFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)
	return composePath
}

func TestManager_Up_Success(t *testing.T) {
	composePath := createTestComposeFile(t, `services:
  dev:
    image: python:3.12
  db:
    image: postgres:15
`)

	executor := NewMockExecutor()
	// Return healthy status for health check
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if containsArg(args, "ps") {
			return []byte(`[{"Name":"dev-1","Service":"dev","State":"running","Health":""},{"Name":"db-1","Service":"db","State":"running","Health":"healthy"}]`), nil
		}
		return []byte{}, nil
	}

	mgr := NewManager(executor, Config{
		BuildTimeout:  5 * time.Minute,
		HealthTimeout: 30 * time.Second,
	})

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "dev",
	}

	stack, err := mgr.Up(context.Background(), env, "test-session-123")
	require.NoError(t, err)
	assert.NotNil(t, stack)
	assert.Equal(t, "stromboli-test-session-123", stack.ProjectName)
	assert.Equal(t, "dev", stack.Service)
	assert.Equal(t, composePath, stack.ComposePath)

	// Verify up command was called
	calls := executor.GetCalls()
	found := false
	for _, call := range calls {
		if containsArg(call, "up") && containsArg(call, "-d") && containsArg(call, "--build") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected up command with -d and --build flags")

	// Verify stack is tracked
	assert.True(t, mgr.IsActive("test-session-123"))
}

func TestManager_Up_ValidationFails(t *testing.T) {
	// Create compose file with privileged service
	composePath := createTestComposeFile(t, `services:
  dev:
    image: python:3.12
    privileged: true
`)

	executor := NewMockExecutor()
	mgr := NewManager(executor, Config{
		AllowPrivileged: false, // Block privileged
		BuildTimeout:    5 * time.Minute,
		HealthTimeout:   30 * time.Second,
	})

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "dev",
	}

	stack, err := mgr.Up(context.Background(), env, "test-session")
	assert.Error(t, err)
	assert.Nil(t, stack)
	assert.True(t, errors.Is(err, ErrPrivilegedNotAllowed) || strings.Contains(err.Error(), "privileged"))

	// Verify no commands were executed
	assert.Empty(t, executor.GetCalls())
}

func TestManager_Up_ServiceNotFound(t *testing.T) {
	composePath := createTestComposeFile(t, `services:
  web:
    image: nginx
`)

	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "nonexistent",
	}

	stack, err := mgr.Up(context.Background(), env, "test-session")
	assert.Error(t, err)
	assert.Nil(t, stack)
	assert.True(t, errors.Is(err, ErrServiceNotFound) || strings.Contains(err.Error(), "not found"))
}

func TestManager_Down_Success(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// Pre-populate the stack
	mgr.mu.Lock()
	mgr.stacks["test-session"] = &Stack{
		ProjectName: "stromboli-test-session",
		SessionID:   "test-session",
	}
	mgr.mu.Unlock()

	err := mgr.Down(context.Background(), "test-session")
	require.NoError(t, err)

	// Verify down command was called
	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	assert.True(t, containsArg(calls[0], "down"))
	assert.True(t, containsArg(calls[0], "-v"))
	assert.True(t, containsArg(calls[0], "-p"))
	assert.True(t, containsArg(calls[0], "stromboli-test-session"))

	// Verify stack is no longer tracked
	assert.False(t, mgr.IsActive("test-session"))
}

func TestManager_Down_Idempotent(t *testing.T) {
	executor := NewMockExecutor()
	// Simulate "not found" error
	executor.DefaultError = errors.New("no such project: stromboli-test-session")

	mgr := NewManager(executor, DefaultConfig())

	// Should succeed even when stack doesn't exist
	err := mgr.Down(context.Background(), "test-session")
	assert.NoError(t, err)
}

func TestManager_Exec(t *testing.T) {
	executor := NewMockExecutor()
	executor.DefaultOutput = []byte("hello world")

	mgr := NewManager(executor, DefaultConfig())

	// Register a stack first (required for Exec to work)
	mgr.mu.Lock()
	mgr.stacks["test-session"] = &Stack{
		SessionID: "test-session",
		Service:   "dev",
		State:     StackStateRunning,
	}
	mgr.mu.Unlock()

	output, err := mgr.Exec(context.Background(), "test-session", "dev", []string{"echo", "hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(output))

	// Verify exec command was called
	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	assert.True(t, containsArg(calls[0], "exec"))
	assert.True(t, containsArg(calls[0], "-T"))
	assert.True(t, containsArg(calls[0], "dev"))
	assert.True(t, containsArg(calls[0], "echo"))
	assert.True(t, containsArg(calls[0], "hello"))
}

func TestManager_Exec_ServiceValidation(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// Register a stack with service "dev"
	mgr.mu.Lock()
	mgr.stacks["test-session"] = &Stack{
		SessionID: "test-session",
		Service:   "dev",
		State:     StackStateRunning,
	}
	mgr.mu.Unlock()

	// Try to exec into a different service - should fail
	_, err := mgr.Exec(context.Background(), "test-session", "db", []string{"echo", "hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
	assert.Contains(t, err.Error(), "dev")

	// Stack not found
	_, err = mgr.Exec(context.Background(), "nonexistent-session", "dev", []string{"echo", "hello"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_CleanupOrphanedStacks(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// Add an old stack and a new stack (both must be in Running state)
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now()

	mgr.mu.Lock()
	mgr.stacks["old-session"] = &Stack{
		SessionID: "old-session",
		StartedAt: oldTime,
		State:     StackStateRunning,
	}
	mgr.stacks["new-session"] = &Stack{
		SessionID: "new-session",
		StartedAt: newTime,
		State:     StackStateRunning,
	}
	mgr.mu.Unlock()

	// Cleanup stacks older than 1 hour
	err := mgr.CleanupOrphanedStacks(context.Background(), 1*time.Hour)
	require.NoError(t, err)

	// Old stack should be removed, new stack should remain
	assert.False(t, mgr.IsActive("old-session"))
	assert.True(t, mgr.IsActive("new-session"))
}

func TestManager_ListStacks(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// Add some stacks
	mgr.mu.Lock()
	mgr.stacks["session-1"] = &Stack{SessionID: "session-1"}
	mgr.stacks["session-2"] = &Stack{SessionID: "session-2"}
	mgr.mu.Unlock()

	stacks := mgr.ListStacks()
	assert.Len(t, stacks, 2)
}

func TestManager_ExecStream(t *testing.T) {
	executor := NewMockExecutor()
	outputContent := "streaming output"
	executor.RunStreamFunc = func(ctx context.Context, args []string) (io.ReadCloser, io.ReadCloser, func() error, func() error, error) {
		stdout := io.NopCloser(strings.NewReader(outputContent))
		stderr := io.NopCloser(strings.NewReader(""))
		return stdout, stderr, func() error { return nil }, func() error { return nil }, nil
	}

	mgr := NewManager(executor, DefaultConfig())

	// Register a stack first (required for ExecStream to work)
	mgr.mu.Lock()
	mgr.stacks["test-session"] = &Stack{
		SessionID: "test-session",
		Service:   "dev",
		State:     StackStateRunning,
	}
	mgr.mu.Unlock()

	stdout, stderr, start, wait, err := mgr.ExecStream(context.Background(), "test-session", "dev", []string{"claude", "-p", "hello"})
	require.NoError(t, err)

	err = start()
	require.NoError(t, err)

	// Read stdout
	data, err := io.ReadAll(stdout)
	require.NoError(t, err)
	assert.Equal(t, outputContent, string(data))

	// Read stderr (empty)
	data, err = io.ReadAll(stderr)
	require.NoError(t, err)
	assert.Empty(t, data)

	err = wait()
	require.NoError(t, err)
}

func TestManager_Up_HealthCheckTimeout(t *testing.T) {
	composePath := createTestComposeFile(t, `services:
  dev:
    image: python:3.12
`)

	executor := NewMockExecutor()
	callCount := 0
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if containsArg(args, "ps") {
			callCount++
			// Always return unhealthy
			return []byte(`[{"Name":"dev-1","Service":"dev","State":"starting","Health":"starting"}]`), nil
		}
		return []byte{}, nil
	}

	mgr := NewManager(executor, Config{
		BuildTimeout:  5 * time.Minute,
		HealthTimeout: 100 * time.Millisecond, // Very short timeout for test
	})

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "dev",
	}

	stack, err := mgr.Up(context.Background(), env, "test-session")
	assert.Error(t, err)
	assert.Nil(t, stack)
	assert.True(t, errors.Is(err, strerrors.ErrComposeHealthTimeout) || strings.Contains(err.Error(), "healthy"))

	// Should have been polled multiple times
	assert.GreaterOrEqual(t, callCount, 1)

	// Stack should not be tracked
	assert.False(t, mgr.IsActive("test-session"))
}

func TestManager_DiscoverOrphanedStacks(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		trackedSession string
		wantOrphaned   []string
	}{
		{
			name:           "empty output",
			output:         "",
			trackedSession: "",
			wantOrphaned:   nil,
		},
		{
			name:           "no stromboli stacks",
			output:         `[{"Name":"myapp","Status":"running"}]`,
			trackedSession: "",
			wantOrphaned:   nil,
		},
		{
			name:           "orphaned stromboli stack",
			output:         `[{"Name":"stromboli-orphan-123","Status":"running"}]`,
			trackedSession: "",
			wantOrphaned:   []string{"orphan-123"},
		},
		{
			name:           "tracked stromboli stack",
			output:         `[{"Name":"stromboli-tracked-456","Status":"running"}]`,
			trackedSession: "tracked-456",
			wantOrphaned:   nil,
		},
		{
			name:           "mixed stacks",
			output:         `[{"Name":"stromboli-orphan-1","Status":"running"},{"Name":"stromboli-tracked-2","Status":"running"},{"Name":"other-app","Status":"running"}]`,
			trackedSession: "tracked-2",
			wantOrphaned:   []string{"orphan-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewMockExecutor()
			executor.DefaultOutput = []byte(tt.output)

			mgr := NewManager(executor, DefaultConfig())

			// Track a session if specified
			if tt.trackedSession != "" {
				mgr.mu.Lock()
				mgr.stacks[tt.trackedSession] = &Stack{SessionID: tt.trackedSession}
				mgr.mu.Unlock()
			}

			orphaned, err := mgr.DiscoverOrphanedStacks(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrphaned, orphaned)
		})
	}
}

func TestManager_DiscoverOrphanedStacks_InvalidJSON(t *testing.T) {
	executor := NewMockExecutor()
	executor.DefaultOutput = []byte("not valid json")

	mgr := NewManager(executor, DefaultConfig())

	_, err := mgr.DiscoverOrphanedStacks(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestManager_BuildTimeoutError(t *testing.T) {
	composePath := createTestComposeFile(t, `services:
  dev:
    image: python:3.12
`)

	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	env := Environment{
		Type:         "compose",
		Path:         composePath,
		Service:      "dev",
		BuildTimeout: "invalid-duration",
	}

	_, err := mgr.Up(context.Background(), env, "test-session")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid build timeout")
	assert.Contains(t, err.Error(), "invalid-duration") // Error should include the bad value
}

func TestManager_ConcurrentGetStack(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// Add a test stack
	mgr.mu.Lock()
	mgr.stacks["test-session"] = &Stack{
		SessionID: "test-session",
		State:     StackStateRunning,
	}
	mgr.mu.Unlock()

	// Concurrent reads should not race
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stack := mgr.GetStack("test-session")
			if stack != nil {
				_ = stack.IsReady()
			}
		}()
	}
	wg.Wait()
}

func TestManager_IsReady(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	// No stack
	assert.False(t, mgr.IsReady("nonexistent"))

	// Stack in starting state
	mgr.mu.Lock()
	mgr.stacks["starting"] = &Stack{SessionID: "starting", State: StackStateStarting}
	mgr.mu.Unlock()
	assert.False(t, mgr.IsReady("starting"))

	// Stack in running state
	mgr.mu.Lock()
	mgr.stacks["running"] = &Stack{SessionID: "running", State: StackStateRunning}
	mgr.mu.Unlock()
	assert.True(t, mgr.IsReady("running"))

	// Stack in stopping state
	mgr.mu.Lock()
	mgr.stacks["stopping"] = &Stack{SessionID: "stopping", State: StackStateStopping}
	mgr.mu.Unlock()
	assert.False(t, mgr.IsReady("stopping"))
}

func TestManager_StackState_Lifecycle(t *testing.T) {
	composePath := createTestComposeFile(t, `services:
  dev:
    image: python:3.12
`)

	executor := NewMockExecutor()
	healthyResponse := []byte(`[{"Name":"dev-1","Service":"dev","State":"running","Health":""}]`)
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if containsArg(args, "ps") {
			return healthyResponse, nil
		}
		return []byte{}, nil
	}

	mgr := NewManager(executor, Config{
		BuildTimeout:  5 * time.Minute,
		HealthTimeout: 30 * time.Second,
	})

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "dev",
	}

	// Start the stack
	stack, err := mgr.Up(context.Background(), env, "lifecycle-test")
	require.NoError(t, err)

	// Stack should be running
	assert.Equal(t, StackStateRunning, stack.State)
	assert.True(t, mgr.IsReady("lifecycle-test"))

	// Stop the stack
	err = mgr.Down(context.Background(), "lifecycle-test")
	require.NoError(t, err)

	// Stack should be gone
	assert.False(t, mgr.IsActive("lifecycle-test"))
}

func TestManager_CleanupOrphanedStacks_OnlyRunningStacks(t *testing.T) {
	executor := NewMockExecutor()
	mgr := NewManager(executor, DefaultConfig())

	now := time.Now()
	oldTime := now.Add(-2 * time.Hour)

	// Add stacks in various states
	mgr.mu.Lock()
	mgr.stacks["old-running"] = &Stack{SessionID: "old-running", StartedAt: oldTime, State: StackStateRunning}
	mgr.stacks["old-starting"] = &Stack{SessionID: "old-starting", StartedAt: oldTime, State: StackStateStarting}
	mgr.stacks["old-stopping"] = &Stack{SessionID: "old-stopping", StartedAt: oldTime, State: StackStateStopping}
	mgr.stacks["new-running"] = &Stack{SessionID: "new-running", StartedAt: now, State: StackStateRunning}
	mgr.mu.Unlock()

	// Cleanup stacks older than 1 hour - should only clean running stacks
	err := mgr.CleanupOrphanedStacks(context.Background(), 1*time.Hour)
	require.NoError(t, err)

	// Old running stack should be cleaned up
	assert.False(t, mgr.IsActive("old-running"))
	// Starting and stopping should be skipped (not cleaned)
	assert.True(t, mgr.IsActive("old-starting"))
	assert.True(t, mgr.IsActive("old-stopping"))
	// New running should remain
	assert.True(t, mgr.IsActive("new-running"))
}

// Helper function to check if args contain a specific argument
func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}
