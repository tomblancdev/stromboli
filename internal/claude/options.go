package claude

import (
	"stromboli/internal/types"
)

// ApplyOptions threads a fully-typed types.ClaudeOptions struct onto a
// CommandBuilder. Empty / zero-value fields leave the builder untouched, so
// callers can chain caller-specific overrides before or after this call.
//
// The split of responsibilities:
//
//   - This function handles every option that maps to a `--flag` argument.
//   - EnvVars (below) handles the env-var-only options
//     (prompt_caching_ttl, bedrock_service_tier, enable_powershell_tool)
//     because those are passed via `podman -e KEY=VALUE`, not on the
//     claude command line.
//
// Both runner.PodmanRunner (for /run, /run/async, /run/stream) and
// main.buildAgentArgv (for /agents) call into these so the surface stays
// consistent across the two execution paths. The previous duplication —
// where /agents simply ignored req.Claude — is what shipped untested in
// v0.5.0; consolidating here is the regression net.
//
// SessionID is intentionally NOT applied here: callers control session
// lifecycle and decide whether to use --session-id (new) or --resume
// (existing) via the surrounding context.
func ApplyOptions(b *CommandBuilder, opts types.ClaudeOptions) *CommandBuilder {
	applySessionOptions(b, opts)
	applyModelOptions(b, opts)
	applyPromptOptions(b, opts)
	applyToolsOptions(b, opts)
	applyPermissionOptions(b, opts)
	applyIOOptions(b, opts)
	applyResourceOptions(b, opts)
	applyMiscOptions(b, opts)
	return b
}

// EnvVars returns the podman `-e KEY=VALUE` flag pairs for the env-var-only
// Claude options (prompt cache TTL, Bedrock service tier, PowerShell tool).
// Empty / zero-value fields contribute nothing. Returns a fresh slice every
// call; safe for the caller to append onto.
func EnvVars(opts types.ClaudeOptions) []string {
	out := make([]string, 0, 6)
	switch opts.PromptCachingTTL {
	case "1h":
		out = append(out, "-e", "ENABLE_PROMPT_CACHING_1H=1")
	case "5m":
		out = append(out, "-e", "FORCE_PROMPT_CACHING_5M=1")
	}
	if opts.BedrockServiceTier != "" {
		out = append(out, "-e", "ANTHROPIC_BEDROCK_SERVICE_TIER="+opts.BedrockServiceTier)
	}
	if opts.EnablePowerShellTool {
		out = append(out, "-e", "CLAUDE_CODE_USE_POWERSHELL_TOOL=1")
	}
	return out
}

// --- internal sub-appliers (one per option group; lifted verbatim from the
// runner package for traceability — there's no behavioral change here).

func applySessionOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.Continue {
		b.WithContinue()
	}
	if opts.ForkSession {
		b.WithForkSession()
	}
	if opts.NoPersistence {
		b.WithNoSessionPersistence()
	}
}

func applyModelOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.Model != "" {
		b.WithModel(opts.Model)
	}
	if opts.FallbackModel != "" {
		b.WithFallbackModel(opts.FallbackModel)
	}
	if opts.Effort != "" {
		b.WithEffort(opts.Effort)
	}
}

func applyPromptOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.SystemPrompt != "" {
		b.WithSystemPrompt(opts.SystemPrompt)
	}
	if opts.AppendSystemPrompt != "" {
		b.WithAppendSystemPrompt(opts.AppendSystemPrompt)
	}
}

func applyToolsOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if len(opts.Tools) > 0 {
		b.WithTools(opts.Tools...)
	}
	if len(opts.AllowedTools) > 0 {
		b.WithAllowedTools(opts.AllowedTools...)
	}
	if len(opts.DisallowedTools) > 0 {
		b.WithDisallowedTools(opts.DisallowedTools...)
	}
}

func applyPermissionOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.PermissionMode != "" {
		b.WithPermissionMode(opts.PermissionMode)
	}
	if opts.DangerouslySkipPermissions {
		b.WithDangerouslySkipPermissions()
	}
	if opts.AllowDangerouslySkipPermissions {
		b.WithAllowDangerouslySkipPermissions()
	}
}

func applyIOOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.InputFormat != "" {
		b.WithInputFormat(opts.InputFormat)
	}
	if opts.OutputFormat != "" {
		b.WithOutputFormat(opts.OutputFormat)
	}
	if opts.IncludePartialMessages {
		b.WithIncludePartialMessages()
	}
	if opts.ReplayUserMessages {
		b.WithReplayUserMessages()
	}
	if opts.JSONSchema != "" {
		b.WithJSONSchema(opts.JSONSchema)
	}
}

func applyResourceOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.MaxBudgetUSD != nil {
		b.WithMaxBudgetUSD(*opts.MaxBudgetUSD)
	}
	if opts.MaxTurns != nil {
		b.WithMaxTurns(*opts.MaxTurns)
	}
	if len(opts.MCPConfigs) > 0 {
		b.WithMCPConfig(opts.MCPConfigs...)
	}
	if opts.StrictMCPConfig {
		b.WithStrictMCPConfig()
	}
	if opts.Agent != "" {
		b.WithAgent(opts.Agent)
	}
	if opts.Agents != nil {
		b.WithAgents(opts.Agents)
	}
	if len(opts.AddDirs) > 0 {
		b.WithAddDir(opts.AddDirs...)
	}
	if len(opts.PluginDirs) > 0 {
		b.WithPluginDir(opts.PluginDirs...)
	}
	if len(opts.Files) > 0 {
		b.WithFiles(opts.Files...)
	}
	if opts.Settings != "" {
		b.WithSettings(opts.Settings)
	}
	if len(opts.SettingSources) > 0 {
		b.WithSettingSources(opts.SettingSources...)
	}
	if len(opts.Betas) > 0 {
		b.WithBetas(opts.Betas...)
	}
}

func applyMiscOptions(b *CommandBuilder, opts types.ClaudeOptions) {
	if opts.Verbose {
		b.WithVerbose()
	}
	if opts.Debug != "" {
		b.WithDebug(opts.Debug)
	}
	if opts.DisableSlashCommands {
		b.WithDisableSlashCommands()
	}
}
