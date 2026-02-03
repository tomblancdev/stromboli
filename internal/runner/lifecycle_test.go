package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"stromboli/internal/types"
)

// makeArgs creates a slice of n non-empty string arguments for testing
func makeArgs(n int) []string {
	args := make([]string, n)
	for i := range args {
		args[i] = "arg"
	}
	return args
}

// =============================================================================
// Helper Function Tests
// =============================================================================

// TestHasAnyHooks checks hook presence detection
func TestHasAnyHooks(t *testing.T) {
	tests := []struct {
		name     string
		hooks    types.LifecycleHooks
		expected bool
	}{
		{
			name:     "empty hooks",
			hooks:    types.LifecycleHooks{},
			expected: false,
		},
		{
			name: "only onCreateCommand",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"cmd"},
			},
			expected: true,
		},
		{
			name: "only postCreate",
			hooks: types.LifecycleHooks{
				PostCreate: []string{"cmd"},
			},
			expected: true,
		},
		{
			name: "only postStart",
			hooks: types.LifecycleHooks{
				PostStart: []string{"cmd"},
			},
			expected: true,
		},
		{
			name: "all hooks",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"cmd1"},
				PostCreate:      []string{"cmd2"},
				PostStart:       []string{"cmd3"},
			},
			expected: true,
		},
		{
			name: "empty slices",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{},
				PostCreate:      []string{},
				PostStart:       []string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAnyHooks(tt.hooks)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHasInitHooks checks init hook presence detection
func TestHasInitHooks(t *testing.T) {
	tests := []struct {
		name     string
		hooks    types.LifecycleHooks
		expected bool
	}{
		{
			name:     "empty hooks",
			hooks:    types.LifecycleHooks{},
			expected: false,
		},
		{
			name: "only postStart (not init)",
			hooks: types.LifecycleHooks{
				PostStart: []string{"cmd"},
			},
			expected: false,
		},
		{
			name: "only onCreateCommand",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"cmd"},
			},
			expected: true,
		},
		{
			name: "only postCreate",
			hooks: types.LifecycleHooks{
				PostCreate: []string{"cmd"},
			},
			expected: true,
		},
		{
			name: "both init hooks",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"cmd1"},
				PostCreate:      []string{"cmd2"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasInitHooks(tt.hooks)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEscapeShellCommand tests command escaping
func TestEscapeShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      []string
		expected string
	}{
		{
			name:     "simple command",
			cmd:      []string{"echo", "hello"},
			expected: "'echo' 'hello'",
		},
		{
			name:     "command with spaces",
			cmd:      []string{"echo", "hello world"},
			expected: "'echo' 'hello world'",
		},
		{
			name:     "command with single quotes",
			cmd:      []string{"echo", "it's working"},
			expected: "'echo' 'it'\\''s working'",
		},
		{
			name:     "command with double quotes",
			cmd:      []string{"echo", "say \"hello\""},
			expected: "'echo' 'say \"hello\"'",
		},
		{
			name:     "empty command",
			cmd:      []string{},
			expected: "",
		},
		{
			name:     "nil command",
			cmd:      nil,
			expected: "",
		},
		{
			name:     "command with special chars",
			cmd:      []string{"bash", "-c", "echo $HOME && ls -la"},
			expected: "'bash' '-c' 'echo $HOME && ls -la'",
		},
		{
			name:     "command with backslashes",
			cmd:      []string{"echo", "path\\to\\file"},
			expected: "'echo' 'path\\to\\file'",
		},
		{
			name:     "command with backslash and quote",
			cmd:      []string{"echo", "it\\'s"},
			expected: "'echo' 'it\\'\\''s'",
		},
		{
			name:     "command with newlines",
			cmd:      []string{"echo", "line1\nline2"},
			expected: "'echo' 'line1\nline2'",
		},
		{
			name:     "command with tabs",
			cmd:      []string{"echo", "col1\tcol2"},
			expected: "'echo' 'col1\tcol2'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeShellCommand(tt.cmd)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWrapCommandWithHooks tests command wrapping
func TestWrapCommandWithHooks(t *testing.T) {
	mainCmd := []string{"claude", "--prompt", "hello"}

	tests := []struct {
		name     string
		hooks    types.LifecycleHooks
		runInit  bool
		wantWrap bool
		contains []string
	}{
		{
			name:     "no hooks - no wrap",
			hooks:    types.LifecycleHooks{},
			runInit:  true,
			wantWrap: false,
		},
		{
			name: "only postStart - wrap",
			hooks: types.LifecycleHooks{
				PostStart: []string{"echo", "starting"},
			},
			runInit:  true,
			wantWrap: true,
			contains: []string{"echo", "starting", "claude"},
		},
		{
			name: "init hooks with runInit=true - all hooks run",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"pip", "install"},
				PostCreate:      []string{"npm", "build"},
				PostStart:       []string{"echo", "start"},
			},
			runInit:  true,
			wantWrap: true,
			contains: []string{"pip", "install", "npm", "build", "echo", "start", "claude"},
		},
		{
			name: "init hooks with runInit=false - only postStart runs",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"pip", "install"},
				PostCreate:      []string{"npm", "build"},
				PostStart:       []string{"echo", "start"},
			},
			runInit:  false,
			wantWrap: true,
			contains: []string{"echo", "start", "claude"},
		},
		{
			name: "init hooks only with runInit=false - no wrap",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"pip", "install"},
				PostCreate:      []string{"npm", "build"},
			},
			runInit:  false,
			wantWrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapCommandWithHooks(mainCmd, tt.hooks, tt.runInit)

			if !tt.wantWrap {
				assert.Equal(t, mainCmd, result)
				return
			}

			// Should be wrapped with sh -c
			assert.Equal(t, "sh", result[0])
			assert.Equal(t, "-c", result[1])

			// Check contains expected substrings
			shellCmd := result[2]
			for _, s := range tt.contains {
				assert.Contains(t, shellCmd, s, "shell command should contain %s", s)
			}

			// Should have && for sequential execution
			assert.Contains(t, shellCmd, "&&")
		})
	}
}

