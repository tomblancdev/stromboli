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

// TestPodmanRunner_Run_WorkspaceAsWorkdir tests workspace sets the working directory
func TestPodmanRunner_Run_WorkspaceAsWorkdir(t *testing.T) {
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

	// Workspace is now just a container path (working directory)
	_, err = runner.Run(context.Background(), Request{
		Prompt:    "test",
		Workdir: "/workspace",
	})
	require.NoError(t, err)

	// Verify --workdir is set in the command
	calls := mock.GetCalls()
	require.Len(t, calls, 1)
	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "--workdir=/workspace")

	// Workspace should NOT create a volume mount (that's what volumes are for)
	// Count volume mounts - should only be the session mount, not workspace
	volCount := strings.Count(cmdStr, "-v ")
	assert.Equal(t, 1, volCount, "should only have session volume mount, not workspace")
}

// TestPodmanRunner_Run_VolumeMounting tests volume mounts via podman.volumes
func TestPodmanRunner_Run_VolumeMounting(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	hostCodeDir := filepath.Join(tmpDir, "code")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(hostCodeDir, 0755)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	// Allow tmpDir for volume mounts
	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{tmpDir}, mock)
	require.NoError(t, err)

	// Use volumes to mount host paths
	_, err = runner.Run(context.Background(), Request{
		Prompt:  "test",
		Workdir: "/app", // Container working directory
		Podman: types.PodmanOptions{
			Volumes: []string{hostCodeDir + ":/app:ro"}, // Host path mount
		},
	})
	require.NoError(t, err)

	// Verify both workdir and volume are in the command
	calls := mock.GetCalls()
	require.Len(t, calls, 1)
	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "--workdir=/app")
	assert.Contains(t, cmdStr, "-v "+hostCodeDir+":/app:ro")
}

// TestPodmanRunner_Run_VolumeValidation tests volume validation against allowlist
func TestPodmanRunner_Run_VolumeValidation(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	allowedDir := filepath.Join(tmpDir, "allowed")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(allowedDir, 0755)
	require.NoError(t, err)

	// Create mock executor
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	// Only allow the allowedDir
	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{allowedDir}, mock)
	require.NoError(t, err)

	// Valid volume (path in allowlist)
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Volumes: []string{allowedDir + ":/workspace"},
		},
	})
	require.NoError(t, err)

	// Invalid volume (path not in allowlist)
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Volumes: []string{"/not/allowed:/workspace"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "volume validation failed")
}

// TestPodmanRunner_Run_WorkdirAutoCreate tests that workdir command includes mkdir
func TestPodmanRunner_Run_WorkdirAutoCreate(t *testing.T) {
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

	// Run with workdir
	_, err = runner.Run(context.Background(), Request{
		Prompt:  "test",
		Workdir: "/my/custom/workdir",
	})
	require.NoError(t, err)

	// Verify command is wrapped with mkdir
	calls := mock.GetCalls()
	require.Len(t, calls, 1)
	cmdStr := strings.Join(calls[0], " ")
	assert.Contains(t, cmdStr, "sh -c")
	assert.Contains(t, cmdStr, "mkdir -p '/my/custom/workdir'")
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

// =============================================================================
// Security Validation Tests
// =============================================================================

// TestParseVolumeComponents tests volume string parsing
func TestParseVolumeComponents(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantHost      string
		wantContainer string
		wantOptions   string
		wantErr       bool
	}{
		{
			name:          "simple unix volume",
			input:         "/host/path:/container/path",
			wantHost:      "/host/path",
			wantContainer: "/container/path",
			wantOptions:   "",
			wantErr:       false,
		},
		{
			name:          "unix volume with options",
			input:         "/host/path:/container/path:ro",
			wantHost:      "/host/path",
			wantContainer: "/container/path",
			wantOptions:   "ro",
			wantErr:       false,
		},
		{
			name:          "unix volume with multiple options",
			input:         "/host:/container:ro,Z",
			wantHost:      "/host",
			wantContainer: "/container",
			wantOptions:   "ro,Z",
			wantErr:       false,
		},
		{
			name:    "invalid single path",
			input:   "/just/one/path",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, container, options, err := parseVolumeComponents(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host)
			assert.Equal(t, tt.wantContainer, container)
			assert.Equal(t, tt.wantOptions, options)
		})
	}
}

