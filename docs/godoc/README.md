# Stromboli Code Documentation

Generated: Wed Jan 21 19:58:03 UTC 2026

## Package api

```
package api // import "github.com/tomblanc/stromboli/internal/api"


TYPES

type ClaudeOptions struct {

	// Resume by session ID (UUID)
	SessionID string `json:"session_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	// Continue most recent conversation in workspace
	Continue bool `json:"continue,omitempty" example:"false"`
	// Create new session ID when resuming
	ForkSession bool `json:"fork_session,omitempty" example:"false"`
	// Don't save session to disk
	NoPersistence bool `json:"no_persistence,omitempty" example:"false"`

	// Model alias (sonnet, opus, haiku) or full name
	Model string `json:"model,omitempty" example:"sonnet"`
	// Fallback model when default is overloaded
	FallbackModel string `json:"fallback_model,omitempty" example:"haiku"`

	// Replace default system prompt
	SystemPrompt string `json:"system_prompt,omitempty" example:"You are a senior Go developer"`
	// Append to default system prompt
	AppendSystemPrompt string `json:"append_system_prompt,omitempty" example:"Focus on security best practices"`

	// Built-in tools ("", "default", or specific names)
	Tools []string `json:"tools,omitempty" example:"Bash,Read,Edit"`
	// Allowed tools with patterns (e.g., "Bash(git:*)")
	AllowedTools []string `json:"allowed_tools,omitempty" example:"Bash(git:*),Read"`
	// Denied tools
	DisallowedTools []string `json:"disallowed_tools,omitempty" example:"Write"`

	// Permission mode: acceptEdits, bypassPermissions, default, delegate, dontAsk, plan
	PermissionMode string `json:"permission_mode,omitempty" example:"bypassPermissions"`
	// Bypass all permission checks (use in sandboxed environments only)
	DangerouslySkipPermissions bool `json:"dangerously_skip_permissions,omitempty" example:"true"`
	// Enable bypass as an option without enabling by default
	AllowDangerouslySkipPermissions bool `json:"allow_dangerously_skip_permissions,omitempty" example:"false"`

	// Input format: text, stream-json
	InputFormat string `json:"input_format,omitempty" example:"text"`
	// Output format: text, json, stream-json
	OutputFormat string `json:"output_format,omitempty" example:"json"`
	// Include partial message chunks (stream-json only)
	IncludePartialMessages bool `json:"include_partial_messages,omitempty" example:"false"`
	// Re-emit user messages on stdout
	ReplayUserMessages bool `json:"replay_user_messages,omitempty" example:"false"`

	// JSON Schema for structured output validation
	JSONSchema string `json:"json_schema,omitempty" example:"{\"type\":\"object\"}"`

	// Maximum dollar amount for API calls
	MaxBudgetUSD float64 `json:"max_budget_usd,omitempty" example:"5.00"`

	// MCP server config files or JSON strings
	MCPConfigs []string `json:"mcp_configs,omitempty"`
	// Only use MCP servers from mcp_configs
	StrictMCPConfig bool `json:"strict_mcp_config,omitempty" example:"false"`

	// Agent for current session
	Agent string `json:"agent,omitempty" example:"reviewer"`
	// Custom agents definition (JSON object)
	Agents map[string]any `json:"agents,omitempty"`

	// Additional directories for tool access
	AddDirs []string `json:"add_dirs,omitempty"`
	// Plugin directories
	PluginDirs []string `json:"plugin_dirs,omitempty"`
	// File resources (format: file_id:path)
	Files []string `json:"files,omitempty"`

	// Path to settings JSON file or JSON string
	Settings string `json:"settings,omitempty"`
	// Setting sources to load: user, project, local
	SettingSources []string `json:"setting_sources,omitempty" example:"user,project"`

	// Beta headers for API requests
	Betas []string `json:"betas,omitempty"`

	// Enable verbose mode
	Verbose bool `json:"verbose,omitempty" example:"false"`
	// Debug mode with optional category filter
	Debug string `json:"debug,omitempty" example:"api,hooks"`
	// Disable all slash commands/skills
	DisableSlashCommands bool `json:"disable_slash_commands,omitempty" example:"false"`
}
    ClaudeOptions contains all Claude CLI headless mode options @Description All
    available Claude CLI options for headless execution

