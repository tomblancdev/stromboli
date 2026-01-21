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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/runner"
)

// MockRunner implements runner.Runner for testing
type MockRunner struct {
	RunFunc func(ctx context.Context, req runner.Request) (*runner.Result, error)
}

func (m *MockRunner) Run(ctx context.Context, req runner.Request) (*runner.Result, error) {
	return m.RunFunc(ctx, req)
}

func newTestServer(t *testing.T, mockRunner runner.Runner, configured bool) *Server {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	if configured {
		err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
		require.NoError(t, err)
	}
	claudeClient := claude.NewClient(secretsFile)
	return NewServer(mockRunner, claudeClient)
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
	mockRunner := &MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			return &runner.Result{
				ID:        "run-123",
				Output:    "Hello! I'm Claude.",
				SessionID: "session-456",
			}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "say hello", "session_id": "session-456"}`)
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
	mockRunner := &MockRunner{
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
	mockRunner := &MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			capturedReq = req
			return &runner.Result{ID: "run-1", Output: "done"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "hello", "model": "opus"}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "opus", capturedReq.Model)
}
