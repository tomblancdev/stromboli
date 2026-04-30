package types

// ClaudeOptions contains all Claude CLI headless mode options
// @Description All available Claude CLI options for headless execution
type ClaudeOptions struct {
	// --- Session Management ---

	// Session ID (UUID) - used for both new and resumed sessions
	SessionID string `json:"session_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Resume an existing session (requires session_id)
	Resume bool `json:"resume,omitempty" example:"true"`
	// Continue most recent conversation in workspace (ignores session_id)
	Continue bool `json:"continue,omitempty" example:"false"`
	// Create new session ID when resuming
	ForkSession bool `json:"fork_session,omitempty" example:"false"`
	// Don't save session to disk
	NoPersistence bool `json:"no_persistence,omitempty" example:"false"`

	// --- Model Configuration ---

	// Model alias (sonnet, opus, haiku) or full name
	Model string `json:"model,omitempty" example:"sonnet"`
	// Fallback model when default is overloaded
	FallbackModel string `json:"fallback_model,omitempty" example:"haiku"`
	// Effort level for thinking/agentic complexity. Valid values per the
	// upstream CLI reference: low, medium, high, xhigh, max. The accepted
	// subset depends on the model — Claude rejects unsupported levels at
	// runtime, so Stromboli does not gate on an enum here.
	Effort string `json:"effort,omitempty" example:"high"`

	// --- System Prompt ---

	// Replace default system prompt
	SystemPrompt string `json:"system_prompt,omitempty" example:"You are a senior Go developer"`
	// Append to default system prompt
	AppendSystemPrompt string `json:"append_system_prompt,omitempty" example:"Focus on security best practices"`

	// --- Tools Configuration ---

	// Built-in tools ("", "default", or specific names)
	Tools []string `json:"tools,omitempty" example:"Bash,Read,Edit"`
	// Allowed tools with patterns (e.g., "Bash(git:*)")
	AllowedTools []string `json:"allowed_tools,omitempty" example:"Bash(git:*),Read"`
	// Denied tools
	DisallowedTools []string `json:"disallowed_tools,omitempty" example:"Write"`

	// --- Permissions ---

	// Permission mode: acceptEdits, bypassPermissions, default, delegate, dontAsk, plan
	PermissionMode string `json:"permission_mode,omitempty" example:"bypassPermissions"`
	// Bypass all permission checks (use in sandboxed environments only)
	DangerouslySkipPermissions bool `json:"dangerously_skip_permissions,omitempty" example:"true"`
	// Enable bypass as an option without enabling by default
	AllowDangerouslySkipPermissions bool `json:"allow_dangerously_skip_permissions,omitempty" example:"false"`

	// --- Input/Output Format ---

	// Input format: text, stream-json
	InputFormat string `json:"input_format,omitempty" example:"text"`
	// Output format: text, json, stream-json
	OutputFormat string `json:"output_format,omitempty" example:"json"`
	// Include partial message chunks (stream-json only)
	IncludePartialMessages bool `json:"include_partial_messages,omitempty" example:"false"`
	// Re-emit user messages on stdout
	ReplayUserMessages bool `json:"replay_user_messages,omitempty" example:"false"`

	// --- Structured Output ---

	// JSON Schema for structured output validation
	JSONSchema string `json:"json_schema,omitempty" example:"{\"type\":\"object\"}"`

	// --- Budget & Limits ---

	// Maximum dollar amount for API calls (pointer to distinguish 0 from unset)
	MaxBudgetUSD *float64 `json:"max_budget_usd,omitempty" example:"5.00"`
	// Maximum number of agentic turns (0 = unlimited, nil = use CLI default)
	MaxTurns *int `json:"max_turns,omitempty" example:"30"`

	// --- MCP Configuration ---

	// MCP server config files or JSON strings
	MCPConfigs []string `json:"mcp_configs,omitempty"`
	// Only use MCP servers from mcp_configs
	StrictMCPConfig bool `json:"strict_mcp_config,omitempty" example:"false"`

	// --- Agents ---

	// Agent for current session
	Agent string `json:"agent,omitempty" example:"reviewer"`
	// Custom agents definition (JSON object)
	Agents map[string]any `json:"agents,omitempty"`

	// --- Additional Resources ---

	// Additional directories for tool access
	AddDirs []string `json:"add_dirs,omitempty"`
	// Plugin directories
	PluginDirs []string `json:"plugin_dirs,omitempty"`
	// File resources (format: file_id:path)
	Files []string `json:"files,omitempty"`

	// --- Settings ---

	// Path to settings JSON file or JSON string
	Settings string `json:"settings,omitempty"`
	// Setting sources to load: user, project, local
	SettingSources []string `json:"setting_sources,omitempty" example:"user,project"`

	// --- Beta Features ---

	// Beta headers for API requests
	Betas []string `json:"betas,omitempty"`

	// --- Provider / Runtime Tuning (env-var-backed) ---

	// Prompt cache TTL. "" leaves Claude's default; "5m" sets
	// FORCE_PROMPT_CACHING_5M=1; "1h" sets ENABLE_PROMPT_CACHING_1H=1.
	// Useful for long-lived agent loops where the same context is re-used
	// across many turns. (#76)
	PromptCachingTTL string `json:"prompt_caching_ttl,omitempty" example:"1h"`

	// Bedrock service tier when running against AWS Bedrock as the model
	// backend. Valid values: "default", "flex", "priority". Translates to
	// ANTHROPIC_BEDROCK_SERVICE_TIER. Ignored when not on Bedrock. (#77)
	BedrockServiceTier string `json:"bedrock_service_tier,omitempty" example:"priority"`

	// Opt into Claude Code's native PowerShell tool on Windows agent hosts.
	// Translates to CLAUDE_CODE_USE_POWERSHELL_TOOL=1. No-op on non-Windows
	// containers. (#81)
	EnablePowerShellTool bool `json:"enable_powershell_tool,omitempty" example:"false"`

	// --- Misc ---

	// Enable verbose mode
	Verbose bool `json:"verbose,omitempty" example:"false"`
	// Debug mode with optional category filter
	Debug string `json:"debug,omitempty" example:"api,hooks"`
	// Disable all slash commands/skills
	DisableSlashCommands bool `json:"disable_slash_commands,omitempty" example:"false"`
}

// LifecycleHooks configures commands to run at container lifecycle events.
//
// Hook Execution Order:
//  1. OnCreateCommand - runs once when session is first created
//  2. PostCreate - runs once after OnCreateCommand completes
//  3. PostStart - runs on every container start
//
// IMPORTANT - Idempotency Requirement:
// Init hooks (OnCreateCommand, PostCreate) should be idempotent since they
// may run again if the first attempt crashes before completion. Design hooks
// to be safe to re-run (e.g., use "pip install" not "pip install && rm cache").
//
// All hooks are chained with && for fail-fast behavior. If any hook fails,
// subsequent hooks and the main command will not run.
//
// @Description Commands to run at specific container lifecycle stages
type LifecycleHooks struct {
	// OnCreateCommand runs after container creation, before Claude starts (first run only).
	// Each element is a complete shell command string (devcontainer-style).
	// Example: ["apt-get update && apt-get install -y git", "pip install -r requirements.txt"]
	OnCreateCommand []string `json:"on_create_command,omitempty" example:"pip install -r requirements.txt"`

	// PostCreate runs after OnCreateCommand completes (first run only).
	// Each element is a complete shell command string (devcontainer-style).
	PostCreate []string `json:"post_create,omitempty" example:"npm run setup"`

	// PostStart runs after container starts (every run, including continues).
	// Each element is a complete shell command string (devcontainer-style).
	PostStart []string `json:"post_start,omitempty" example:"redis-server --daemonize yes"`

	// HooksTimeout is the maximum duration for all hooks combined (e.g., "5m", "30s").
	// If not specified, hooks run with the container's timeout.
	// This is useful to prevent long-running hooks from blocking the main command.
	HooksTimeout string `json:"hooks_timeout,omitempty" example:"5m"`
}

// EnvironmentConfig specifies the runtime environment for Claude execution.
// When type is "compose", the agent runs in a multi-service environment
// defined by a Docker/Podman Compose file.
//
// @Description Runtime environment configuration (single container or compose)
type EnvironmentConfig struct {
	// Type of environment: "" (default single container) or "compose"
	Type string `json:"type,omitempty" example:"compose"`

	// Path to compose file (required when type="compose")
	// Must be an absolute path ending in .yml or .yaml
	Path string `json:"path,omitempty" example:"/home/user/project/docker-compose.yml"`

	// Service name where Claude will run (required when type="compose")
	Service string `json:"service,omitempty" example:"dev"`

	// Optional build timeout override for compose (e.g., "15m")
	// If not specified, uses server default (10m)
	BuildTimeout string `json:"build_timeout,omitempty" example:"15m"`
}

// PodmanOptions contains Podman container configuration
// @Description Podman container mount configuration
type PodmanOptions struct {
	// Container image override (must match allowed patterns)
	Image string `json:"image,omitempty" example:"python:3.12"`

	// Volume mounts (host:container or host:container:options format)
	Volumes []string `json:"volumes,omitempty" example:"/data:/data:ro"`

	// Secrets to inject as environment variables (env_var_name -> podman_secret_name)
	// The Podman secret must exist beforehand (created via `podman secret create`)
	// Example: {"GH_TOKEN": "github-token"} mounts secret "github-token" as env var GH_TOKEN
	SecretsEnv map[string]string `json:"secrets_env,omitempty"`

	// Container timeout (e.g., "5m", "1h", "30s")
	Timeout string `json:"timeout,omitempty" example:"5m"`

	// Memory limit (e.g., "512m", "1g")
	Memory string `json:"memory,omitempty" example:"512m"`

	// CPU limit (e.g., "0.5", "2")
	CPUs string `json:"cpus,omitempty" example:"1"`

	// CPU shares (relative weight, default 1024; pointer to distinguish 0 from unset)
	CPUShares *int `json:"cpu_shares,omitempty" example:"512"`

	// Lifecycle hooks for running commands at specific container events
	Lifecycle LifecycleHooks `json:"lifecycle,omitempty"`

	// Environment specifies a compose-based multi-service environment.
	// When set, the agent runs inside the specified service of the compose stack
	// instead of a standalone container.
	Environment EnvironmentConfig `json:"environment,omitempty"`
}
