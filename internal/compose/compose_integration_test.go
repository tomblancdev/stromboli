//go:build integration

package compose

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ShellExecutor implements Executor for real podman commands
type ShellExecutor struct{}

func (e *ShellExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	return cmd.CombinedOutput()
}

func (e *ShellExecutor) RunStream(ctx context.Context, args []string) (stdout, stderr io.ReadCloser, start func() error, wait func() error, err error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	stdout, err = cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	start = cmd.Start
	wait = cmd.Wait

	return stdout, stderr, start, wait, nil
}

func skipIfNoPodman(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping integration test")
	}
}

func TestComposeManager_Integration_UpDown(t *testing.T) {
	skipIfNoPodman(t)

	// Create a temporary compose file
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  alpine:
    image: alpine:latest
    command: sleep infinity
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	executor := &ShellExecutor{}
	mgr := NewManager(executor, Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
		BuildTimeout:     2 * time.Minute,
		HealthTimeout:    30 * time.Second,
		StackTTL:         1 * time.Hour,
	})

	ctx := context.Background()
	sessionID := "test-integration-" + time.Now().Format("20060102150405")

	// Start the stack
	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "alpine",
	}

	stack, err := mgr.Up(ctx, env, sessionID)
	require.NoError(t, err)
	assert.NotNil(t, stack)
	assert.Equal(t, ProjectName(sessionID), stack.ProjectName)

	// Clean up
	defer func() {
		err := mgr.Down(ctx, sessionID)
		assert.NoError(t, err)
	}()

	// Verify stack is tracked
	assert.True(t, mgr.IsActive(sessionID))

	// Execute a command in the container
	output, err := mgr.Exec(ctx, sessionID, "alpine", []string{"echo", "hello from compose"})
	require.NoError(t, err)
	assert.Contains(t, string(output), "hello from compose")
}

func TestComposeManager_Integration_ValidationRejectsPrivileged(t *testing.T) {
	skipIfNoPodman(t)

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	content := `services:
  alpine:
    image: alpine:latest
    privileged: true
`
	err := os.WriteFile(composePath, []byte(content), 0644)
	require.NoError(t, err)

	executor := &ShellExecutor{}
	mgr := NewManager(executor, Config{
		AllowPrivileged:  false,
		AllowHostNetwork: false,
		AllowHostVolumes: false,
		BuildTimeout:     2 * time.Minute,
		HealthTimeout:    30 * time.Second,
		StackTTL:         1 * time.Hour,
	})

	ctx := context.Background()
	sessionID := "test-validation-" + time.Now().Format("20060102150405")

	env := Environment{
		Type:    "compose",
		Path:    composePath,
		Service: "alpine",
	}

	_, err = mgr.Up(ctx, env, sessionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "privileged")

	// Ensure no stack was created
	assert.False(t, mgr.IsActive(sessionID))
}
