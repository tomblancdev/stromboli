package api

// HealthResponse represents the health check response
// @Description Health check response
type HealthResponse struct {
	Status string `json:"status" example:"ok"`
	Name   string `json:"name" example:"stromboli"`
}

// ClaudeStatusResponse represents the Claude status response
// @Description Claude configuration status
type ClaudeStatusResponse struct {
	Configured bool   `json:"configured" example:"true"`
	Message    string `json:"message" example:"Claude is configured"`
}

// RunRequest represents a request to run Claude in a container
// @Description Request to execute Claude Code in an isolated container
type RunRequest struct {
	// Required: the prompt to send to Claude
	Prompt string `json:"prompt" binding:"required" example:"Analyze this code and suggest improvements"`

	// Workspace to mount (host path -> /workspace in container)
	Workspace string `json:"workspace,omitempty" example:"/home/user/project"`

	// Claude configuration - all CLI options exposed
	Claude ClaudeOptions `json:"claude,omitempty"`

	// Podman configuration
	Podman PodmanOptions `json:"podman,omitempty"`
}

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

	// --- Budget Control ---

	// Maximum dollar amount for API calls
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty" example:"5.00"`

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

	// --- Misc ---

	// Enable verbose mode
	Verbose bool `json:"verbose,omitempty" example:"false"`
	// Debug mode with optional category filter
	Debug string `json:"debug,omitempty" example:"api,hooks"`
	// Disable all slash commands/skills
	DisableSlashCommands bool `json:"disable_slash_commands,omitempty" example:"false"`
}

// PodmanOptions contains Podman container configuration
// @Description Podman container mount configuration
type PodmanOptions struct {
	// Volume mounts (host:container or host:container:options format)
	Volumes []string `json:"volumes,omitempty" example:"/data:/data:ro"`
}

// RunResponse represents the response from a Claude run
// @Description Response from Claude execution
type RunResponse struct {
	// Unique run identifier
	ID string `json:"id" example:"run-abc123def456"`
	// Execution status: completed, error
	Status string `json:"status" example:"completed"`
	// Claude's output (when successful)
	Output string `json:"output,omitempty" example:"Here is my analysis..."`
	// Error message (when failed)
	Error string `json:"error,omitempty" example:""`
	// Session ID for conversation continuation
	SessionID string `json:"session_id,omitempty" example:"sess-abc123def456"`
}

// SessionListResponse represents the response from listing sessions
// @Description List of existing sessions
type SessionListResponse struct {
	Sessions []string `json:"sessions" example:"sess-abc123,sess-def456"`
	Error    string   `json:"error,omitempty"`
}

// SessionDestroyResponse represents the response from destroying a session
// @Description Result of session destruction
type SessionDestroyResponse struct {
	Success   bool   `json:"success" example:"true"`
	SessionID string `json:"session_id,omitempty" example:"sess-abc123"`
	Error     string `json:"error,omitempty"`
}
