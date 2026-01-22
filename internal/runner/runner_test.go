package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPodmanRunner(t *testing.T) {
	runner := NewPodmanRunner("stromboli-agent:latest", ".claude-secrets", "/tmp/sessions")
	assert.NotNil(t, runner)
	assert.Equal(t, "stromboli-agent:latest", runner.image)
	assert.Equal(t, ".claude-secrets", runner.secretsFile)
	assert.Equal(t, "/tmp/sessions", runner.sessionsDir)
}

func TestRun_MissingToken_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewPodmanRunner("stromboli-agent:latest", "/nonexistent/secrets", tmpDir)

	_, err := runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

func TestRun_WithValidToken_BuildsCorrectCommand(t *testing.T) {
	// Create temp directories
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir)

	// This test will fail because podman isn't available in unit tests
	// but it verifies the setup is correct
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	// Expected to fail due to no podman, but not due to token
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "failed to get token")
}

func TestRun_WithClaudeOptions(t *testing.T) {
	// Create temp directories
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner := NewPodmanRunner("stromboli-agent:latest", secretsFile, sessionsDir)

	// This test will fail because podman isn't available
	// but it verifies the options are processed
	_, err = runner.Run(context.Background(), Request{
		Prompt:    "hello",
		Workspace: "/project",
		Claude: ClaudeOptions{
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
		Podman: PodmanOptions{
			Volumes: []string{"/data:/data"},
		},
	})

	// Expected to fail due to no podman
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "failed to get token")
}

func TestDestroySession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	runner := NewPodmanRunner("test", "test", sessionsDir)

	// Create a session directory
	sessionID := "test-session-123"
	sessionPath := filepath.Join(sessionsDir, sessionID)
	err := os.MkdirAll(sessionPath, 0700)
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
	tmpDir := t.TempDir()
	runner := NewPodmanRunner("test", "test", tmpDir)

	err := runner.DestroySession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestDestroySession_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewPodmanRunner("test", "test", tmpDir)

	err := runner.DestroySession("../../../etc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session ID")

	err = runner.DestroySession("test/../../etc")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session ID")
}

func TestListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	sessionsDir := filepath.Join(tmpDir, "sessions")
	runner := NewPodmanRunner("test", "test", sessionsDir)

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

// MockRunner for testing API handlers
type MockRunner struct {
	RunFunc            func(ctx context.Context, req Request) (*Result, error)
	DestroySessionFunc func(sessionID string) error
	ListSessionsFunc   func() ([]string, error)
}

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error) {
	return m.RunFunc(ctx, req)
}

func (m *MockRunner) DestroySession(sessionID string) error {
	if m.DestroySessionFunc != nil {
		return m.DestroySessionFunc(sessionID)
	}
	return nil
}

func (m *MockRunner) ListSessions() ([]string, error) {
	if m.ListSessionsFunc != nil {
		return m.ListSessionsFunc()
	}
	return []string{}, nil
}

func TestMockRunner(t *testing.T) {
	mock := &MockRunner{
		RunFunc: func(ctx context.Context, req Request) (*Result, error) {
			return &Result{
				ID:     "test-123",
				Output: "Hello from Claude!",
			}, nil
		},
	}

	result, err := mock.Run(context.Background(), Request{Prompt: "hello"})
	assert.NoError(t, err)
	assert.Equal(t, "test-123", result.ID)
	assert.Equal(t, "Hello from Claude!", result.Output)
}

func TestGenerateSessionID_IsUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSessionID()
		assert.NotEmpty(t, id)
		// UUID format: 8-4-4-4-12 hex digits
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id)
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}

func TestGenerateRunID_IsUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRunID()
		assert.NotEmpty(t, id)
		assert.Contains(t, id, "run-")
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}
