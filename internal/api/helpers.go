package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/runner"
)

// validateClaudeConfigured checks if Claude is configured and returns an error response if not
func validateClaudeConfigured(c *gin.Context, client *claude.Client) bool {
	if !client.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, RunResponse{
			Status: "error",
			Error:  "Claude not configured. Run 'make claude-setup' first",
		})
		return false
	}
	return true
}

// buildRunnerRequest converts an API RunRequest to a runner.Request
func buildRunnerRequest(req RunRequest) runner.Request {
	return runner.Request{
		Prompt:    req.Prompt,
		Workspace: req.Workspace,
		Claude:    req.Claude,
		Podman:    req.Podman,
	}
}

// handleRunError sends a standardized error response for execution failures
func handleRunError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, RunResponse{
		Status: "error",
		Error:  err.Error(),
	})
}

// handleBadRequest sends a standardized error response for invalid requests
func handleBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, RunResponse{
		Status: "error",
		Error:  "Invalid request: " + err.Error(),
	})
}