type ClaudeStatusResponse struct {
	Configured bool   `json:"configured" example:"true"`
	Message    string `json:"message" example:"Claude is configured"`
}
    ClaudeStatusResponse represents the Claude status response @Description
    Claude configuration status

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
	Name   string `json:"name" example:"stromboli"`
}
    HealthResponse represents the health check response @Description Health
    check response

type PodmanOptions struct {
	// Volume mounts (host:container or host:container:options format)
	Volumes []string `json:"volumes,omitempty" example:"/data:/data:ro"`
}
    PodmanOptions contains Podman container configuration @Description Podman
    container mount configuration

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
    RunRequest represents a request to run Claude in a container @Description
    Request to execute Claude Code in an isolated container

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
	SessionID string `json:"session_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}
    RunResponse represents the response from a Claude run @Description Response
    from Claude execution

type Server struct {
	// Has unexported fields.
}
    Server represents the HTTP API server

func NewServer(r runner.Runner, claudeClient *claude.Client) *Server
    NewServer creates a new API server

func (s *Server) Run(addr string) error
    Run starts the server on the given address

```

## Package claude

```
package claude // import "github.com/tomblanc/stromboli/internal/claude"


CONSTANTS

const DefaultSecretsFile = ".claude-secrets"

VARIABLES

var ErrTokenNotFound = errors.New("token not found")

TYPES

type Client struct {
	// Has unexported fields.
}
    Client wraps Claude Code token operations

func NewClient(secretsFile string) *Client
    NewClient creates a new Claude client

func (c *Client) GetToken() (string, error)
    GetToken reads the token from secrets file

func (c *Client) IsConfigured() bool
    IsConfigured checks if token file exists

type CommandBuilder struct {
	// Has unexported fields.
}
    CommandBuilder builds claude CLI commands with a fluent API Supports ALL
    Claude CLI options for headless mode

func NewCommandBuilder() *CommandBuilder
    NewCommandBuilder creates a new claude command builder

func (b *CommandBuilder) Build() []string
    Build generates the final claude command

func (b *CommandBuilder) WithAddDir(dirs ...string) *CommandBuilder
    WithAddDir adds directories for tool access

func (b *CommandBuilder) WithAgent(agent string) *CommandBuilder
    WithAgent sets the agent for current session

func (b *CommandBuilder) WithAgents(agents map[string]any) *CommandBuilder
    WithAgents sets custom agents as JSON object

func (b *CommandBuilder) WithAllowDangerouslySkipPermissions() *CommandBuilder
    WithAllowDangerouslySkipPermissions enables bypass as an option

func (b *CommandBuilder) WithAllowedTools(tools ...string) *CommandBuilder
    WithAllowedTools sets allowed tools (e.g., "Bash(git:*) Edit")

func (b *CommandBuilder) WithAppendSystemPrompt(prompt string) *CommandBuilder
    WithAppendSystemPrompt appends to the default system prompt

func (b *CommandBuilder) WithBetas(betas ...string) *CommandBuilder
    WithBetas adds beta headers for API requests

func (b *CommandBuilder) WithContinue() *CommandBuilder
    WithContinue continues the most recent conversation in current directory

func (b *CommandBuilder) WithDangerouslySkipPermissions() *CommandBuilder
    WithDangerouslySkipPermissions bypasses all permission checks

func (b *CommandBuilder) WithDebug(filter string) *CommandBuilder
    WithDebug enables debug mode with optional category filter

func (b *CommandBuilder) WithDisableSlashCommands() *CommandBuilder
    WithDisableSlashCommands disables all skills

func (b *CommandBuilder) WithDisallowedTools(tools ...string) *CommandBuilder
    WithDisallowedTools sets denied tools

func (b *CommandBuilder) WithFallbackModel(model string) *CommandBuilder
    WithFallbackModel sets fallback model when default is overloaded

func (b *CommandBuilder) WithFiles(files ...string) *CommandBuilder
    WithFiles adds file resources (format: file_id:path)

func (b *CommandBuilder) WithForkSession() *CommandBuilder
    WithForkSession creates a new session ID when resuming

func (b *CommandBuilder) WithIncludePartialMessages() *CommandBuilder
    WithIncludePartialMessages includes partial message chunks

func (b *CommandBuilder) WithInputFormat(format string) *CommandBuilder
    WithInputFormat sets input format ("text" or "stream-json")

func (b *CommandBuilder) WithJSONSchema(schema string) *CommandBuilder
    WithJSONSchema sets JSON Schema for structured output validation

func (b *CommandBuilder) WithMCPConfig(configs ...string) *CommandBuilder
    WithMCPConfig adds MCP server configs (file paths or JSON strings)

func (b *CommandBuilder) WithMaxBudgetUSD(amount float64) *CommandBuilder
    WithMaxBudgetUSD sets maximum dollar amount for API calls

func (b *CommandBuilder) WithModel(model string) *CommandBuilder
    WithModel sets the model (e.g., "opus", "sonnet", or full name)

func (b *CommandBuilder) WithNoSessionPersistence() *CommandBuilder
    WithNoSessionPersistence disables session saving to disk

func (b *CommandBuilder) WithOutputFormat(format string) *CommandBuilder
    WithOutputFormat sets output format ("text", "json", "stream-json")

func (b *CommandBuilder) WithPermissionMode(mode string) *CommandBuilder
    WithPermissionMode sets permission mode Options: "acceptEdits",
    "bypassPermissions", "default", "delegate", "dontAsk", "plan"

func (b *CommandBuilder) WithPluginDir(dirs ...string) *CommandBuilder
    WithPluginDir loads plugins from directories

func (b *CommandBuilder) WithPrompt(prompt string) *CommandBuilder
    WithPrompt sets the prompt to send to Claude

func (b *CommandBuilder) WithReplayUserMessages() *CommandBuilder
    WithReplayUserMessages re-emits user messages on stdout

func (b *CommandBuilder) WithResume(sessionID string) *CommandBuilder
    WithResume resumes a conversation by session ID

func (b *CommandBuilder) WithSessionID(id string) *CommandBuilder
    WithSessionID sets a specific session ID (must be valid UUID)

func (b *CommandBuilder) WithSettingSources(sources ...string) *CommandBuilder
    WithSettingSources sets which setting sources to load

func (b *CommandBuilder) WithSettings(settings string) *CommandBuilder
    WithSettings sets path to settings JSON file or JSON string

func (b *CommandBuilder) WithStrictMCPConfig() *CommandBuilder
    WithStrictMCPConfig only uses MCP servers from --mcp-config

func (b *CommandBuilder) WithSystemPrompt(prompt string) *CommandBuilder
    WithSystemPrompt sets a custom system prompt (replaces default)

func (b *CommandBuilder) WithTools(tools ...string) *CommandBuilder
    WithTools sets available tools from built-in set Use "" to disable all,
    "default" for all, or specific names

func (b *CommandBuilder) WithVerbose() *CommandBuilder
    WithVerbose enables verbose mode

```

