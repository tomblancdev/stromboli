package api

import (
	"encoding/json"

	"stromboli/internal/job"
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
	// Execution status: completed, crashed, error
	Status string `json:"status" example:"completed"`
	// Claude's raw output (full CLI output, including JSON envelope when output_format=json)
	Output string `json:"output,omitempty" example:"Here is my analysis..."`
	// Extracted structured_output from Claude's JSON envelope (when json_schema is used).
	// Convenience field so callers don't need to parse the envelope themselves.
	StructuredOutput json.RawMessage `json:"structured_output,omitempty" swaggertype:"object"`
	// Error message (when failed)
	Error string `json:"error,omitempty" example:""`
	// Session ID for conversation continuation
	SessionID string `json:"session_id,omitempty" example:"sess-abc123def456"`
	// Token usage totals + estimated USD cost. Populated best-effort from the
	// session's JSONL; nil when the session file isn't readable yet or no
	// assistant message reported usage.
	Usage *job.Usage `json:"usage,omitempty"`
	// Crash details (when status is "crashed")
	CrashInfo *job.CrashInfo `json:"crash_info,omitempty"`
}

// extractStructuredOutput parses Claude's JSON envelope and returns the
// structured_output field if present. Returns nil if the output is not JSON
// or doesn't contain structured_output.
func extractStructuredOutput(output string) json.RawMessage {
	var envelope struct {
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return nil
	}
	if len(envelope.StructuredOutput) == 0 || string(envelope.StructuredOutput) == "null" {
		return nil
	}
	return envelope.StructuredOutput
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

// ============================================================================
// Image Discovery API Types
// ============================================================================

// ImagesListResponse represents the list of local container images
// @Description List of local container images sorted by compatibility
type ImagesListResponse struct {
	Images []ImageInfoResponse `json:"images"`
}

// ImageInfoResponse represents metadata about a container image
// @Description Container image metadata with compatibility information
type ImageInfoResponse struct {
	ID                string   `json:"id" example:"sha256:abc123def456"`
	Repository        string   `json:"repository" example:"python"`
	Tag               string   `json:"tag" example:"3.12-slim"`
	Size              int64    `json:"size" example:"125000000"`
	Created           string   `json:"created" example:"2024-01-15T10:30:00Z"`
	CompatibilityRank int      `json:"compatibility_rank" example:"3"`
	Compatible        bool     `json:"compatible" example:"true"`
	Tools             []string `json:"tools,omitempty" example:"python,pip"`
	HasClaudeCLI      bool     `json:"has_claude_cli" example:"false"`
	Description       string   `json:"description,omitempty" example:"Python development image"`
}

// ImageDetailResponse represents detailed information about a specific image
// @Description Detailed container image information including labels
type ImageDetailResponse struct {
	ID                string            `json:"id" example:"sha256:abc123def456"`
	Repository        string            `json:"repository" example:"python"`
	Tag               string            `json:"tag" example:"3.12-slim"`
	Size              int64             `json:"size" example:"125000000"`
	Created           string            `json:"created" example:"2024-01-15T10:30:00Z"`
	Labels            map[string]string `json:"labels,omitempty"`
	CompatibilityRank int               `json:"compatibility_rank" example:"3"`
	RankDescription   string            `json:"rank_description" example:"Standard glibc-based (compatible)"`
	Compatible        bool              `json:"compatible" example:"true"`
	Tools             []string          `json:"tools,omitempty" example:"python,pip"`
	HasClaudeCLI      bool              `json:"has_claude_cli" example:"false"`
	Description       string            `json:"description,omitempty" example:"Python development image"`
}

// ImageSearchResponse represents registry search results
// @Description Search results from container registries
type ImageSearchResponse struct {
	Results []SearchResultResponse `json:"results"`
}

// SearchResultResponse represents a single search result
// @Description Search result from a container registry
type SearchResultResponse struct {
	Index       string `json:"index" example:"docker.io"`
	Name        string `json:"name" example:"python"`
	Description string `json:"description" example:"Python is an interpreted programming language"`
	Stars       int    `json:"stars" example:"8500"`
	Official    bool   `json:"official" example:"true"`
	Automated   bool   `json:"automated" example:"false"`
}

// ImagePullRequest represents a request to pull an image
// @Description Request to pull a container image from a registry
type ImagePullRequest struct {
	Image    string `json:"image" binding:"required" example:"python:3.12-slim"`
	Platform string `json:"platform,omitempty" example:"linux/amd64"`
	Quiet    bool   `json:"quiet,omitempty" example:"true"`
}

// ImagePullResponse represents the response from pulling an image
// @Description Result of image pull operation
type ImagePullResponse struct {
	Success bool   `json:"success" example:"true"`
	ImageID string `json:"image_id,omitempty" example:"sha256:abc123def456"`
	Image   string `json:"image" example:"python:3.12-slim"`
}