// TestValidateWorkdir tests workdir validation for shell safety
func TestValidateWorkdir(t *testing.T) {
	tests := []struct {
		name    string
		workdir string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty workdir is valid",
			workdir: "",
			wantErr: false,
		},
		{
			name:    "simple valid path",
			workdir: "/workspace",
			wantErr: false,
		},
		{
			name:    "nested valid path",
			workdir: "/home/user/projects/myapp",
			wantErr: false,
		},
		{
			name:    "path with dots and hyphens",
			workdir: "/my.project-v2/src",
			wantErr: false,
		},
		{
			name:    "path with underscore",
			workdir: "/my_project/src_files",
			wantErr: false,
		},
		{
			name:    "relative path fails",
			workdir: "relative/path",
			wantErr: true,
			errMsg:  "must be an absolute path",
		},
		{
			name:    "path traversal fails",
			workdir: "/workspace/../etc/passwd",
			wantErr: true,
			errMsg:  "cannot contain path traversal",
		},
		{
			name:    "shell metacharacters fail",
			workdir: "/workspace$(id)",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "backticks fail",
			workdir: "/workspace`id`",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "semicolon fails",
			workdir: "/workspace;rm -rf /",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "newline fails",
			workdir: "/workspace\nrm -rf /",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "spaces fail",
			workdir: "/path with spaces",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "too long path fails",
			workdir: "/" + strings.Repeat("a", 5000),
			wantErr: true,
			errMsg:  "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkdir(tt.workdir)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateContainerPath tests container path blocklist validation
func TestValidateContainerPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid container path",
			path:    "/workspace",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "/app/data/files",
			wantErr: false,
		},
		{
			name:    "blocked: /etc",
			path:    "/etc",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: /etc subdirectory",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: user .claude directory",
			path:    "/home/user/.claude",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: user .claude subdirectory",
			path:    "/home/user/.claude/credentials.json",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .ssh directory",
			path:    "/home/user/.ssh",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .aws credentials",
			path:    "/home/user/.aws",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .docker credentials",
			path:    "/home/user/.docker",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .kube credentials",
			path:    "/home/user/.kube",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .config directory",
			path:    "/home/user/.config",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: .bashrc",
			path:    "/home/user/.bashrc",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: /root",
			path:    "/root",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: /proc",
			path:    "/proc",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "blocked: /dev",
			path:    "/dev",
			wantErr: true,
			errMsg:  "blocked for security",
		},
		{
			name:    "empty path fails",
			path:    "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "relative path fails",
			path:    "relative",
			wantErr: true,
			errMsg:  "must be absolute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContainerPath(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestValidateMountOptions tests mount option validation
func TestValidateMountOptions(t *testing.T) {
	tests := []struct {
		name    string
		options string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty options valid",
			options: "",
			wantErr: false,
		},
		{
			name:    "ro option valid",
			options: "ro",
			wantErr: false,
		},
		{
			name:    "rw option valid",
			options: "rw",
			wantErr: false,
		},
		{
			name:    "Z option valid",
			options: "Z",
			wantErr: false,
		},
		{
			name:    "z option valid",
			options: "z",
			wantErr: false,
		},
		{
			name:    "multiple valid options",
			options: "ro,Z",
			wantErr: false,
		},
		{
			name:    "multiple valid options with spaces",
			options: "ro, Z, nocopy",
			wantErr: false,
		},
		{
			name:    "noexec option valid (security-enhancing)",
			options: "noexec",
			wantErr: false,
		},
		{
			name:    "nosuid option valid (security-enhancing)",
			options: "nosuid",
			wantErr: false,
		},
		{
			name:    "nodev option valid (security-enhancing)",
			options: "nodev",
			wantErr: false,
		},
		{
			name:    "combined security options valid",
			options: "ro,noexec,nosuid,nodev",
			wantErr: false,
		},
		{
			name:    "exec option not allowed",
			options: "exec",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "suid option not allowed",
			options: "suid",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "noexec mixed with invalid",
			options: "ro,exec",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "arbitrary string not allowed",
			options: "arbitrary",
			wantErr: true,
			errMsg:  "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMountOptions(tt.options)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestWrapCommandWithWorkdirSetup tests command wrapping
func TestWrapCommandWithWorkdirSetup(t *testing.T) {
	tests := []struct {
		name       string
		workdir    string
		cmd        []string
		autoCreate bool
		wantWrap   bool
	}{
		{
			name:       "no wrap when empty workdir",
			workdir:    "",
			cmd:        []string{"echo", "hello"},
			autoCreate: true,
			wantWrap:   false,
		},
		{
			name:       "no wrap when autoCreate false",
			workdir:    "/workspace",
			cmd:        []string{"echo", "hello"},
			autoCreate: false,
			wantWrap:   false,
		},
		{
			name:       "wrap when workdir and autoCreate",
			workdir:    "/workspace",
			cmd:        []string{"echo", "hello"},
			autoCreate: true,
			wantWrap:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapCommandWithWorkdirSetup(tt.workdir, tt.cmd, tt.autoCreate)
			if tt.wantWrap {
				assert.Equal(t, "sh", result[0])
				assert.Equal(t, "-c", result[1])
				assert.Contains(t, result[2], "mkdir -p")
			} else {
				assert.Equal(t, tt.cmd, result)
			}
		})
	}
}

// TestWrapCommandWithWorkdirSetup_QuoteEscaping tests proper quote escaping
func TestWrapCommandWithWorkdirSetup_QuoteEscaping(t *testing.T) {
	// Command with single quotes
	cmd := []string{"echo", "it's working"}
	result := wrapCommandWithWorkdirSetup("/workspace", cmd, true)

	// Should properly escape the quote
	assert.Contains(t, result[2], "'it'\\''s working'")
}

// TestPodmanRunner_Run_WorkdirValidation tests workdir validation in runner
func TestPodmanRunner_Run_WorkdirValidation(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	allowedDir := filepath.Join(tmpDir, "allowed")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(allowedDir, 0755)
	require.NoError(t, err)

	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{allowedDir}, mock)
	require.NoError(t, err)

	// Invalid workdir with shell metacharacters should fail
	_, err = runner.Run(context.Background(), Request{
		Prompt:  "test",
		Workdir: "/workspace$(id)",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workdir validation failed")
}

// TestPodmanRunner_Run_ContainerPathValidation tests container path blocking
func TestPodmanRunner_Run_ContainerPathValidation(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	allowedDir := filepath.Join(tmpDir, "allowed")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(allowedDir, 0755)
	require.NoError(t, err)

	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{allowedDir}, mock)
	require.NoError(t, err)

	// Mounting to blocked container path should fail
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Volumes: []string{allowedDir + ":/etc"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked for security")
}

// TestPodmanRunner_Run_MountOptionsValidation tests mount options validation
func TestPodmanRunner_Run_MountOptionsValidation(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	allowedDir := filepath.Join(tmpDir, "allowed")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)
	err = os.MkdirAll(allowedDir, 0755)
	require.NoError(t, err)

	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	runner, err := NewPodmanRunnerWithExecutor("test-image", secretsFile, sessionsDir, []string{allowedDir}, mock)
	require.NoError(t, err)

	// Valid mount options should work
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Volumes: []string{allowedDir + ":/workspace:ro"},
		},
	})
	require.NoError(t, err)

	// Invalid mount options should fail
	_, err = runner.Run(context.Background(), Request{
		Prompt: "test",
		Podman: types.PodmanOptions{
			Volumes: []string{allowedDir + ":/workspace:exec"},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

// TestPodmanRunner_Run_WorkdirAutoCreateDisabled tests workdir_auto_create=false
func TestPodmanRunner_Run_WorkdirAutoCreateDisabled(t *testing.T) {
	skipIfNoPodman(t)
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".credentials.json")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	volumeConfig := VolumeConfig{
		AllowedVolumes:    []string{},
		AllowAllVolumes:   true, // Allow all for this test
		WorkdirAutoCreate: false,
	}

	runner, err := NewPodmanRunnerFull("test-image", secretsFile, sessionsDir, sessionsDir, ResourceDefaults{}, ImageConfig{}, volumeConfig, mock)
	require.NoError(t, err)

	// Run with workdir but auto-create disabled
	_, err = runner.Run(context.Background(), Request{
		Prompt:  "test",
		Workdir: "/my/workdir",
	})
	require.NoError(t, err)

	// Verify command does NOT include mkdir wrapper
	calls := mock.GetCalls()
	require.Len(t, calls, 1)
	cmdStr := strings.Join(calls[0], " ")

	// Should NOT be wrapped with sh -c mkdir
	assert.NotContains(t, cmdStr, "mkdir -p")
	// But should still have workdir flag
	assert.Contains(t, cmdStr, "--workdir=/my/workdir")
}
