package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommandBuilder_Empty(t *testing.T) {
	cmd := NewCommandBuilder().Build()
	assert.Equal(t, []string{"podman", "compose"}, cmd)
}

func TestCommandBuilder_Up(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *CommandBuilder
		expected []string
	}{
		{
			name: "basic up",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().Up()
			},
			expected: []string{"podman", "compose", "up"},
		},
		{
			name: "up with file",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithFile("/path/to/docker-compose.yml").
					Up()
			},
			expected: []string{"podman", "compose", "-f", "/path/to/docker-compose.yml", "up"},
		},
		{
			name: "up with project",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-session-123").
					Up()
			},
			expected: []string{"podman", "compose", "-p", "stromboli-session-123", "up"},
		},
		{
			name: "up detached",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Up().
					Detached()
			},
			expected: []string{"podman", "compose", "up", "-d"},
		},
		{
			name: "up with build",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Up().
					WithBuild()
			},
			expected: []string{"podman", "compose", "up", "--build"},
		},
		{
			name: "up detached with build",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Up().
					Detached().
					WithBuild()
			},
			expected: []string{"podman", "compose", "up", "-d", "--build"},
		},
		{
			name: "full up command",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithFile("/path/to/compose.yml").
					WithProject("stromboli-abc123").
					Up().
					Detached().
					WithBuild()
			},
			expected: []string{
				"podman", "compose",
				"-f", "/path/to/compose.yml",
				"-p", "stromboli-abc123",
				"up", "-d", "--build",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.builder().Build()
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestCommandBuilder_Down(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *CommandBuilder
		expected []string
	}{
		{
			name: "basic down",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().Down()
			},
			expected: []string{"podman", "compose", "down"},
		},
		{
			name: "down with project",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-session-123").
					Down()
			},
			expected: []string{"podman", "compose", "-p", "stromboli-session-123", "down"},
		},
		{
			name: "down with volumes",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Down().
					RemoveVolumes()
			},
			expected: []string{"podman", "compose", "down", "-v"},
		},
		{
			name: "full down command",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithFile("/path/to/compose.yml").
					WithProject("stromboli-abc123").
					Down().
					RemoveVolumes()
			},
			expected: []string{
				"podman", "compose",
				"-f", "/path/to/compose.yml",
				"-p", "stromboli-abc123",
				"down", "-v",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.builder().Build()
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestCommandBuilder_Exec(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *CommandBuilder
		expected []string
	}{
		{
			name: "basic exec",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Exec("dev", "echo", "hello")
			},
			expected: []string{"podman", "compose", "exec", "-T", "dev", "echo", "hello"},
		},
		{
			name: "exec with project",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-session-123").
					Exec("dev", "pip", "install", "-r", "requirements.txt")
			},
			expected: []string{
				"podman", "compose",
				"-p", "stromboli-session-123",
				"exec", "-T", "dev",
				"pip", "install", "-r", "requirements.txt",
			},
		},
		{
			name: "exec with TTY",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Exec("dev", "bash").
					WithTTY()
			},
			expected: []string{"podman", "compose", "exec", "dev", "bash"},
		},
		{
			name: "exec claude command",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-abc123").
					Exec("dev", "claude", "-p", "hello world")
			},
			expected: []string{
				"podman", "compose",
				"-p", "stromboli-abc123",
				"exec", "-T", "dev",
				"claude", "-p", "hello world",
			},
		},
		{
			name: "exec with file and project",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithFile("/path/to/compose.yml").
					WithProject("stromboli-abc123").
					Exec("api", "npm", "test")
			},
			expected: []string{
				"podman", "compose",
				"-f", "/path/to/compose.yml",
				"-p", "stromboli-abc123",
				"exec", "-T", "api",
				"npm", "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.builder().Build()
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestCommandBuilder_Ps(t *testing.T) {
	tests := []struct {
		name     string
		builder  func() *CommandBuilder
		expected []string
	}{
		{
			name: "basic ps",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().Ps()
			},
			expected: []string{"podman", "compose", "ps"},
		},
		{
			name: "ps with project",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-session-123").
					Ps()
			},
			expected: []string{"podman", "compose", "-p", "stromboli-session-123", "ps"},
		},
		{
			name: "ps json format",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					Ps().
					FormatJSON()
			},
			expected: []string{"podman", "compose", "ps", "--format", "json"},
		},
		{
			name: "full ps command",
			builder: func() *CommandBuilder {
				return NewCommandBuilder().
					WithProject("stromboli-abc123").
					Ps().
					FormatJSON()
			},
			expected: []string{
				"podman", "compose",
				"-p", "stromboli-abc123",
				"ps", "--format", "json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.builder().Build()
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestProjectName(t *testing.T) {
	tests := []struct {
		sessionID string
		expected  string
	}{
		{"abc123", "stromboli-abc123"},
		{"session-001", "stromboli-session-001"},
		{"550e8400-e29b-41d4-a716-446655440000", "stromboli-550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.sessionID, func(t *testing.T) {
			result := ProjectName(tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
