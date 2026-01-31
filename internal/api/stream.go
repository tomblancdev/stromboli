package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"stromboli/internal/runner"
	"stromboli/internal/types"
)

// StreamResponse represents the final SSE event with result metadata
type StreamResponse struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

// runStream handles streaming Claude output via Server-Sent Events
// @Summary Stream Claude output
// @Description Executes Claude and streams output in real-time using SSE
// @Tags execution
// @Accept json
// @Produce text/event-stream
// @Param prompt query string true "The prompt to send to Claude"
// @Param workdir query string false "Working directory inside container"
// @Param session_id query string false "Session ID for conversation continuation"
// @Success 200 {string} string "Event stream of output lines"
// @Failure 400 {string} string "Invalid request"
// @Failure 503 {string} string "Claude not configured"
// @Router /run/stream [get]
func (s *Server) runStream(c *gin.Context) {
	// Parse query parameters
	prompt := c.Query("prompt")
	if prompt == "" {
		c.String(http.StatusBadRequest, "Missing required parameter: prompt")
		return
	}

	workdir := c.Query("workdir")
	sessionID := c.Query("session_id")

	// Check if configured
	if !s.claudeClient.IsConfigured() {
		c.String(http.StatusServiceUnavailable, "Claude not configured. Run 'make claude-setup' first")
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		slog.Error("Streaming not supported")
		c.String(http.StatusInternalServerError, "event: error\ndata: Streaming not supported\n\n")
		return
	}

	// Build runner request
	runnerReq := buildRunnerRequest(RunRequest{
		Prompt:  prompt,
		Workdir: workdir,
		Claude: types.ClaudeOptions{
			SessionID: sessionID,
		},
	})

	// Create output channel
	output := make(chan string, 100)

	// Run Claude with streaming in a goroutine
	resultChan := make(chan *runner.Result)
	errChan := make(chan error)

	go func() {
		result, err := s.runner.RunStream(c.Request.Context(), runnerReq, output)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Stream output to client
	for {
		select {
		case line, ok := <-output:
			if !ok {
				// Channel closed, wait for result or error
				goto waitForResult
			}

			// Send SSE event
			fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			flusher.Flush()

		case <-c.Request.Context().Done():
			// Client disconnected
			slog.Info("Client disconnected during streaming")
			return
		}
	}

waitForResult:
	// Wait for final result or error
	select {
	case result := <-resultChan:
		// Send final event with metadata
		response := StreamResponse{
			ID:        result.ID,
			SessionID: result.SessionID,
			Status:    "completed",
		}

		data, err := json.Marshal(response)
		if err != nil {
			slog.Error("Failed to marshal final result", "error", err)
			fmt.Fprintf(c.Writer, "event: error\ndata: Failed to marshal result\n\n")
			flusher.Flush()
			return
		}

		fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(data))
		flusher.Flush()

	case err := <-errChan:
		// Send error event
		slog.Error("Execution failed during streaming", "error", err)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()

	case <-c.Request.Context().Done():
		// Client disconnected while waiting for result
		slog.Info("Client disconnected while waiting for result")
		return
	}
}
