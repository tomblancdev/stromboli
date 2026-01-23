package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/auth"
	"stromboli/internal/claude"
	strerrors "stromboli/internal/errors"
	"stromboli/internal/job"
	"stromboli/internal/runner"
)

func newTestServer(t *testing.T, mockRunner runner.Runner, configured bool) *Server {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	if configured {
		err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
		require.NoError(t, err)
	}
	claudeClient := claude.NewClient(secretsFile)
	// Auth disabled for tests (backward compatibility)
	authConfig := auth.Config{Enabled: false}
	// Rate limiting disabled for tests (backward compatibility)
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()
	// Health checker, blacklist nil and tracing disabled for basic tests
	return NewServer(mockRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, nil, nil, false)
}

func TestHealthCheck(t *testing.T) {
	server := newTestServer(t, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "stromboli", response["name"])
}

func TestHealthCheck_WithHealthChecker(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	claudeClient := claude.NewClient(secretsFile)
	authConfig := auth.Config{Enabled: false}
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()

	// Create mock executor that returns success for all checks
	mockExecutor := runner.NewMockExecutor()
	mockExecutor.DefaultOutput = []byte("ok")
	mockExecutor.DefaultError = nil

	healthConfig := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	healthChecker := NewHealthChecker(mockExecutor, healthConfig)

	server := NewServer(nil, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response HealthResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "stromboli", response.Name)
	require.Len(t, response.Components, 2)
	assert.Equal(t, "podman", response.Components[0].Name)
	assert.Equal(t, "ok", response.Components[0].Status)
	assert.Equal(t, "claude-secret", response.Components[1].Name)
	assert.Equal(t, "ok", response.Components[1].Status)
}

func TestHealthCheck_Degraded(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	claudeClient := claude.NewClient(secretsFile)
	authConfig := auth.Config{Enabled: false}
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()

	// Create mock executor that returns error for podman check
	mockExecutor := runner.NewMockExecutor()
	mockExecutor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "podman" && len(args) > 1 && args[1] == "version" {
			return nil, context.DeadlineExceeded
		}
		return []byte("ok"), nil
	}

	healthConfig := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	healthChecker := NewHealthChecker(mockExecutor, healthConfig)

	server := NewServer(nil, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response HealthResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "degraded", response.Status)
	assert.Equal(t, "stromboli", response.Name)
	require.Len(t, response.Components, 2)
	assert.Equal(t, "error", response.Components[0].Status)
	assert.Equal(t, "ok", response.Components[1].Status)
}

func TestClaudeStatus_NotConfigured(t *testing.T) {
	server := newTestServer(t, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/claude/status", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, false, response["configured"])
	assert.Contains(t, response["message"], "make claude-setup")
}

func TestClaudeStatus_Configured(t *testing.T) {
	server := newTestServer(t, nil, true)

	req, err := http.NewRequest(http.MethodGet, "/claude/status", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, true, response["configured"])
}

func TestRun_MissingPrompt_ReturnsBadRequest(t *testing.T) {
	server := newTestServer(t, nil, true)

	body := bytes.NewBufferString(`{}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response RunResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Error, "Invalid request")
}

func TestRun_NotConfigured_ReturnsServiceUnavailable(t *testing.T) {
	server := newTestServer(t, nil, false)

	body := bytes.NewBufferString(`{"prompt": "hello"}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var response RunResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Error, "not configured")
}

func TestRun_Success(t *testing.T) {
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			return &runner.Result{
				ID:        "run-123",
				Output:    "Hello! I'm Claude.",
				SessionID: "session-456",
			}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "say hello", "claude": {"session_id": "session-456"}}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response RunResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, "run-123", response.ID)
	assert.Equal(t, "Hello! I'm Claude.", response.Output)
	assert.Equal(t, "session-456", response.SessionID)
}

func TestRun_WithWorkspace(t *testing.T) {
	var capturedReq runner.Request
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			capturedReq = req
			return &runner.Result{ID: "run-1", Output: "done"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "analyze", "workspace": "/home/user/project"}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/home/user/project", capturedReq.Workspace)
}

func TestRun_WithModel(t *testing.T) {
	var capturedReq runner.Request
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			capturedReq = req
			return &runner.Result{ID: "run-1", Output: "done"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "hello", "claude": {"model": "opus"}}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "opus", capturedReq.Claude.Model)
}

func TestRun_WithAllClaudeOptions(t *testing.T) {
	var capturedReq runner.Request
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			capturedReq = req
			return &runner.Result{ID: "run-1", Output: "done"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	requestBody := `{
		"prompt": "test",
		"workspace": "/project",
		"claude": {
			"session_id": "sess-123",
			"model": "opus",
			"system_prompt": "You are a tester",
			"allowed_tools": ["Bash", "Read"],
			"disallowed_tools": ["Write"],
			"permission_mode": "bypassPermissions",
			"output_format": "json",
			"max_budget_usd": 5.00,
			"verbose": true
		},
		"podman": {
			"volumes": ["/data:/data:ro"]
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "/run", bytes.NewBufferString(requestBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify all options were captured
	assert.Equal(t, "sess-123", capturedReq.Claude.SessionID)
	assert.Equal(t, "opus", capturedReq.Claude.Model)
	assert.Equal(t, "You are a tester", capturedReq.Claude.SystemPrompt)
	assert.Equal(t, []string{"Bash", "Read"}, capturedReq.Claude.AllowedTools)
	assert.Equal(t, []string{"Write"}, capturedReq.Claude.DisallowedTools)
	assert.Equal(t, "bypassPermissions", capturedReq.Claude.PermissionMode)
	assert.Equal(t, "json", capturedReq.Claude.OutputFormat)
	assert.Equal(t, 5.00, capturedReq.Claude.MaxBudgetUSD)
	assert.True(t, capturedReq.Claude.Verbose)
	assert.Equal(t, []string{"/data:/data:ro"}, capturedReq.Podman.Volumes)
}

func TestListSessions_Success(t *testing.T) {
	mockRunner := &runner.MockRunner{
		ListSessionsFunc: func() ([]string, error) {
			return []string{"sess-1", "sess-2", "sess-3"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/sessions", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response SessionListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response.Sessions, 3)
	assert.Contains(t, response.Sessions, "sess-1")
	assert.Contains(t, response.Sessions, "sess-2")
	assert.Contains(t, response.Sessions, "sess-3")
}

func TestListSessions_Empty(t *testing.T) {
	mockRunner := &runner.MockRunner{
		ListSessionsFunc: func() ([]string, error) {
			return []string{}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/sessions", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response SessionListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Empty(t, response.Sessions)
}

func TestDestroySession_Success(t *testing.T) {
	var destroyedID string
	mockRunner := &runner.MockRunner{
		DestroySessionFunc: func(sessionID string) error {
			destroyedID = sessionID
			return nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodDelete, "/sessions/sess-123", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "sess-123", destroyedID)

	var response SessionDestroyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.Equal(t, "sess-123", response.SessionID)
}

func TestDestroySession_NotFound(t *testing.T) {
	mockRunner := &runner.MockRunner{
		DestroySessionFunc: func(sessionID string) error {
			return strerrors.SessionNotFound(sessionID)
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodDelete, "/sessions/nonexistent", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var response SessionDestroyResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "session not found")
}
