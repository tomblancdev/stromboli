package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/agent"
	"stromboli/internal/types"
)

// indexOf returns the position of want in args, or -1 if not present.
func indexOf(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

// hasContiguous reports whether the given subsequence appears in args.
func hasContiguous(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j, w := range want {
			if args[i+j] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// findArgvImage returns the index of the agent image in argv. The image is
// the first non-flag arg after `podman run` and our flag block; identifiers
// of the form ghcr.io/... or local-name:tag are matched here.
func findArgvImage(args []string, image string) int {
	for i, a := range args {
		if a == image {
			return i
		}
	}
	return -1
}

func TestBuildAgentArgv_RejectsCannotCreateSessionDir(t *testing.T) {
	// sessionsDir under a path that exists as a regular file → MkdirAll fails.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "iam-a-file")
	require.NoError(t, os.WriteFile(blocker, []byte{}, 0o600))

	build := buildAgentArgv("img:latest", "/tmp/creds", filepath.Join(blocker, "sessions"))
	_, err := build(agent.CreateRequest{}, "agent-x", "session-y")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create session dir")
}

func TestBuildAgentArgv_BasicShape(t *testing.T) {
	sessions := t.TempDir()
	build := buildAgentArgv("ghcr.io/example/stromboli-agent:test", "/host/creds.json", sessions)

	args, err := build(agent.CreateRequest{}, "agent-abc", "8e29f4d2-aaaa-bbbb-cccc-1234567890ab")
	require.NoError(t, err)

	// Bug #1 regression: session dir must be created on disk.
	sessionDir := filepath.Join(sessions, "8e29f4d2-aaaa-bbbb-cccc-1234567890ab")
	info, statErr := os.Stat(sessionDir)
	require.NoError(t, statErr, "buildAgentArgv must pre-create the session subdirectory")
	assert.True(t, info.IsDir())

	// Bug #3 regression: "claude" must be prepended to the in-container args
	// (the agent image's entrypoint defaults to node).
	assert.Contains(t, args, "claude", "argv must name claude as the in-container binary")

	// Bug #4 regression: --userns=keep-id and --user $UID:$GID are required
	// or claude refuses dangerously-skip-permissions for the root container user.
	assert.Contains(t, args, "--userns=keep-id")
	userIdx := indexOf(args, "--user")
	require.GreaterOrEqual(t, userIdx, 0, "--user flag must be present")
	require.Less(t, userIdx, len(args)-1, "--user must have a value")
	assert.Regexp(t, `^\d+:\d+$`, args[userIdx+1], "--user must be UID:GID, got %q", args[userIdx+1])

	// Bug #5 regression: stream-json output requires --verbose; without it
	// claude exits 1 on the first turn.
	assert.Contains(t, args, "--verbose")

	// Stream-json contract that the dispatcher relies on; both directions.
	assert.True(t, hasContiguous(args, "--input-format", "stream-json"))
	assert.True(t, hasContiguous(args, "--output-format", "stream-json"))
	assert.Contains(t, args, "--include-partial-messages")

	// Session ID must thread through to the claude flag.
	assert.True(t, hasContiguous(args, "--session-id", "8e29f4d2-aaaa-bbbb-cccc-1234567890ab"))
}

func TestBuildAgentArgv_ThreadsClaudeOptionsThrough(t *testing.T) {
	// Bug #6 regression: req.Claude was parsed but ignored. Operators sending
	// `claude.model` to /agents got the CLI's default model instead.
	sessions := t.TempDir()
	build := buildAgentArgv("img:latest", "/creds", sessions)

	args, err := build(agent.CreateRequest{
		Claude: &types.ClaudeOptions{
			Model:                      "sonnet",
			Effort:                     "high",
			SystemPrompt:               "Custom system prompt",
			AllowedTools:               []string{"Read", "Bash"},
			DangerouslySkipPermissions: true,
		},
	}, "agent-x", "00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)

	assert.True(t, hasContiguous(args, "--model", "sonnet"), "argv=%v", args)
	assert.True(t, hasContiguous(args, "--effort", "high"))
	assert.True(t, hasContiguous(args, "--system-prompt", "Custom system prompt"))
	assert.Contains(t, args, "--dangerously-skip-permissions")
}

func TestBuildAgentArgv_ThreadsEnvVarOnlyOptions(t *testing.T) {
	// Bug #6 regression continued: env-var-only options
	// (prompt_caching_ttl, bedrock_service_tier, enable_powershell_tool)
	// were dropped on the floor for /agents. Verify they reach podman -e.
	sessions := t.TempDir()
	build := buildAgentArgv("img:latest", "/creds", sessions)

	args, err := build(agent.CreateRequest{
		Claude: &types.ClaudeOptions{
			PromptCachingTTL:     "1h",
			BedrockServiceTier:   "priority",
			EnablePowerShellTool: true,
		},
	}, "agent-x", "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)

	assert.True(t, hasContiguous(args, "-e", "ENABLE_PROMPT_CACHING_1H=1"))
	assert.True(t, hasContiguous(args, "-e", "ANTHROPIC_BEDROCK_SERVICE_TIER=priority"))
	assert.True(t, hasContiguous(args, "-e", "CLAUDE_CODE_USE_POWERSHELL_TOOL=1"))
}

func TestBuildAgentArgv_ForcesStreamJsonEvenIfUserOverrides(t *testing.T) {
	// The dispatcher in internal/agent reads stdout one JSON-line per event;
	// flipping output to "text" or "json" would silently break every
	// subscriber. The agent always pins these regardless of the user's input.
	sessions := t.TempDir()
	build := buildAgentArgv("img:latest", "/creds", sessions)

	args, err := build(agent.CreateRequest{
		Claude: &types.ClaudeOptions{
			InputFormat:  "text",
			OutputFormat: "json",
		},
	}, "agent-x", "00000000-0000-0000-0000-000000000003")
	require.NoError(t, err)

	// Last value of each flag wins on the claude CLI; we re-set them after
	// ApplyOptions so user values can't override.
	// Count occurrences and ensure the LAST one is stream-json.
	lastInputFormat := ""
	lastOutputFormat := ""
	for i, a := range args {
		if i+1 >= len(args) {
			break
		}
		if a == "--input-format" {
			lastInputFormat = args[i+1]
		}
		if a == "--output-format" {
			lastOutputFormat = args[i+1]
		}
	}
	assert.Equal(t, "stream-json", lastInputFormat,
		"last --input-format value wins; agent must pin stream-json")
	assert.Equal(t, "stream-json", lastOutputFormat,
		"last --output-format value wins; agent must pin stream-json")
}

func TestBuildAgentArgv_WorkdirIsPodmanDashW(t *testing.T) {
	// -w is a podman flag, NOT a claude flag — it sets the container's
	// working directory. Used to be a sed-style insert "before the image";
	// now appended explicitly so order isn't fragile.
	sessions := t.TempDir()
	build := buildAgentArgv("ghcr.io/example/stromboli-agent:test", "/creds", sessions)

	args, err := build(agent.CreateRequest{Workdir: "/workspace"}, "agent-x", "00000000-0000-0000-0000-000000000004")
	require.NoError(t, err)

	wIdx := indexOf(args, "-w")
	require.GreaterOrEqual(t, wIdx, 0, "-w must be present when Workdir is set")
	assert.Equal(t, "/workspace", args[wIdx+1])

	// -w must come BEFORE the image name (it's a podman flag).
	imgIdx := findArgvImage(args, "ghcr.io/example/stromboli-agent:test")
	require.GreaterOrEqual(t, imgIdx, 0, "image must appear in argv")
	assert.Less(t, wIdx, imgIdx, "-w is a podman flag and must precede the image")
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
		ok   bool
	}{
		{"", slog.LevelInfo, true},
		{"info", slog.LevelInfo, true},
		{"INFO", slog.LevelInfo, true},
		{"  Info  ", slog.LevelInfo, true},
		{"debug", slog.LevelDebug, true},
		{"warn", slog.LevelWarn, true},
		{"warning", slog.LevelWarn, true},
		{"error", slog.LevelError, true},
		{"trace", slog.LevelInfo, false},
		{"verbose", slog.LevelInfo, false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := parseLogLevel(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("parseLogLevel(%q) = (%v, %v), want (%v, %v)",
					tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}
