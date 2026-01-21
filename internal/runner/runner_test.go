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
	runner := NewPodmanRunner("stromboli-agent:latest", ".claude-secrets")
	assert.NotNil(t, runner)
	assert.Equal(t, "stromboli-agent:latest", runner.image)
	assert.Equal(t, ".claude-secrets", runner.secretsFile)
}

func TestRun_MissingToken_ReturnsError(t *testing.T) {
	runner := NewPodmanRunner("stromboli-agent:latest", "/nonexistent/secrets")

	_, err := runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get token")
}

func TestRun_WithValidToken_BuildsCorrectCommand(t *testing.T) {
	// Create temp secrets file
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	runner := NewPodmanRunner("stromboli-agent:latest", secretsFile)

	// This test will fail because podman isn't available in unit tests
	// but it verifies the setup is correct
	_, err = runner.Run(context.Background(), Request{
		Prompt: "hello",
	})

	// Expected to fail due to no podman, but not due to token
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "failed to get token")
}

// MockRunner for testing API handlers
type MockRunner struct {
	RunFunc func(ctx context.Context, req Request) (*Result, error)
}

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error) {
	return m.RunFunc(ctx, req)
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
