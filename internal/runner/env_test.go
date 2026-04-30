package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"stromboli/internal/podman"
	"stromboli/internal/types"
)

func TestApplyClaudeEnvVars_PromptCachingTTL(t *testing.T) {
	cases := map[string]struct {
		ttl    string
		envKey string
		envSet bool
	}{
		"1h":                             {ttl: "1h", envKey: "ENABLE_PROMPT_CACHING_1H", envSet: true},
		"5m":                             {ttl: "5m", envKey: "FORCE_PROMPT_CACHING_5M", envSet: true},
		"unset/default":                  {ttl: "", envKey: "ENABLE_PROMPT_CACHING_1H", envSet: false},
		"unknown value silently ignored": {ttl: "30m", envKey: "ENABLE_PROMPT_CACHING_1H", envSet: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := podman.NewCommand().WithImage("test-image")
			applyClaudeEnvVars(b, types.ClaudeOptions{PromptCachingTTL: tc.ttl})
			cmd := strings.Join(b.Build(), " ")
			if tc.envSet {
				assert.Contains(t, cmd, "-e "+tc.envKey+"=1")
			} else {
				assert.NotContains(t, cmd, tc.envKey)
			}
		})
	}
}

func TestApplyClaudeEnvVars_BedrockServiceTier(t *testing.T) {
	b := podman.NewCommand().WithImage("test-image")
	applyClaudeEnvVars(b, types.ClaudeOptions{BedrockServiceTier: "priority"})
	cmd := strings.Join(b.Build(), " ")
	assert.Contains(t, cmd, "-e ANTHROPIC_BEDROCK_SERVICE_TIER=priority")
}

func TestApplyClaudeEnvVars_PowerShellTool(t *testing.T) {
	b := podman.NewCommand().WithImage("test-image")
	applyClaudeEnvVars(b, types.ClaudeOptions{EnablePowerShellTool: true})
	cmd := strings.Join(b.Build(), " ")
	assert.Contains(t, cmd, "-e CLAUDE_CODE_USE_POWERSHELL_TOOL=1")
}

func TestApplyClaudeEnvVars_AllOff(t *testing.T) {
	b := podman.NewCommand().WithImage("test-image")
	applyClaudeEnvVars(b, types.ClaudeOptions{})
	cmd := strings.Join(b.Build(), " ")
	assert.NotContains(t, cmd, "ENABLE_PROMPT_CACHING_1H")
	assert.NotContains(t, cmd, "FORCE_PROMPT_CACHING_5M")
	assert.NotContains(t, cmd, "ANTHROPIC_BEDROCK_SERVICE_TIER")
	assert.NotContains(t, cmd, "CLAUDE_CODE_USE_POWERSHELL_TOOL")
}
