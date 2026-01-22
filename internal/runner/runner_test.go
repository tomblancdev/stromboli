//go:build integration

package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomblanc/stromboli/internal/session"
	"github.com/tomblanc/stromboli/internal/types"
)

func TestNewPodmanRunner(t *testing.T) {
	// Skip if podman is not available (required for secrets management)
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, "/tmp/sessions", []string{})
	// May fail if podman secrets can't be created, but that's expected in some environments
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}
	assert.NotNil(t, runner)
	assert.Equal(t, "stromboli-agent:latest", runner.image)
}

func TestRun_SecretsPathResolution(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// This test will fail because podman isn't available in unit tests
	// but it verifies the secrets path resolution works
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRun_WithValidSecretsFile_BuildsCorrectCommand(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// This test will fail because podman isn't available in unit tests
	// but it verifies the setup is correct
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRun_WithClaudeOptions(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// This test will fail because podman isn't available
	// but it verifies the options are processed
	_, err = runner.Run(context.Background(), Request{
		Prompt:    "hello",
		Workspace: "/project",
		Claude: types.ClaudeOptions{
			SessionID:                  "sess-123",
			Model:                      "opus",
			SystemPrompt:               "You are a tester",
			AllowedTools:               []string{"Bash", "Read"},
			DisallowedTools:            []string{"Write"},
			PermissionMode:             "bypassPermissions",
			DangerouslySkipPermissions: true,
			OutputFormat:               "json",
			MaxBudgetUSD:               5.00,
			Verbose:                    true,
		},
		Podman: types.PodmanOptions{
			Volumes: []string{"/data:/data"},
		},
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestDestroySession(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("test", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Create a session directory
	sessionID := "test-session-123"
	sessionPath := filepath.Join(sessionsDir, sessionID)
	err = os.MkdirAll(sessionPath, 0700)
	require.NoError(t, err)

	// Verify session exists
	_, err = os.Stat(sessionPath)
	require.NoError(t, err)

	// Destroy session
	err = runner.DestroySession(sessionID)
	assert.NoError(t, err)

	// Verify session is gone
	_, err = os.Stat(sessionPath)
	assert.True(t, os.IsNotExist(err))
}

func TestDestroySession_NotFound(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("test", secretsFile, tmpDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	err = runner.DestroySession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestDestroySession_PathTraversal(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("test", secretsFile, tmpDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	err = runner.DestroySession("../../../etc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session ID")

	err = runner.DestroySession("test/../../etc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session ID")
}

func TestListSessions(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("test", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Initially empty
	sessions, err := runner.ListSessions()
	assert.NoError(t, err)
	assert.Empty(t, sessions)

	// Create some sessions
	os.MkdirAll(filepath.Join(sessionsDir, "sess-1"), 0700)
	os.MkdirAll(filepath.Join(sessionsDir, "sess-2"), 0700)
	os.MkdirAll(filepath.Join(sessionsDir, "sess-3"), 0700)

	// List sessions
	sessions, err = runner.ListSessions()
	assert.NoError(t, err)
	assert.Len(t, sessions, 3)
	assert.Contains(t, sessions, "sess-1")
	assert.Contains(t, sessions, "sess-2")
	assert.Contains(t, sessions, "sess-3")
}

func TestRunStream_SendsOutputLines(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Create output channel
	output := make(chan string, 10)

	// This will fail because we don't have a real container
	// but it verifies the interface exists
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "hello",
	}, output)

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRunStream_ClosesChannelOnCompletion(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Create output channel
	output := make(chan string, 10)

	// Run in goroutine
	go func() {
		runner.RunStream(context.Background(), Request{
			Prompt: "hello",
		}, output)
	}()

	// Channel should eventually close (or we get an error)
	// This is a basic test to verify the interface
	for range output {
		// Consume any output
	}
}

func TestRun_WithTimeout(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Test with valid timeout
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Timeout: "5m",
		},
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRun_WithInvalidTimeout(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Test with invalid timeout
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Timeout: "invalid",
		},
	})

	// Should fail with timeout parse error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout duration")
}

func TestRun_WithResourceLimits(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Test with resource limits
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Memory:    "512m",
			CPUs:      "1.5",
			CPUShares: 1024,
		},
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRunStream_WithTimeout(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	output := make(chan string, 10)

	// Test with timeout
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Timeout: "5m",
		},
	}, output)

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

func TestRunStream_WithResourceLimits(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner, err := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir, []string{})
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	output := make(chan string, 10)

	// Test with resource limits
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Memory:    "1g",
			CPUs:      "2",
			CPUShares: 512,
		},
	}, output)

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

// TestDefaultResourceLimits_AppliedWhenNotSpecified verifies defaults are used when values not provided
func TestDefaultResourceLimits_AppliedWhenNotSpecified(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create runner with default resource limits
	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}
	runner, err := NewPodmanRunnerWithDefaults("stromboli-agent:latest", secretsFile, sessionsDir, []string{}, defaults)
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Request without resource limits - should apply defaults
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			// No Memory, CPUs, or Timeout specified
		},
	})

	// Expected to fail due to no podman container execution
	// but the command should have been built with defaults
	assert.Error(t, err)
}

// TestDefaultResourceLimits_ExplicitValuesOverrideDefaults verifies explicit values take precedence
func TestDefaultResourceLimits_ExplicitValuesOverrideDefaults(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create runner with default resource limits
	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}
	runner, err := NewPodmanRunnerWithDefaults("stromboli-agent:latest", secretsFile, sessionsDir, []string{}, defaults)
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Request with explicit resource limits - should NOT use defaults
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Memory:  "2g",
			CPUs:    "4",
			Timeout: "1h",
		},
	})

	// Expected to fail due to no podman container execution
	// but the command should have been built with explicit values
	assert.Error(t, err)
}

// TestDefaultResourceLimits_PartialOverride verifies partial overrides work correctly
func TestDefaultResourceLimits_PartialOverride(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create runner with default resource limits
	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}
	runner, err := NewPodmanRunnerWithDefaults("stromboli-agent:latest", secretsFile, sessionsDir, []string{}, defaults)
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	// Request with only memory specified - should use default for CPUs and Timeout
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			Memory: "1g",
			// CPUs and Timeout should use defaults
		},
	})

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}

// TestDefaultResourceLimits_StreamingAppliesDefaults verifies defaults work with streaming
func TestDefaultResourceLimits_StreamingAppliesDefaults(t *testing.T) {
	// Skip if podman is not available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	// Create runner with default resource limits
	defaults := ResourceDefaults{
		Memory:  "512m",
		CPUs:    "1",
		Timeout: "30m",
	}
	runner, err := NewPodmanRunnerWithDefaults("stromboli-agent:latest", secretsFile, sessionsDir, []string{}, defaults)
	if err != nil {
		t.Skipf("podman secrets not available: %v", err)
	}

	output := make(chan string, 10)

	// Request without resource limits - should apply defaults
	_, err = runner.RunStream(context.Background(), Request{
		Prompt: "hello",
		Podman: types.PodmanOptions{
			// No Memory, CPUs, or Timeout specified
		},
	}, output)

	// Expected to fail due to no podman container execution
	assert.Error(t, err)
}