// TestWrapCommandWithHooks_FailFast verifies commands are joined with &&
func TestWrapCommandWithHooks_FailFast(t *testing.T) {
	mainCmd := []string{"claude", "--prompt", "hello"}
	hooks := types.LifecycleHooks{
		OnCreateCommand: []string{"cmd1"},
		PostCreate:      []string{"cmd2"},
		PostStart:       []string{"cmd3"},
	}

	result := WrapCommandWithHooks(mainCmd, hooks, true)

	shellCmd := result[2]
	// Count && occurrences (should be 3: between cmd1, cmd2, cmd3, and main)
	count := strings.Count(shellCmd, "&&")
	assert.Equal(t, 3, count, "should have 3 && operators for fail-fast chaining")
}

// TestWrapCommandWithHooks_PreservesMainCommand verifies main command is at the end
func TestWrapCommandWithHooks_PreservesMainCommand(t *testing.T) {
	mainCmd := []string{"claude", "--prompt", "complex prompt with spaces"}
	hooks := types.LifecycleHooks{
		PostStart: []string{"echo", "ready"},
	}

	result := WrapCommandWithHooks(mainCmd, hooks, false)

	shellCmd := result[2]
	// Main command should be at the end
	assert.True(t, strings.HasSuffix(shellCmd, "'claude' '--prompt' 'complex prompt with spaces'"))
}

// =============================================================================
// Validation Tests
// =============================================================================

