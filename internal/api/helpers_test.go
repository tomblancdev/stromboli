package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"stromboli/internal/types"
)

func TestValidateClaudeConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns true when Claude is configured", func(t *testing.T) {
		server := newTestServer(t, nil, true)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())

		result := validateClaudeConfigured(c, server.claudeClient)

		assert.True(t, result)
	})

	t.Run("returns false and sends error response when not configured", func(t *testing.T) {
		server := newTestServer(t, nil, false)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)

		result := validateClaudeConfigured(c, server.claudeClient)

		assert.False(t, result)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Contains(t, rec.Body.String(), "Claude not configured")
		assert.Contains(t, rec.Body.String(), "make claude-setup")
	})
}

func TestBuildRunnerRequest(t *testing.T) {
	t.Run("builds runner request with all fields", func(t *testing.T) {
		apiReq := RunRequest{
			Prompt:    "test prompt",
			Workdir: "/test/workspace",
			Claude: types.ClaudeOptions{
				SessionID: "sess-123",
				Model:     "claude-opus-4",
			},
			Podman: types.PodmanOptions{
				CPUs:   "2",
				Memory: "4g",
			},
		}

		runnerReq := buildRunnerRequest(apiReq)

		assert.Equal(t, "test prompt", runnerReq.Prompt)
		assert.Equal(t, "/test/workspace", runnerReq.Workdir)
		assert.Equal(t, "sess-123", runnerReq.Claude.SessionID)
		assert.Equal(t, "claude-opus-4", runnerReq.Claude.Model)
		assert.Equal(t, "2", runnerReq.Podman.CPUs)
		assert.Equal(t, "4g", runnerReq.Podman.Memory)
	})

	t.Run("builds runner request with minimal fields", func(t *testing.T) {
		apiReq := RunRequest{
			Prompt: "minimal test",
		}

		runnerReq := buildRunnerRequest(apiReq)

		assert.Equal(t, "minimal test", runnerReq.Prompt)
		assert.Empty(t, runnerReq.Workdir)
		assert.Empty(t, runnerReq.Claude.SessionID)
	})
}

func TestHandleRunError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sends internal server error with error message", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)

		testErr := errors.New("execution failed")
		handleRunError(c, testErr)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "execution failed")
		assert.Contains(t, rec.Body.String(), `"status":"error"`)
	})
}

func TestHandleBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sends bad request with error message", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)

		testErr := errors.New("missing field")
		handleBadRequest(c, testErr)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Invalid request")
		assert.Contains(t, rec.Body.String(), "missing field")
		assert.Contains(t, rec.Body.String(), `"status":"error"`)
	})
}
