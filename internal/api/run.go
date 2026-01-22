package api

import (
	"github.com/tomblanc/stromboli/internal/types"
)

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