// TestValidateLifecycleHooks tests hook validation
func TestValidateLifecycleHooks(t *testing.T) {
	tests := []struct {
		name    string
		hooks   types.LifecycleHooks
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty hooks - valid",
			hooks:   types.LifecycleHooks{},
			wantErr: false,
		},
		{
			name: "simple hooks - valid",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"pip", "install"},
				PostStart:       []string{"echo", "ready"},
			},
			wantErr: false,
		},
		{
			name: "too many arguments in onCreateCommand",
			hooks: types.LifecycleHooks{
				OnCreateCommand: makeArgs(101),
			},
			wantErr: true,
			errMsg:  "too many arguments",
		},
		{
			name: "too many arguments in postCreate",
			hooks: types.LifecycleHooks{
				PostCreate: makeArgs(101),
			},
			wantErr: true,
			errMsg:  "too many arguments",
		},
		{
			name: "too many arguments in postStart",
			hooks: types.LifecycleHooks{
				PostStart: makeArgs(101),
			},
			wantErr: true,
			errMsg:  "too many arguments",
		},
		{
			name: "argument too long",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{strings.Repeat("a", 4097)},
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "total length exceeded",
			hooks: types.LifecycleHooks{
				// Create hooks that together exceed MaxTotalHookLength (65536)
				// Each arg is under 4096, but combined they exceed 65536
				OnCreateCommand: func() []string {
					// 20 args of 2000 chars each = 40000
					args := make([]string, 20)
					for i := range args {
						args[i] = strings.Repeat("a", 2000)
					}
					return args
				}(),
				PostCreate: func() []string {
					// 20 args of 2000 chars each = 40000
					// Total: 80000 > 65536
					args := make([]string, 20)
					for i := range args {
						args[i] = strings.Repeat("b", 2000)
					}
					return args
				}(),
			},
			wantErr: true,
			errMsg:  "total hook length",
		},
		{
			name: "at the limit - valid",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{strings.Repeat("a", 4096)},
			},
			wantErr: false,
		},
		{
			name: "max arguments - valid",
			hooks: types.LifecycleHooks{
				OnCreateCommand: makeArgs(100),
			},
			wantErr: false,
		},
		{
			name: "empty argument in onCreateCommand",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"pip", "", "install"},
			},
			wantErr: true,
			errMsg:  "is empty",
		},
		{
			name: "empty argument in postCreate",
			hooks: types.LifecycleHooks{
				PostCreate: []string{""},
			},
			wantErr: true,
			errMsg:  "is empty",
		},
		{
			name: "null byte in onCreateCommand",
			hooks: types.LifecycleHooks{
				OnCreateCommand: []string{"cmd", "arg\x00malicious"},
			},
			wantErr: true,
			errMsg:  "contains null byte",
		},
		{
			name: "null byte in postCreate",
			hooks: types.LifecycleHooks{
				PostCreate: []string{"\x00bad"},
			},
			wantErr: true,
			errMsg:  "contains null byte",
		},
		{
			name: "null byte in postStart",
			hooks: types.LifecycleHooks{
				PostStart: []string{"echo", "test\x00"},
			},
			wantErr: true,
			errMsg:  "contains null byte",
		},
		{
			name: "total arguments exceeded",
			hooks: types.LifecycleHooks{
				// Total = 100 + 100 + 50 = 250 > 200 (MaxTotalHookArgs)
				OnCreateCommand: makeArgs(100),
				PostCreate:      makeArgs(100),
				PostStart:       makeArgs(50),
			},
			wantErr: true,
			errMsg:  "total hook arguments",
		},
		{
			name: "total arguments at limit - valid",
			hooks: types.LifecycleHooks{
				// Total = 200 (MaxTotalHookArgs)
				OnCreateCommand: makeArgs(100),
				PostCreate:      makeArgs(50),
				PostStart:       makeArgs(50),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLifecycleHooks(tt.hooks)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEscapeShellArg tests the exported helper function
func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		expected string
	}{
		{
			name:     "simple arg",
			arg:      "hello",
			expected: "'hello'",
		},
		{
			name:     "arg with spaces",
			arg:      "hello world",
			expected: "'hello world'",
		},
		{
			name:     "arg with single quote",
			arg:      "it's",
			expected: "'it'\\''s'",
		},
		{
			name:     "arg with special chars",
			arg:      "$HOME && rm -rf /",
			expected: "'$HOME && rm -rf /'",
		},
		{
			name:     "empty arg",
			arg:      "",
			expected: "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeShellArg(tt.arg)
			assert.Equal(t, tt.expected, result)
		})
	}
}