## Package podman

```
package podman // import "github.com/tomblanc/stromboli/internal/podman"


TYPES

type CommandBuilder struct {
	// Has unexported fields.
}
    CommandBuilder builds podman run commands with a fluent API

func NewCommand() *CommandBuilder
    NewCommand creates a new podman command builder

func (b *CommandBuilder) Build() []string
    Build generates the final podman command

func (b *CommandBuilder) WithCommand(cmd []string) *CommandBuilder
    WithCommand sets the command to run in the container

func (b *CommandBuilder) WithEnv(key, value string) *CommandBuilder
    WithEnv adds an environment variable

func (b *CommandBuilder) WithImage(image string) *CommandBuilder
    WithImage sets the container image

func (b *CommandBuilder) WithInteractive() *CommandBuilder
    WithInteractive enables interactive mode (-it)

func (b *CommandBuilder) WithName(name string) *CommandBuilder
    WithName sets the container name

func (b *CommandBuilder) WithTmpfs(containerPath string) *CommandBuilder
    WithTmpfs adds a tmpfs mount (ephemeral in-memory storage) Useful for
    session isolation - data is lost when container stops

func (b *CommandBuilder) WithTmpfsSized(containerPath, size string) *CommandBuilder
    WithTmpfsSized adds a tmpfs mount with size limit

func (b *CommandBuilder) WithVolume(hostPath, containerPath string) *CommandBuilder
    WithVolume adds a volume mount

func (b *CommandBuilder) WithVolumeRaw(volume string) *CommandBuilder
    WithVolumeRaw adds a raw volume string (for complex mounts like :ro, :Z,
    etc.)

func (b *CommandBuilder) WithVolumeReadOnly(hostPath, containerPath string) *CommandBuilder
    WithVolumeReadOnly adds a read-only volume mount

func (b *CommandBuilder) WithWorkdir(dir string) *CommandBuilder
    WithWorkdir sets the working directory

```

