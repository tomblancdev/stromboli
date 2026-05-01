package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"stromboli/internal/types"
)

// containsArg returns true if want appears as a contiguous run in args.
// Avoids false positives when a value happens to equal a flag name.
func containsArg(args []string, want ...string) bool {
	if len(want) == 0 {
		return true
	}
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

func TestApplyOptions_EmptyOptionsAddsNothing(t *testing.T) {
	// Build the same builder twice: once with no ApplyOptions call, once
	// with an empty struct passed through. The two outputs must match —
	// empty / zero-value fields must not contribute any flags.
	bare := NewCommandBuilder().Build()

	b := NewCommandBuilder()
	ApplyOptions(b, types.ClaudeOptions{})
	withEmptyOpts := b.Build()

	assert.Equal(t, bare, withEmptyOpts,
		"empty options must produce the same argv as no ApplyOptions call")
}

func TestApplyOptions_ModelEffortAndPermissions(t *testing.T) {
	b := NewCommandBuilder()
	ApplyOptions(b, types.ClaudeOptions{
		Model:                      "sonnet",
		Effort:                     "high",
		DangerouslySkipPermissions: true,
	})
	args := b.Build()

	assert.True(t, containsArg(args, "--model", "sonnet"))
	assert.True(t, containsArg(args, "--effort", "high"))
	assert.True(t, containsArg(args, "--dangerously-skip-permissions"))
}

func TestApplyOptions_ToolsLists(t *testing.T) {
	b := NewCommandBuilder()
	ApplyOptions(b, types.ClaudeOptions{
		AllowedTools:    []string{"Read", "Bash"},
		DisallowedTools: []string{"Write"},
	})
	args := b.Build()

	// AllowedTools is comma-joined into one --allowedTools value by the builder.
	assert.True(t, containsArg(args, "--allowedTools"), "expected --allowedTools flag, got %v", args)
	assert.True(t, containsArg(args, "--disallowedTools"), "expected --disallowedTools flag, got %v", args)
}

func TestApplyOptions_SessionIDIsNotApplied(t *testing.T) {
	// SessionID is the caller's responsibility (new vs resume distinction).
	b := NewCommandBuilder()
	ApplyOptions(b, types.ClaudeOptions{
		SessionID: "abc123",
	})
	args := b.Build()
	assert.False(t, containsArg(args, "--session-id"),
		"ApplyOptions must NOT set --session-id; the caller controls session lifecycle")
}

func TestApplyOptions_BudgetAndTurnsRespectPointers(t *testing.T) {
	budget := 5.5
	turns := 30
	b := NewCommandBuilder()
	ApplyOptions(b, types.ClaudeOptions{
		MaxBudgetUSD: &budget,
		MaxTurns:     &turns,
	})
	args := b.Build()
	assert.True(t, containsArg(args, "--max-budget-usd", "5.50"))
	assert.True(t, containsArg(args, "--max-turns", "30"))
}

func TestEnvVars_PromptCachingTTL(t *testing.T) {
	cases := []struct {
		ttl      string
		wantKV   string
		wantNone bool
	}{
		{"5m", "FORCE_PROMPT_CACHING_5M=1", false},
		{"1h", "ENABLE_PROMPT_CACHING_1H=1", false},
		{"", "", true},
		{"30m", "", true}, // only 5m and 1h are recognised; anything else is ignored
	}
	for _, tc := range cases {
		t.Run(tc.ttl, func(t *testing.T) {
			got := EnvVars(types.ClaudeOptions{PromptCachingTTL: tc.ttl})
			if tc.wantNone {
				assert.Empty(t, got, "unrecognised TTL %q must contribute no env vars", tc.ttl)
				return
			}
			assert.Equal(t, []string{"-e", tc.wantKV}, got)
		})
	}
}

func TestEnvVars_BedrockServiceTier(t *testing.T) {
	got := EnvVars(types.ClaudeOptions{BedrockServiceTier: "priority"})
	assert.Equal(t, []string{"-e", "ANTHROPIC_BEDROCK_SERVICE_TIER=priority"}, got)
}

func TestEnvVars_PowerShellTool(t *testing.T) {
	got := EnvVars(types.ClaudeOptions{EnablePowerShellTool: true})
	assert.Equal(t, []string{"-e", "CLAUDE_CODE_USE_POWERSHELL_TOOL=1"}, got)
}

func TestEnvVars_AllAtOnce(t *testing.T) {
	got := EnvVars(types.ClaudeOptions{
		PromptCachingTTL:     "1h",
		BedrockServiceTier:   "flex",
		EnablePowerShellTool: true,
	})
	assert.Equal(t, []string{
		"-e", "ENABLE_PROMPT_CACHING_1H=1",
		"-e", "ANTHROPIC_BEDROCK_SERVICE_TIER=flex",
		"-e", "CLAUDE_CODE_USE_POWERSHELL_TOOL=1",
	}, got)
}

func TestEnvVars_EmptyReturnsEmptySlice(t *testing.T) {
	got := EnvVars(types.ClaudeOptions{})
	assert.Empty(t, got)
}
