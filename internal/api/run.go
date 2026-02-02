package api

import (
	"stromboli/internal/types"
)

// HealthResponse represents the health check response
// @Description Health check response
type HealthResponse struct {
	Status     string            `json:"status" example:"ok"`
	Name       string            `json:"name" example:"stromboli"`
	Version    string            `json:"version" example:"0.1.4"`
	Components []ComponentHealth `json:"components,omitempty"`
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

	// Working directory inside the container where Claude will spawn
	// Use podman.volumes to mount host paths into the container
	Workdir string `json:"workdir,omitempty" example:"/workspace"`

	// Webhook URL to notify when job completes (async only)
	WebhookURL string `json:"webhook_url,omitempty" example:"https://example.com/webhook"`

	// Claude configuration - all CLI options exposed
	Claude types.ClaudeOptions `json:"claude,omitempty"`

	// Podman configuration
	Podman types.PodmanOptions `json:"podman,omitempty"`
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

// SecretsListResponse represents the list of available Podman secrets
// @Description List of available secrets that can be injected into agents
type SecretsListResponse struct {
	Secrets []SecretInfoResponse `json:"secrets"`
	Error   string               `json:"error,omitempty"`
}

// SecretInfoResponse represents metadata about a secret
// @Description Secret metadata (never contains the actual secret value)
type SecretInfoResponse struct {
	ID        string `json:"id" example:"abc123def456"`
	Name      string `json:"name" example:"github-token"`
	CreatedAt string `json:"created_at" example:"2024-01-15T10:30:00Z"`
}

// CreateSecretRequest represents a request to create a new secret
// @Description Request to create a new Podman secret
type CreateSecretRequest struct {
	Name  string `json:"name" binding:"required" example:"github-token"`
	Value string `json:"value" binding:"required" example:"ghp_xxxxxxxxxxxx"`
}

// CreateSecretResponse represents the response from creating a secret
// @Description Result of secret creation
type CreateSecretResponse struct {
	Success bool   `json:"success" example:"true"`
	Name    string `json:"name,omitempty" example:"github-token"`
	Error   string `json:"error,omitempty"`
}

// DeleteSecretResponse represents the response from deleting a secret
// @Description Result of secret deletion
type DeleteSecretResponse struct {
	Success bool   `json:"success" example:"true"`
	Name    string `json:"name,omitempty" example:"github-token"`
	Error   string `json:"error,omitempty"`
}