## Package runner

```
package runner // import "github.com/tomblanc/stromboli/internal/runner"


TYPES

type ClaudeOptions struct {
	// Session management
	SessionID     string
	Continue      bool
	ForkSession   bool
	NoPersistence bool

	// Model
	Model         string
	FallbackModel string

	// System prompt
	SystemPrompt       string
	AppendSystemPrompt string

	// Tools
	Tools           []string
	AllowedTools    []string
	DisallowedTools []string

	// Permissions
	PermissionMode                  string
	DangerouslySkipPermissions      bool
	AllowDangerouslySkipPermissions bool

	// I/O format
	InputFormat            string
	OutputFormat           string
	IncludePartialMessages bool
	ReplayUserMessages     bool

	// Structured output
	JSONSchema string

	// Budget
	MaxBudgetUSD float64

	// MCP
	MCPConfigs      []string
	StrictMCPConfig bool

	// Agents
	Agent  string
	Agents map[string]any

	// Resources
	AddDirs    []string
	PluginDirs []string
	Files      []string

	// Settings
	Settings       string
	SettingSources []string

	// Beta
	Betas []string

	// Misc
	Verbose              bool
	Debug                string
	DisableSlashCommands bool
}
    ClaudeOptions mirrors api.ClaudeOptions for runner layer

type PodmanOptions struct {
	Volumes []string
}
    PodmanOptions mirrors api.PodmanOptions for runner layer

type PodmanRunner struct {
	// Has unexported fields.
}
    PodmanRunner runs Claude using Podman containers

func NewPodmanRunner(image, secretsFile string) *PodmanRunner
    NewPodmanRunner creates a new Podman-based runner

func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error)
    Run executes Claude in a Podman container

type Request struct {
	Prompt    string
	Workspace string
	Claude    ClaudeOptions
	Podman    PodmanOptions
}
    Request contains the parameters for running Claude

type Result struct {
	ID        string
	Output    string
	SessionID string
}
    Result contains the output from running Claude

type Runner interface {
	Run(ctx context.Context, req Request) (*Result, error)
}
    Runner executes Claude in a container

```

## Package container

```
package container // import "github.com/tomblanc/stromboli/internal/container"


VARIABLES

var ErrCommandFailed = errors.New("podman command failed")
var ErrContainerNotFound = errors.New("container not found")

TYPES

type Manager struct{}
    Manager handles Podman container operations

func NewManager() *Manager
    NewManager creates a new container manager

func (m *Manager) Create(spec Spec) (string, error)
    Create creates a new container (does not start it)

func (m *Manager) Remove(nameOrID string) error
    Remove removes a container

func (m *Manager) Run(spec Spec) (string, error)
    Run creates and starts a container (shorthand)

func (m *Manager) Start(nameOrID string) error
    Start starts an existing container

func (m *Manager) Stop(nameOrID string) error
    Stop stops a running container

func (m *Manager) VolumeCreate(name string) error
    VolumeCreate creates a new volume

func (m *Manager) VolumeExists(name string) (bool, error)
    VolumeExists checks if a volume exists

func (m *Manager) VolumeRemove(name string) error
    VolumeRemove removes a volume

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}
    Mount represents a volume mount

type Spec struct {
	Name   string
	Image  string
	Mounts []Mount
	Env    map[string]string
	Cmd    []string
}
    Spec defines container configuration

```

