# 📚 Stromboli API Reference

> Auto-generated Go documentation for all packages

**Generated:** 2026-01-23 15:14 UTC

---

## 📋 Table of Contents

### Core
- [api](#package-api) - HTTP handlers and REST API endpoints
- [config](#package-config) - Configuration management with Viper
- [errors](#package-errors) - Custom error types

### Execution
- [runner](#package-runner) - Container execution engine
- [podman](#package-podman) - Podman command builder
- [claude](#package-claude) - Claude CLI command builder
- [job](#package-job) - Async job management

### Security
- [auth](#package-auth) - JWT authentication and middleware
- [secrets](#package-secrets) - Podman secrets management

### Infrastructure
- [session](#package-session) - Session ID generation
- [workspace](#package-workspace) - Workspace validation
- [webhook](#package-webhook) - Webhook notifications
- [metrics](#package-metrics) - Prometheus metrics
- [tracing](#package-tracing) - OpenTelemetry distributed tracing
- [types](#package-types) - Shared data types

---

## Package api

<details>
<summary>📦 Click to expand</summary>

```go
package api // import "stromboli/internal/api"

Package api provides HTTP handlers and REST API endpoints for the Stromboli
service.

FUNCTIONS

func EnhancedLoggingMiddleware() gin.HandlerFunc
    EnhancedLoggingMiddleware logs requests with structured fields

func RateLimitMiddleware(config RateLimitConfig) gin.HandlerFunc
    RateLimitMiddleware creates a rate limiting middleware

func RequestIDMiddleware() gin.HandlerFunc
    RequestIDMiddleware generates or uses a request ID for tracking


TYPES

type AsyncRunResponse struct {
	JobID string `json:"job_id" example:"job-abc123def456"`
}
    AsyncRunResponse represents the response from starting an async run
    @Description Response from starting an async Claude execution

type ClaudeStatusResponse struct {
	Configured bool   `json:"configured" example:"true"`
	Message    string `json:"message" example:"Claude is configured"`
}
    ClaudeStatusResponse represents the Claude status response @Description
    Claude configuration status

type ComponentHealth struct {
	// Name of the component
	Name string `json:"name" example:"podman"`
	// Status is "ok" or "error"
	Status string `json:"status" example:"ok"`
	// Error message if status is "error"
	Error string `json:"error,omitempty" example:""`
}
    ComponentHealth represents the health status of a single component

type DetailedHealth struct {
	// Status is "ok" if all components healthy, "degraded" if any component unhealthy
	Status string `json:"status" example:"ok"`
	// Name of the service
	Name string `json:"name" example:"stromboli"`
	// Components contains individual component health statuses
	Components []ComponentHealth `json:"components"`
}
    DetailedHealth represents the overall health with component breakdown

type ErrorResponse struct {
	Error string `json:"error"`
}
    ErrorResponse represents a generic error response

type HealthChecker struct {
	// Has unexported fields.
}
    HealthChecker performs health checks on system components

func NewHealthChecker(executor runner.Executor, config HealthConfig) *HealthChecker
    NewHealthChecker creates a new HealthChecker with the given executor and
    config

func (h *HealthChecker) Check(ctx context.Context) DetailedHealth
    Check performs all health checks and returns the detailed health status

type HealthConfig struct {
	// Timeout is the maximum time to wait for each health check
	Timeout time.Duration
	// SecretName is the name of the Podman secret to check for
	SecretName string
}
    HealthConfig holds health check configuration

func DefaultHealthConfig() HealthConfig
    DefaultHealthConfig returns the default health check configuration

type HealthResponse struct {
	Status     string            `json:"status" example:"ok"`
	Name       string            `json:"name" example:"stromboli"`
	Components []ComponentHealth `json:"components,omitempty"`
}
    HealthResponse represents the health check response @Description Health
    check response

type JobListResponse struct {
	Jobs []*JobResponse `json:"jobs"`
}
    JobListResponse represents a list of jobs @Description List of async jobs

type JobResponse struct {
	ID        string     `json:"id" example:"job-abc123def456"`
	Status    job.Status `json:"status" example:"running"`
	Output    string     `json:"output,omitempty" example:"Hello!"`
	Error     string     `json:"error,omitempty"`
	SessionID string     `json:"session_id,omitempty" example:"sess-abc123def456"`
	CreatedAt string     `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt string     `json:"updated_at" example:"2024-01-15T10:31:00Z"`
}
    JobResponse represents a job status response @Description Job status and
    result

type LogoutResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
    LogoutResponse represents a logout response

type RateLimitConfig struct {
	Enabled         bool          // Whether rate limiting is enabled
	Rate            int           // Requests per period
	Period          time.Duration // Time period (e.g., time.Second)
	Burst           int           // Maximum burst size
	CleanupInterval time.Duration // How often to clean up stale IP entries (default: 10 minutes)
}
    RateLimitConfig defines rate limiting settings

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
    RefreshRequest represents a token refresh request

type RunRequest struct {
	// Required: the prompt to send to Claude
	Prompt string `json:"prompt" binding:"required" example:"Analyze this code and suggest improvements"`

	// Workspace to mount (host path -> /workspace in container)
	Workspace string `json:"workspace,omitempty" example:"/home/user/project"`

	// Webhook URL to notify when job completes (async only)
	WebhookURL string `json:"webhook_url,omitempty" example:"https://example.com/webhook"`

	// Claude configuration - all CLI options exposed
	Claude types.ClaudeOptions `json:"claude,omitempty"`

	// Podman configuration
	Podman types.PodmanOptions `json:"podman,omitempty"`
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
	SessionID string `json:"session_id,omitempty" example:"sess-abc123def456"`
}
    RunResponse represents the response from a Claude run @Description Response
    from Claude execution

type Server struct {
	// Has unexported fields.
}
    Server represents the HTTP API server

func NewServer(r runner.Runner, claudeClient *claude.Client, authConfig auth.Config, rateLimitConfig RateLimitConfig, jobMgr *job.Manager, healthChecker *HealthChecker, blacklist *auth.TokenBlacklist, tracingEnabled bool) *Server
    NewServer creates a new API server

func (s *Server) Handler() http.Handler
    Handler returns the HTTP handler for use with http.Server

func (s *Server) Run(addr string) error
    Run starts the server on the given address

type SessionDestroyResponse struct {
	Success   bool   `json:"success" example:"true"`
	SessionID string `json:"session_id,omitempty" example:"sess-abc123"`
	Error     string `json:"error,omitempty"`
}
    SessionDestroyResponse represents the response from destroying a session
    @Description Result of session destruction

type SessionListResponse struct {
	Sessions []string `json:"sessions" example:"sess-abc123,sess-def456"`
	Error    string   `json:"error,omitempty"`
}
    SessionListResponse represents the response from listing sessions
    @Description List of existing sessions

type StreamResponse struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}
    StreamResponse represents the final SSE event with result metadata

type TokenRequest struct {
	ClientID string `json:"client_id" binding:"required"`
}
    TokenRequest represents a request to generate JWT tokens

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}
    TokenResponse represents a JWT token response

type ValidateResponse struct {
	Valid     bool   `json:"valid"`
	Subject   string `json:"subject,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}
    ValidateResponse represents a token validation response

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package config

<details>
<summary>📦 Click to expand</summary>

```go
package config // import "stromboli/internal/config"

Package config provides centralized configuration management for Stromboli
using Viper. It supports configuration from multiple sources with the following
priority: 1. Environment variables (highest) 2. Config file 3. Defaults (lowest)

Package config handles configuration loading and validation.

TYPES

type AgentConfig struct {
	Image       string           // Container image name
	ImageTag    string           // Container image tag
	SecretsFile string           // Path to Claude secrets file
	SessionsDir string           // Directory for session data
	TokenCache  TokenCacheConfig // Token cache configuration
}
    AgentConfig holds agent container configuration

type AuthConfig struct {
	Enabled     bool     // Enable authentication
	ValidTokens []string // List of valid API tokens (legacy)
}
    AuthConfig holds authentication configuration

type Config struct {
	Server    ServerConfig
	Agent     AgentConfig
	Resources ResourceConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	JWT       JWTConfig
	Jobs      JobsConfig
	Tracing   TracingConfig
}
    Config holds all application configuration

func Load() (*Config, error)
    Load reads configuration from environment variables, config files,
    and defaults. Priority: Environment variables > Config file > Defaults

func LoadFromFile(path string) (*Config, error)
    LoadFromFile loads configuration from a specific file path

func (c *Config) Validate() error
    Validate checks that the configuration is valid

type JWTConfig struct {
	Secret        string        // Secret key for signing tokens
	AccessExpiry  time.Duration // Access token lifetime
	RefreshExpiry time.Duration // Refresh token lifetime
}
    JWTConfig holds JWT authentication configuration

type JobsConfig struct {
	CleanupTTL      time.Duration // How long to keep completed jobs
	CleanupInterval time.Duration // How often to run cleanup
}
    JobsConfig holds job management configuration

type RateLimitConfig struct {
	Enabled bool          // Enable rate limiting
	Rate    int           // Requests per second
	Burst   int           // Maximum burst size
	Period  time.Duration // Period for rate limit (always 1 second)
}
    RateLimitConfig holds rate limiting configuration

type ResourceConfig struct {
	Memory  string // Memory limit (e.g., "512m", "1g")
	CPUs    string // CPU limit (e.g., "1", "2")
	Timeout string // Execution timeout (e.g., "30m", "1h")
}
    ResourceConfig holds default resource limits for containers

type ServerConfig struct {
	Address string // Server address (default: ":8080")
}
    ServerConfig holds HTTP server configuration

type TokenCacheConfig struct {
	Enabled bool          // Enable token caching
	TTL     time.Duration // Cache time-to-live
}
    TokenCacheConfig holds token caching configuration

type TracingConfig struct {
	Enabled     bool   // Enable tracing
	ServiceName string // Service name in traces
	Endpoint    string // OTLP collector endpoint (e.g., "localhost:4317")
	Insecure    bool   // Use insecure connection (no TLS)
}
    TracingConfig holds OpenTelemetry tracing configuration

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package errors

<details>
<summary>📦 Click to expand</summary>

```go
package errors // import "stromboli/internal/errors"

Package errors defines domain error types for Stromboli.

# Error Types

Session errors:
  - ErrSessionIDRequired: Session ID is required but not provided
  - ErrInvalidSessionID: Session ID format is invalid
  - ErrSessionNotFound: Session does not exist

Workspace errors:
  - ErrWorkspaceNotFound: Workspace directory does not exist
  - ErrWorkspaceNotAllowed: Workspace is not in the allowed list

# Usage

    if errors.Is(err, strerrors.ErrSessionNotFound) {
        // Handle missing session
    }

These errors are designed to be used with errors.Is() for type checking and with
fmt.Errorf("%w") for wrapping with additional context.

VARIABLES

var (
	ErrTokenNotFound       = errors.New("token not found")
	ErrContainerNotFound   = errors.New("container not found")
	ErrCommandFailed       = errors.New("podman command failed")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionIDRequired   = errors.New("session ID is required")
	ErrInvalidSessionID    = errors.New("invalid session ID")
	ErrWorkspaceNotAllowed = errors.New("workspace path not allowed")
)
    Domain errors


FUNCTIONS

func ContainerError(action string, err error) error
    ContainerError wraps a container operation error

func SessionNotFound(id string) error
    SessionNotFound returns a session not found error with the session ID

func TokenError(err error) error
    TokenError wraps a token retrieval error

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package runner

<details>
<summary>📦 Click to expand</summary>

```go
package runner // import "stromboli/internal/runner"

Package runner executes Claude Code agents in isolated containers.

TYPES

type Executor interface {
	// Run executes a command and returns the combined output
	Run(ctx context.Context, args []string) ([]byte, error)

	// RunStream executes a command and returns pipes for streaming output
	// Returns stdout and stderr readers, and a function to start and wait for the command
	RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
}
    Executor executes shell commands This interface abstracts command execution
    to enable testing without Podman

type MockExecutor struct {

	// RunFunc is called when Run is invoked, if set
	RunFunc func(ctx context.Context, args []string) ([]byte, error)

	// RunStreamFunc is called when RunStream is invoked, if set
	RunStreamFunc func(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)

	// Calls tracks all command invocations
	Calls [][]string

	// DefaultOutput is returned if RunFunc is not set
	DefaultOutput []byte

	// DefaultError is returned if RunFunc is not set
	DefaultError error

	// StreamOutput is the content to stream if RunStreamFunc is not set
	StreamOutput string

	// StreamError is the error content to stream if RunStreamFunc is not set
	StreamError string

	// StreamStartError is returned from start() if set
	StreamStartError error

	// StreamWaitError is returned from wait() if set
	StreamWaitError error
	// Has unexported fields.
}
    MockExecutor is a test implementation of Executor

func NewMockExecutor() *MockExecutor
    NewMockExecutor creates a new MockExecutor

func (m *MockExecutor) GetCalls() [][]string
    GetCalls returns all recorded command calls

func (m *MockExecutor) Reset()
    Reset clears all recorded calls

func (m *MockExecutor) Run(ctx context.Context, args []string) ([]byte, error)
    Run executes the mock command

func (m *MockExecutor) RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
    RunStream executes the mock command with streaming

type MockRunner struct {
	RunFunc            func(ctx context.Context, req Request) (*Result, error)
	RunStreamFunc      func(ctx context.Context, req Request, output chan<- string) (*Result, error)
	RunAsyncFunc       func(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
	DestroySessionFunc func(sessionID string) error
	ListSessionsFunc   func() ([]string, error)
}
    MockRunner implements Runner for testing

func (m *MockRunner) DestroySession(sessionID string) error

func (m *MockRunner) ListSessions() ([]string, error)

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error)

func (m *MockRunner) RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))

func (m *MockRunner) RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)

type PodmanRunner struct {
	// Has unexported fields.
}
    PodmanRunner runs Claude using Podman containers

func NewPodmanRunner(image, secretsFile, sessionsDir string, allowedWorkspaces []string) (*PodmanRunner, error)
    NewPodmanRunner creates a new Podman-based runner with no default resource
    limits It ensures the Podman secret exists for secure token handling

func NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir string, allowedWorkspaces []string, defaults ResourceDefaults) (*PodmanRunner, error)
    NewPodmanRunnerWithDefaults creates a new Podman-based runner with default
    resource limits It ensures the Podman secret exists for secure token
    handling

func NewPodmanRunnerWithDefaultsAndExecutor(image, secretsFile, sessionsDir string, allowedWorkspaces []string, defaults ResourceDefaults, executor Executor) (*PodmanRunner, error)
    NewPodmanRunnerWithDefaultsAndExecutor creates a new Podman-based runner
    with default resource limits and custom executor This is the most flexible
    constructor, primarily useful for testing

func NewPodmanRunnerWithExecutor(image, secretsFile, sessionsDir string, allowedWorkspaces []string, executor Executor) (*PodmanRunner, error)
    NewPodmanRunnerWithExecutor creates a new Podman-based runner with a custom
    executor This is primarily useful for testing

func (r *PodmanRunner) DestroySession(sessionID string) error
    DestroySession removes a session and all its data

func (r *PodmanRunner) ListSessions() ([]string, error)
    ListSessions returns all existing session IDs

func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error)
    Run executes Claude in a Podman container

func (r *PodmanRunner) RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
    RunAsync executes Claude in a goroutine and calls onComplete when done

func (r *PodmanRunner) RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)
    RunStream executes Claude in a Podman container and streams output in
    real-time

type Request struct {
	Prompt    string
	Workspace string
	Claude    types.ClaudeOptions
	Podman    types.PodmanOptions
}
    Request contains the parameters for running Claude

type ResourceDefaults struct {
	Memory  string
	CPUs    string
	Timeout string
}
    ResourceDefaults contains default resource limits for containers

type Result struct {
	ID        string
	Output    string
	SessionID string
}
    Result contains the output from running Claude

type Runner interface {
	Run(ctx context.Context, req Request) (*Result, error)
	RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)
	RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
	DestroySession(sessionID string) error
	ListSessions() ([]string, error)
}
    Runner executes Claude in a container

type ShellExecutor struct{}
    ShellExecutor executes commands using os/exec

func NewShellExecutor() *ShellExecutor
    NewShellExecutor creates a new ShellExecutor

func (e *ShellExecutor) Run(ctx context.Context, args []string) ([]byte, error)
    Run executes a command and returns the combined output

func (e *ShellExecutor) RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
    RunStream executes a command and returns pipes for streaming output

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package podman

<details>
<summary>📦 Click to expand</summary>

```go
package podman // import "stromboli/internal/podman"

Package podman provides Podman container runtime integration.

TYPES

type CommandBuilder struct {
	// Has unexported fields.
}
    CommandBuilder builds podman run commands with a fluent API

func NewCommand() *CommandBuilder
    NewCommand creates a new podman command builder

func (b *CommandBuilder) Build() []string
    Build generates the final podman command

func (b *CommandBuilder) WithCPUShares(shares int) *CommandBuilder
    WithCPUShares sets the CPU shares (relative weight, default 1024)

func (b *CommandBuilder) WithCPUs(cpus string) *CommandBuilder
    WithCPUs sets the CPU limit (e.g., "0.5", "2")

func (b *CommandBuilder) WithCommand(cmd []string) *CommandBuilder
    WithCommand sets the command to run in the container

func (b *CommandBuilder) WithEnv(key, value string) *CommandBuilder
    WithEnv adds an environment variable

func (b *CommandBuilder) WithImage(image string) *CommandBuilder
    WithImage sets the container image

func (b *CommandBuilder) WithInteractive() *CommandBuilder
    WithInteractive enables interactive mode (-it)

func (b *CommandBuilder) WithKeepID() *CommandBuilder
    WithKeepID enables --userns=keep-id for host user ID mapping This maps the
    host user to the same UID inside the container

func (b *CommandBuilder) WithMemory(limit string) *CommandBuilder
    WithMemory sets the memory limit (e.g., "512m", "1g")

func (b *CommandBuilder) WithName(name string) *CommandBuilder
    WithName sets the container name

func (b *CommandBuilder) WithSecret(name string) *CommandBuilder
    WithSecret adds a podman secret to the container The secret must be
    created beforehand with `podman secret create` Secret is available at
    /run/secrets/<name> inside the container

func (b *CommandBuilder) WithSecretFile(hostPath, containerPath string) *CommandBuilder
    WithSecretFile is deprecated - use WithSecret instead This mounts a file
    read-only but has permission issues with rootless podman

func (b *CommandBuilder) WithTmpfs(containerPath string) *CommandBuilder
    WithTmpfs adds a tmpfs mount (ephemeral in-memory storage) Useful for
    session isolation - data is lost when container stops

func (b *CommandBuilder) WithTmpfsSized(containerPath, size string) *CommandBuilder
    WithTmpfsSized adds a tmpfs mount with size limit

func (b *CommandBuilder) WithUser(user string) *CommandBuilder
    WithUser sets the user to run the container as (e.g., "1000:1000")

func (b *CommandBuilder) WithVolume(hostPath, containerPath string) *CommandBuilder
    WithVolume adds a volume mount

func (b *CommandBuilder) WithVolumeChown(hostPath, containerPath string) *CommandBuilder
    WithVolumeChown adds a volume mount with :U flag The :U flag tells Podman
    to adjust ownership for the container user This allows the container user to
    write to the mounted directory

func (b *CommandBuilder) WithVolumeRaw(volume string) *CommandBuilder
    WithVolumeRaw adds a raw volume string (for complex mounts like :ro, :Z,
    etc.)

func (b *CommandBuilder) WithVolumeReadOnly(hostPath, containerPath string) *CommandBuilder
    WithVolumeReadOnly adds a read-only volume mount

func (b *CommandBuilder) WithWorkdir(dir string) *CommandBuilder
    WithWorkdir sets the working directory

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package claude

<details>
<summary>📦 Click to expand</summary>

```go
package claude // import "stromboli/internal/claude"

Package claude provides Claude CLI command building and token management.

CONSTANTS

const (
	DefaultSecretsFile = ".claude-secrets"
	DefaultCacheTTL    = 5 * time.Minute
)

TYPES

type Client struct {
	// Has unexported fields.
}
    Client wraps Claude Code token operations with caching support

func NewClient(secretsFile string) *Client
    NewClient creates a new Claude client with default cache settings Cache is
    enabled by default with 5 minute TTL

func NewClientWithCache(secretsFile string, cacheEnabled bool, cacheTTL time.Duration) *Client
    NewClientWithCache creates a new Claude client with custom cache settings

func (c *Client) GetToken() (string, error)
    GetToken reads the token from secrets file with caching support If cache
    is enabled and valid, returns cached token. Otherwise reads from file and
    updates cache.

func (c *Client) InvalidateCache()
    InvalidateCache clears the cached token, forcing next GetToken to read from
    file

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
    Build generates the final claude command arguments Note: Does NOT include
    "claude" itself - the container ENTRYPOINT provides that

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

func (b *CommandBuilder) WithResume() *CommandBuilder
    WithResume enables resuming an existing conversation (requires session ID to
    be set)

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

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package job

<details>
<summary>📦 Click to expand</summary>

```go
package job // import "stromboli/internal/job"

Package job provides async job management for long-running Claude execution
tasks.

# Features

  - Create and track async jobs
  - Query job status (pending, running, completed, error)
  - Automatic cleanup of expired jobs
  - Thread-safe job storage

# Job Lifecycle

    pending -> running -> completed
                       -> error

# Usage

Create a job manager:

    mgr := job.NewManager()
    mgr.StartCleanup(time.Hour, 5*time.Minute)
    defer mgr.StopCleanup()

Create and manage jobs:

    jobID := mgr.Create()
    mgr.SetRunning(jobID)
    mgr.Complete(jobID, "output", "session-id")

Query jobs:

    j, exists := mgr.Get(jobID)
    all := mgr.List()

# Cleanup

Jobs are automatically cleaned up based on:
  - Retention time: How long completed jobs are kept
  - Cleanup interval: How often cleanup runs

TYPES

type Job struct {
	ID          string     `json:"id"`
	Status      Status     `json:"status"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	SessionID   string     `json:"session_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}
    Job represents an async execution job

type Manager struct {
	// Has unexported fields.
}
    Manager manages async jobs

func NewManager() *Manager
    NewManager creates a new job manager

func (m *Manager) Cancel(id string) bool
    Cancel cancels a pending or running job

func (m *Manager) Create(id string) *Job
    Create creates a new pending job

func (m *Manager) Delete(id string) bool
    Delete removes a job

func (m *Manager) Get(id string) (*Job, bool)
    Get retrieves a job by ID

func (m *Manager) List() []*Job
    List returns all jobs

func (m *Manager) StartCleanup(ttl time.Duration, interval time.Duration)
    StartCleanup starts a background goroutine that removes
    completed/failed/cancelled jobs older than TTL

func (m *Manager) StopCleanup()
    StopCleanup stops the cleanup goroutine

func (m *Manager) Update(id string, status Status, output, errMsg, sessionID string)
    Update updates a job's status and output

type Status string
    Status represents the current state of a job

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)
```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package auth

<details>
<summary>📦 Click to expand</summary>

```go
package auth // import "stromboli/internal/auth"

Package auth provides JWT-based authentication and authorization for Stromboli.

# Features

  - JWT token generation and validation
  - Token refresh with refresh tokens
  - Token blacklist for logout support
  - Gin middleware for route protection
  - Configurable via Config struct

# Usage

Create a middleware with authentication config:

    cfg := auth.Config{
        Enabled: true,
        JWTConfig: auth.JWTConfig{
            Secret:              "your-secret",
            AccessTokenExpiry:   15 * time.Minute,
            RefreshTokenExpiry:  24 * time.Hour,
        },
    }
    router.Use(auth.Middleware(cfg))

Generate tokens:

    generator := auth.NewTokenGenerator(cfg.JWTConfig)
    access, refresh, err := generator.GenerateTokenPair("user-id")

Validate tokens:

    claims, err := generator.ValidateToken(tokenString)

# Token Blacklist

For logout support, use the TokenBlacklist:

    blacklist := auth.NewTokenBlacklist()
    blacklist.StartCleanup(time.Hour)
    blacklist.Add(claims.ID, claims.ExpiresAt.Time)

CONSTANTS

const (
	DefaultAccessExpiry  = 24 * time.Hour
	DefaultRefreshExpiry = 7 * 24 * time.Hour
)
    Default expiry times


VARIABLES

var (
	ErrMissingAuthHeader = errors.New("missing authorization header")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
	ErrInvalidToken      = errors.New("invalid or expired token")
)

FUNCTIONS

func GenerateRefreshToken(subject string, cfg JWTConfig) (string, error)
    GenerateRefreshToken creates a new JWT refresh token for the given subject

func GenerateToken(subject string, cfg JWTConfig) (string, error)
    GenerateToken creates a new JWT access token for the given subject

func Middleware(cfg Config) gin.HandlerFunc
    Middleware returns a Gin middleware that validates JWT tokens


TYPES

type Claims struct {
	jwt.RegisteredClaims
	IsRefresh bool `json:"is_refresh,omitempty"`
}
    Claims represents JWT claims

func ValidateRefreshToken(tokenString string, cfg JWTConfig) (*Claims, error)
    ValidateRefreshToken validates a refresh token and returns its claims

func ValidateToken(tokenString string, cfg JWTConfig) (*Claims, error)
    ValidateToken validates an access token and returns its claims

type Config struct {
	// Enabled determines if auth is required
	Enabled bool
	// ValidTokens is a simple list of valid tokens (for backward compatibility)
	// These are checked first before JWT validation
	ValidTokens []string
	// JWTConfig holds JWT-specific configuration
	// If JWTConfig.Secret is set, JWT validation is enabled
	JWTConfig JWTConfig
	// Blacklist holds the token blacklist for logout support
	// If set, tokens will be checked against the blacklist
	Blacklist *TokenBlacklist
}
    Config holds JWT middleware configuration

type JWTConfig struct {
	// Secret is the key used to sign tokens
	Secret string
	// AccessExpiry is how long access tokens are valid (default: 24h)
	AccessExpiry time.Duration
	// RefreshExpiry is how long refresh tokens are valid (default: 7 days)
	RefreshExpiry time.Duration
}
    JWTConfig holds JWT configuration

type TokenBlacklist struct {
	// Has unexported fields.
}
    TokenBlacklist manages blacklisted JWT tokens by their JTI (JWT ID) claim.
    Tokens are stored with their expiration time and automatically cleaned up
    after they naturally expire.

func NewTokenBlacklist() *TokenBlacklist
    NewTokenBlacklist creates a new token blacklist

func (b *TokenBlacklist) Add(jti string, expiresAt time.Time)
    Add adds a token to the blacklist. The token will remain blacklisted until
    its natural expiration time, after which it will be cleaned up.

func (b *TokenBlacklist) IsBlacklisted(jti string) bool
    IsBlacklisted checks if a token with the given JTI is blacklisted

func (b *TokenBlacklist) Size() int
    Size returns the current number of blacklisted tokens

func (b *TokenBlacklist) StartCleanup(interval time.Duration)
    StartCleanup starts a background goroutine that removes expired tokens at
    the specified interval. This is idempotent - calling it multiple times will
    not create multiple goroutines.

func (b *TokenBlacklist) StopCleanup()
    StopCleanup stops the cleanup goroutine. This is idempotent.

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package secrets

<details>
<summary>📦 Click to expand</summary>

```go
package secrets // import "stromboli/internal/secrets"

Package secrets provides Podman secret management for secure token handling.

# Overview

Podman secrets are the recommended way to pass sensitive data to containers.
This package wraps Podman's secret commands to create and manage secrets that
can be mounted into containers at /run/secrets/<name>.

# Usage

Create or update a secret from a file:

    mgr := secrets.NewManager(executor)
    err := mgr.EnsureSecret("claude-token", "/path/to/secrets-file")

The secret is created if it doesn't exist, or updated if the content changed.
Secrets are available to containers via --secret flag.

# Security

Secrets are:
  - Stored encrypted by Podman
  - Only accessible to containers that explicitly request them
  - Mounted read-only at /run/secrets/<name>
  - Not visible in container inspection or logs

CONSTANTS

const (
	// DefaultSecretName is the default name for the Claude token secret
	DefaultSecretName = "claude-token"
)

TYPES

type Manager struct {
	// Has unexported fields.
}
    Manager handles Podman secret operations

func NewManager(secretName string) *Manager
    NewManager creates a new secrets manager

func (m *Manager) Create(ctx context.Context, filePath string) error
    Create creates the secret from a file

func (m *Manager) EnsureExists(ctx context.Context, filePath string) error
    EnsureExists creates the secret if it doesn't exist

func (m *Manager) Exists(ctx context.Context) (bool, error)
    Exists checks if the secret exists in Podman

func (m *Manager) Remove(ctx context.Context) error
    Remove removes the secret

func (m *Manager) SecretName() string
    SecretName returns the name of the managed secret

func (m *Manager) Update(ctx context.Context, filePath string) error
    Update removes and recreates the secret with new content

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package session

<details>
<summary>📦 Click to expand</summary>

```go
package session // import "stromboli/internal/session"

Package session manages session lifecycle and persistent storage.

# Session IDs

Sessions are identified by UUIDs in the format "sess-<uuid>". The prefix makes
session IDs easily identifiable in logs and URLs.

# Usage

Generate a new session ID:

    id := session.NewID()  // Returns "sess-abc123..."

Validate a session ID:

    if err := session.Validate(id); err != nil {
        // Invalid format
    }

# Storage

Sessions are stored as directories under the configured sessions path.
Each session directory contains Claude's conversation state and can be resumed
or forked for continued conversations.

# Persistence Options

Sessions support two modes:
  - Persistent: Session data is saved to disk (default)
  - Ephemeral: Session data is stored in tmpfs and lost on container stop

FUNCTIONS

func GenerateID() string
    GenerateID creates a unique session ID in UUID v4 format Claude CLI expects
    UUIDs for --resume flag


TYPES

type Manager struct {
	// Has unexported fields.
}
    Manager handles session lifecycle and filesystem operations

func NewManager(sessionsDir string) *Manager
    NewManager creates a new session Manager

func (m *Manager) Create(sessionID string) (string, string, error)
    Create creates a session directory and returns the sessionID and absolute
    path. If sessionID is empty, generates a new UUID.

func (m *Manager) Destroy(sessionID string) error
    Destroy removes a session and all its data

func (m *Manager) List() ([]string, error)
    List returns all existing session IDs

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package workspace

<details>
<summary>📦 Click to expand</summary>

```go
package workspace // import "stromboli/internal/workspace"

Package workspace provides workspace path validation and security.

# Overview

Workspaces are host directories mounted into containers for Claude to access.
This package ensures only approved directories can be mounted, preventing
unauthorized access to the host filesystem.

# Usage

Create a validator with allowed paths:

    v := workspace.NewValidator([]string{
        "/home/user/projects",
        "/var/data/repos",
    })

Validate a workspace path:

    if err := v.Validate("/home/user/projects/myapp"); err != nil {
        // Path not allowed
    }

# Security

The validator checks that:
  - The path exists on the host
  - The path is under one of the allowed base directories
  - Symlinks are resolved to prevent traversal attacks
  - Empty allowed list means all paths are allowed (use with caution)

# Path Normalization

Paths are cleaned and resolved before validation:
  - Relative paths are made absolute
  - Symlinks are resolved
  - ".." components are resolved

TYPES

type Validator struct {
	// Has unexported fields.
}
    Validator validates workspace paths against an allowlist

func NewValidator(allowedPaths []string) *Validator
    NewValidator creates a workspace validator with allowed paths If
    allowedPaths is empty, all paths are allowed (backward compatible)

func (v *Validator) IsConfigured() bool
    IsConfigured returns true if allowlist is configured

func (v *Validator) Validate(path string) (string, error)
    Validate checks if the workspace path is allowed Returns the cleaned
    absolute path if valid, or error if not

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package webhook

<details>
<summary>📦 Click to expand</summary>

```go
package webhook // import "stromboli/internal/webhook"

Package webhook provides HTTP webhook notifications for async job completion.

# Features

  - POST notifications with JSON payload
  - Automatic retry on failure (1 retry after 100ms)
  - 5 second timeout per request
  - Standard job result payload format

# Usage

    notifier := webhook.NewNotifier()
    err := notifier.Notify("https://example.com/webhook", webhook.JobResult{
        JobID:     "job-123",
        Status:    "completed",
        Output:    "Task completed successfully",
        SessionID: "sess-456",
    })

# Payload Format

The webhook sends a POST request with JSON body:

    {
        "job_id": "job-123",
        "status": "completed|error",
        "output": "...",
        "error": "...",
        "session_id": "sess-456"
    }

TYPES

type JobResult struct {
	JobID     string `json:"job_id"`
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}
    JobResult represents the result of a job to be sent to webhook

type Notifier struct {
	// Has unexported fields.
}
    Notifier sends webhook notifications

func NewNotifier() *Notifier
    NewNotifier creates a new webhook notifier

func (n *Notifier) Notify(url string, payload JobResult) error
    Notify sends a webhook notification with retry logic

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package metrics

<details>
<summary>📦 Click to expand</summary>

```go
package metrics // import "stromboli/internal/metrics"

Package metrics provides Prometheus metrics for Stromboli monitoring.

# Available Metrics

HTTP Metrics:
  - http_requests_total: Counter of HTTP requests by method, path, and status
  - http_request_duration_seconds: Histogram of request durations

Container Metrics:
  - active_containers: Gauge of currently running containers

# Usage

Record HTTP request:

    metrics.RecordRequest("POST", "/run", 200)
    metrics.RecordDuration("POST", "/run", 1.5)

Track containers:

    metrics.IncActiveContainers()
    defer metrics.DecActiveContainers()

# Prometheus Endpoint

Metrics are exposed at /metrics endpoint automatically via promhttp.Handler().
Use Grafana dashboard in deployments/grafana/ for visualization.

FUNCTIONS

func DecActiveContainers()
    DecActiveContainers decrements the active containers counter

func IncActiveContainers()
    IncActiveContainers increments the active containers counter

func RecordDuration(method, path string, durationSeconds float64)
    RecordDuration records the HTTP request duration

func RecordRequest(method, path string, status int)
    RecordRequest increments the HTTP request counter

func SetActiveContainers(count int)
    SetActiveContainers sets the number of active containers

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package tracing

<details>
<summary>📦 Click to expand</summary>

```go
package tracing // import "stromboli/internal/tracing"

Package tracing provides OpenTelemetry distributed tracing support for
Stromboli.

# Configuration

Tracing is configured via environment variables or config file:

    STROMBOLI_TRACING_ENABLED=true
    STROMBOLI_TRACING_SERVICE_NAME=stromboli
    STROMBOLI_TRACING_ENDPOINT=localhost:4317
    STROMBOLI_TRACING_INSECURE=true

# Usage

Initialize tracing at application startup:

    shutdown, err := tracing.Init(ctx, tracing.Config{
        Enabled:     true,
        ServiceName: "stromboli",
        Endpoint:    "localhost:4317",
        Insecure:    true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

Create spans in your code:

    ctx, span := tracing.StartSpan(ctx, "operation-name")
    defer span.End()

    // Add attributes
    tracing.AddSpanAttributes(ctx, "key", "value", "count", 42)

    // Record errors
    if err != nil {
        tracing.RecordError(ctx, err)
    }

# Gin Middleware

For HTTP tracing, use the otelgin middleware in your routes:

    import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

    router.Use(otelgin.Middleware("stromboli"))

Package tracing provides OpenTelemetry distributed tracing support for
Stromboli. It configures a trace provider with OTLP export and provides helper
functions for creating and managing spans throughout the application.

CONSTANTS

const (
	SpanKindServer   = trace.SpanKindServer
	SpanKindClient   = trace.SpanKindClient
	SpanKindInternal = trace.SpanKindInternal
)
    Span kind constants for convenience

const (
	StatusOK    = codes.Ok
	StatusError = codes.Error
)
    Status code constants for convenience


FUNCTIONS

func AddSpanAttributes(ctx context.Context, kvs ...interface{})
    AddSpanAttributes adds attributes to the current span in the context.
    Attributes should be provided as key-value pairs.

func RecordError(ctx context.Context, err error, opts ...trace.EventOption)
    RecordError records an error on the current span in the context.

func SetSpanStatus(ctx context.Context, code codes.Code, description string)
    SetSpanStatus sets the status of the current span.

func SpanFromContext(ctx context.Context) trace.Span
    SpanFromContext returns the current span from the context. Returns a noop
    span if no span is in the context.

func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span)
    StartSpan starts a new span with the given name. Returns the updated context
    and the span. The span must be ended with span.End().

func StartSpanWithKind(ctx context.Context, name string, kind trace.SpanKind) (context.Context, trace.Span)
    StartSpanWithKind starts a new span with a specific span kind.

func Tracer(name string) trace.Tracer
    Tracer returns a named tracer for creating spans


TYPES

type Config struct {
	// Enabled determines if tracing is active
	Enabled bool
	// ServiceName is the name of the service for traces
	ServiceName string
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317")
	Endpoint string
	// Insecure disables TLS for the gRPC connection
	Insecure bool
}
    Config holds OpenTelemetry tracing configuration

type ShutdownFunc func(context.Context) error
    ShutdownFunc is a function that shuts down the trace provider

func Init(ctx context.Context, cfg Config) (ShutdownFunc, error)
    Init initializes the OpenTelemetry trace provider. If tracing is disabled,
    it sets up a noop provider. Returns a shutdown function that must be called
    on application exit.

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

## Package types

<details>
<summary>📦 Click to expand</summary>

```go
package types // import "stromboli/internal/types"

Package types defines shared data types used across packages.

# Overview

This package contains data structures that are used by multiple packages
to avoid circular dependencies. It includes request/response types and
configuration options.

# Types

ClaudeOptions: Configuration for Claude CLI execution
  - Model selection (opus, sonnet, haiku)
  - Session management (ID, resume, fork)
  - Permission modes and allowed tools
  - System prompt customization
  - Output format (text, json, stream-json)

PodmanOptions: Configuration for container execution
  - Resource limits (memory, CPU, timeout)
  - Network settings
  - Environment variables
  - Volume mounts

# Usage

    req := types.ClaudeOptions{
        Model:          "sonnet",
        SessionID:      "sess-123",
        Resume:         true,
        PermissionMode: "bypassPermissions",
    }

TYPES

type ClaudeOptions struct {

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

type PodmanOptions struct {
	// Volume mounts (host:container or host:container:options format)
	Volumes []string `json:"volumes,omitempty" example:"/data:/data:ro"`

	// Container timeout (e.g., "5m", "1h", "30s")
	Timeout string `json:"timeout,omitempty" example:"5m"`

	// Memory limit (e.g., "512m", "1g")
	Memory string `json:"memory,omitempty" example:"512m"`

	// CPU limit (e.g., "0.5", "2")
	CPUs string `json:"cpus,omitempty" example:"1"`

	// CPU shares (relative weight, default 1024)
	CPUShares int `json:"cpu_shares,omitempty" example:"512"`
}
    PodmanOptions contains Podman container configuration @Description Podman
    container mount configuration

```

</details>

[⬆️ Back to top](#-table-of-contents)

---

