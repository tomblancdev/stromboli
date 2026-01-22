package podman

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommand_DefaultsToRun(t *testing.T) {
	cmd := NewCommand().Build()
	assert.Equal(t, []string{"podman", "run", "--rm"}, cmd)
}

func TestWithImage(t *testing.T) {
	cmd := NewCommand().
		WithImage("stromboli-agent:latest").
		Build()
	assert.Equal(t, []string{"podman", "run", "--rm", "stromboli-agent:latest"}, cmd)
}

func TestWithEnv(t *testing.T) {
	cmd := NewCommand().
		WithEnv("TOKEN", "secret123").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "-e")
	assert.Contains(t, cmd, "TOKEN=secret123")
}

func TestWithMultipleEnv(t *testing.T) {
	cmd := NewCommand().
		WithEnv("VAR1", "value1").
		WithEnv("VAR2", "value2").
		WithImage("alpine").
		Build()
	// Count -e flags
	count := 0
	for _, arg := range cmd {
		if arg == "-e" {
			count++
		}
	}
	assert.Equal(t, 2, count)
}

func TestWithVolume(t *testing.T) {
	cmd := NewCommand().
		WithVolume("/host/path", "/container/path").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "-v")
	assert.Contains(t, cmd, "/host/path:/container/path")
}

func TestWithVolumeReadOnly(t *testing.T) {
	cmd := NewCommand().
		WithVolumeReadOnly("/host", "/container").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "/host:/container:ro")
}

func TestWithSecretFile(t *testing.T) {
	cmd := NewCommand().
		WithSecretFile("/path/to/secrets", "/run/secrets/claude-token").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "-v")
	assert.Contains(t, cmd, "/path/to/secrets:/run/secrets/claude-token:ro")
}

func TestWithWorkdir(t *testing.T) {
	cmd := NewCommand().
		WithWorkdir("/workspace").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "-w")
	assert.Contains(t, cmd, "/workspace")
}

func TestWithInteractive(t *testing.T) {
	cmd := NewCommand().
		WithInteractive().
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "-it")
}

func TestWithName(t *testing.T) {
	cmd := NewCommand().
		WithName("my-container").
		WithImage("alpine").
		Build()
	assert.Contains(t, cmd, "--name")
	assert.Contains(t, cmd, "my-container")
}

func TestWithCommand(t *testing.T) {
	cmd := NewCommand().
		WithImage("alpine").
		WithCommand([]string{"echo", "hello"}).
		Build()
	// Command should be at the end after image
	assert.Equal(t, "echo", cmd[len(cmd)-2])
	assert.Equal(t, "hello", cmd[len(cmd)-1])
}

func TestFullBuilder(t *testing.T) {
	cmd := NewCommand().
		WithName("test-agent").
		WithSecretFile("/path/to/.claude-secrets", "/run/secrets/claude-token").
		WithEnv("HOME", "/home/claude").
		WithVolume("/home/user/project", "/workspace").
		WithWorkdir("/workspace").
		WithImage("stromboli-agent:latest").
		WithCommand([]string{"claude", "-p", "hello"}).
		Build()

	assert.Equal(t, "podman", cmd[0])
	assert.Equal(t, "run", cmd[1])
	assert.Contains(t, cmd, "--rm")
	assert.Contains(t, cmd, "--name")
	assert.Contains(t, cmd, "test-agent")
	assert.Contains(t, cmd, "-v")
	assert.Contains(t, cmd, "/path/to/.claude-secrets:/run/secrets/claude-token:ro")
	assert.Contains(t, cmd, "-e")
	assert.Contains(t, cmd, "HOME=/home/claude")
	assert.Contains(t, cmd, "/home/user/project:/workspace")
	assert.Contains(t, cmd, "-w")
	assert.Contains(t, cmd, "/workspace")
	assert.Contains(t, cmd, "stromboli-agent:latest")
	// Command at the end
	assert.Equal(t, "claude", cmd[len(cmd)-3])
	assert.Equal(t, "-p", cmd[len(cmd)-2])
	assert.Equal(t, "hello", cmd[len(cmd)-1])

	// SECURITY: Verify token is NOT in command line
	for _, arg := range cmd {
		assert.NotContains(t, arg, "CLAUDE_CODE_OAUTH_TOKEN")
	}
}
