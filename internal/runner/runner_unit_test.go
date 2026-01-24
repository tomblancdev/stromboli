package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/types"
)

// skipIfNoPodman skips the test if podman is not available
func skipIfNoPodman(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping test")
	}
}

// TestPodmanRunner_Run_WithMockExecutor tests runner logic without Podman
func TestPodmanRunner_Run_WithMockExecutor(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("mocked claude output")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	// Run a command
	result, err := runner.Run(context.Background(), Request{
		Prompt: "hello world",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "mocked claude output", result.Output)
	assert.NotEmpty(t, result.SessionID)

	// Verify the command was called
	calls := mock.GetCalls()
	assert.Len(t, calls, 1)
	assert.Contains(t, calls[0], "podman")
	assert.Contains(t, calls[0], "run")
}

// TestPodmanRunner_Run_WithExecutorError tests error handling
func TestPodmanRunner_Run_WithExecutorError(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor that fails
	mock := NewMockExecutor()
	mock.DefaultError = errors.New("execution failed")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	// Run should fail
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution failed")
}

// TestPodmanRunner_RunStream_WithMockExecutor tests streaming with mock
func TestPodmanRunner_RunStream_WithMockExecutor(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.StreamOutput = "line1\nline2\nline3"

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	// Run streaming
	output := make(chan string, 10)
	result, err := runner.RunStream(context.Background(), Request{
		Prompt: "test",
	}, output)

	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.SessionID)

	// Collect output lines
	var lines []string
	for line := range output {
		lines = append(lines, line)
	}

	assert.Len(t, lines, 3)
	assert.Contains(t, lines, "line1")
	assert.Contains(t, lines, "line2")
	assert.Contains(t, lines, "line3")
}

// TestPodmanRunner_Run_AppliesDefaults tests default resource limits are applied
func TestPodmanRunner_Run_AppliesDefaults(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}

	runner, err := NewPodmanRunnerWithDefaultsAndExecutor("test-image", secretsFile, sessionsDir, []string{}, defaults, mock)
	require.NoError(t, err)

	// Run without explicit limits
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			// No Memory, CPUs, or Timeout
		},
	})

	require.NoError(t, err)

	// Verify defaults were applied
	calls := mock.GetCalls()
	require.Len(t, calls, 1)

	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "--memory=512m")
	assert.Contains(t, cmdStr, "--cpus=1")
}

// TestPodmanRunner_Run_OverridesDefaults tests explicit values override defaults
func TestPodmanRunner_Run_OverridesDefaults(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}

	runner, err := NewPodmanRunnerWithDefaultsAndExecutor("test-image", secretsFile, sessionsDir, []string{}, defaults, mock)
	require.NoError(t, err)

	// Run with explicit limits
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Memory: "2g",
			CPUs:   "4",
		},
	})

	require.NoError(t, err)

	// Verify explicit values were used
	calls := mock.GetCalls()
	require.Len(t, calls, 1)

	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "--memory=2g")
	assert.Contains(t, cmdStr, "--cpus=4")
	assert.NotContains(t, cmdStr, "--memory=512m")
}

// TestPodmanRunner_Run_SessionHandling tests session creation and reuse
func TestPodmanRunner_Run_SessionHandling(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	// First run creates session
	result1, err := runner.Run(context.Background(), Request{
		Prompt: "test1",
	})
	require.NoError(t, err)
	sessionID1 := result1.SessionID

	// Second run with same session ID
	result2, err := runner.Run(context.Background(), Request{
		Prompt: "test2",
		Claude: types.ClaudeOptions{
			SessionID: sessionID1,
			Resume:    true,
		},
	})
	require.NoError(t, err)

	// Should use the same session
	assert.Equal(t, sessionID1, result2.SessionID)

	// Verify both commands used the same session
	calls := mock.GetCalls()
	assert.Len(t, calls, 2)

	// Both should reference the same session ID
	cmd1 := strings.Join(calls[0], " ")
	cmd2 := strings.Join(calls[1], " ")
	assert.Contains(t, cmd1, sessionID1)
	assert.Contains(t, cmd2, sessionID1)
	assert.Contains(t, cmd2, "--resume") // Second call should have resume
}

// TestPodmanRunner_Run_WorkspaceValidation tests workspace validation
func TestPodmanRunner_Run_WorkspaceValidation(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	workspaceDir := filepath.Join(tmpDir, "workspace")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(workspaceDir, 0755)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	// Only allow workspaceDir
	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{workspaceDir}, mock)
	require.NoError(t, err)

	// Valid workspace
	_, err = runner.Run(context.Background(), Request{
		Prompt:    "test",
		Workspace: workspaceDir,
	})
	require.NoError(t, err)

	// Invalid workspace
	_, err = runner.Run(context.Background(), Request{
		Prompt:    "test",
		Workspace: "/invalid/path",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace validation failed")
}

// TestPodmanRunner_RunStream_StartError tests handling of start errors
func TestPodmanRunner_RunStream_StartError(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor that fails to start
	mock := NewMockExecutor()
	mock.StreamStartError = errors.New("failed to start")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	output := make(chan string, 10)
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "test",
	}, output)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start")
}

// TestPodmanRunner_RunStream_WaitError tests handling of wait errors
func TestPodmanRunner_RunStream_WaitError(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor that fails during wait
	mock := NewMockExecutor()
	mock.StreamOutput = "some output"
	mock.StreamWaitError = errors.New("command failed")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	output := make(chan string, 10)
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "test",
	}, output)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "command failed")
}

// TestPodmanRunner_Run_BuildsCorrectCommand tests that Podman command is built correctly
func TestPodmanRunner_Run_BuildsCorrectCommand(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	runner, err := NewPodmanRunnerWithExecutor("my-image:latest", secretsFile, sessionsDir, []string{}, mock)
	require.NoError(t, err)

	// Run with various options
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test prompt",
		Claude: types.ClaudeOptions{
			Model:   "opus",
			Verbose: true,
		},
		Podman: types.PodmanOptions{
			Memory: "1g",
			CPUs:   "2",
		},
	})

	require.NoError(t, err)

	// Verify command structure
	calls := mock.GetCalls()
	require.Len(t, calls, 1)

	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "podman")
	assert.Contains(t, cmdStr, "run")
	assert.Contains(t, cmdStr, "my-image:latest")
	assert.Contains(t, cmdStr, "--memory=1g")
	assert.Contains(t, cmdStr, "--cpus=2")
	assert.Contains(t, cmdStr, "test prompt")
	assert.Contains(t, cmdStr, "--model=opus")
	assert.Contains(t, cmdStr, "--verbose")
}

// TestMockExecutor_RunStream_CustomFunction tests custom stream function
func TestMockExecutor_RunStream_CustomFunction(t *testing.T) {
	mock := NewMockExecutor()

	// Custom function that returns specific pipes
	mock.RunStreamFunc = func(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error) {
		stdout = io.NopCloser(strings.NewReader("custom stdout"))
		stderr = io.NopCloser(strings.NewReader("custom stderr"))
		start = func() error { return nil }
		wait = func() error { return nil }
		return stdout, stderr, start, wait, nil
	}

	stdout, stderr, start, wait, err := mock.RunStream(context.Background(), []string{"test"})
	require.NoError(t, err)

	err = start()
	require.NoError(t, err)

	data, _ := io.ReadAll(stdout)
	assert.Equal(t, "custom stdout", string(data))

	data, _ = io.ReadAll(stderr)
	assert.Equal(t, "custom stderr", string(data))

	err = wait()
	require.NoError(t, err)
}
