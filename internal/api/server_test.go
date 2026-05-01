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
	"stromboli/internal/images"
	"stromboli/internal/job"
	"stromboli/internal/runner"
	"stromboli/internal/secrets"
)

// mockTestExecutor implements secrets.Executor for testing
type mockTestExecutor struct{}

func (m *mockTestExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	return []byte(""), nil // Return empty output (Podman format for no secrets)
}

func newTestServer(t *testing.T, mockRunner runner.Runner, configured bool) *Server {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
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
	// Create mock registries
	secretsRegistry := secrets.NewRegistry(&mockTestExecutor{})
	imagesRegistry := images.NewRegistry(&mockTestExecutor{})
	// Health checker, blacklist nil and tracing disabled for basic tests
	return NewServer(mockRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, nil, nil, false, sessionsDir, secretsRegistry, imagesRegistry, nil, "")
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
	credentialsFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credentialsFile, []byte("{}"), 0600))
	claudeClient := claude.NewClient(credentialsFile)
	authConfig := auth.Config{Enabled: false}
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()

	// Create mock executor that returns success for all checks
	mockExecutor := runner.NewMockExecutor()
	mockExecutor.DefaultOutput = []byte("ok")
	mockExecutor.DefaultError = nil

	healthConfig := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: credentialsFile,
		SecretName:      "claude-credentials",
	}
	healthChecker := NewHealthChecker(mockExecutor, healthConfig)

	secretsRegistry := secrets.NewRegistry(&mockTestExecutor{})
	imagesRegistry := images.NewRegistry(&mockTestExecutor{})
	server := NewServer(nil, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, nil, false, tmpDir, secretsRegistry, imagesRegistry, nil, "")

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
	require.Len(t, response.Components, 3)
	assert.Equal(t, "podman", response.Components[0].Name)
	assert.Equal(t, "ok", response.Components[0].Status)
	assert.Equal(t, "claude-credentials-file", response.Components[1].Name)
	assert.Equal(t, "ok", response.Components[1].Status)
	assert.Equal(t, "claude-credentials-secret", response.Components[2].Name)
	assert.Equal(t, "ok", response.Components[2].Status)
}

func TestHealthCheck_Degraded(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credentialsFile, []byte("{}"), 0600))
	claudeClient := claude.NewClient(credentialsFile)
	authConfig := auth.Config{Enabled: false}
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()

	// Create mock executor that returns error for podman version check only
	mockExecutor := runner.NewMockExecutor()
	mockExecutor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "podman" && len(args) > 1 && args[1] == "version" {
			return nil, context.DeadlineExceeded
		}
		return []byte("ok"), nil
	}

	healthConfig := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: credentialsFile,
		SecretName:      "claude-credentials",
	}
	healthChecker := NewHealthChecker(mockExecutor, healthConfig)

	secretsRegistry := secrets.NewRegistry(&mockTestExecutor{})
	imagesRegistry := images.NewRegistry(&mockTestExecutor{})
	server := NewServer(nil, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, nil, false, tmpDir, secretsRegistry, imagesRegistry, nil, "")

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
	require.Len(t, response.Components, 3)
	assert.Equal(t, "error", response.Components[0].Status) // podman
	assert.Equal(t, "ok", response.Components[1].Status)    // credentials file
	assert.Equal(t, "ok", response.Components[2].Status)    // secret
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
	assert.Contains(t, response["message"], "claude")
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

func TestRun_WithStructuredOutput(t *testing.T) {
	// Claude CLI envelope with json_schema: result is empty, data in structured_output
	envelopeOutput := `{"type":"result","subtype":"success","result":"","structured_output":{"answer":4}}`

	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			return &runner.Result{ID: "run-so", Output: envelopeOutput, SessionID: "sess-so"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "what is 2+2?", "claude": {"output_format": "json", "json_schema": "{}"}}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Raw output should still be present
	assert.Equal(t, envelopeOutput, response["output"])
	// Structured output should be extracted from the envelope
	so, ok := response["structured_output"].(map[string]interface{})
	require.True(t, ok, "structured_output should be a JSON object")
	assert.Equal(t, float64(4), so["answer"])
}

func TestRun_WithCrashInfo(t *testing.T) {
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			return &runner.Result{
				ID:        "run-crash",
				Output:    "partial output before crash",
				SessionID: "sess-crash",
				CrashInfo: &job.CrashInfo{
					ExitCode:      137,
					Signal:        "killed",
					Reason:        "OOM killed",
					PartialOutput: "partial output before crash",
					TaskCompleted: false,
				},
			}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "heavy task"}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response RunResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "crashed", response.Status)
	assert.Equal(t, "partial output before crash", response.Output)
	assert.NotNil(t, response.CrashInfo)
	assert.Equal(t, 137, response.CrashInfo.ExitCode)
	assert.Equal(t, "killed", response.CrashInfo.Signal)
	assert.False(t, response.CrashInfo.TaskCompleted)
}

func TestExtractStructuredOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantNil  bool
		wantJSON string
	}{
		{"with structured_output object", `{"result":"","structured_output":{"answer":4}}`, false, `{"answer":4}`},
		{"with structured_output array", `{"result":"","structured_output":[1,2,3]}`, false, `[1,2,3]`},
		{"with structured_output string", `{"result":"","structured_output":"hello"}`, false, `"hello"`},
		{"without structured_output", `{"result":"hello"}`, true, ""},
		{"plain text output", "Hello world", true, ""},
		{"null structured_output", `{"result":"ok","structured_output":null}`, true, ""},
		{"empty string", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStructuredOutput(tt.output)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.JSONEq(t, tt.wantJSON, string(result))
			}
		})
	}
}

func TestRun_WithWorkdir(t *testing.T) {
	var capturedReq runner.Request
	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			capturedReq = req
			return &runner.Result{ID: "run-1", Output: "done"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	body := bytes.NewBufferString(`{"prompt": "analyze", "workdir": "/workspace"}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/workspace", capturedReq.Workdir)
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
	require.NotNil(t, capturedReq.Claude.MaxBudgetUSD)
	assert.Equal(t, 5.00, *capturedReq.Claude.MaxBudgetUSD)
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
	ids := make([]string, len(response.Sessions))
	for i, s := range response.Sessions {
		ids[i] = s.ID
	}
	assert.Contains(t, ids, "sess-1")
	assert.Contains(t, ids, "sess-2")
	assert.Contains(t, ids, "sess-3")
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

func TestListSecrets_Success(t *testing.T) {
	// Use the server with the default mock executor (returns empty list)
	server := newTestServer(t, nil, true)

	req, err := http.NewRequest(http.MethodGet, "/secrets", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response SecretsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Empty(t, response.Error)
	// Default mock returns empty list
	assert.Empty(t, response.Secrets)
}

func TestListSecrets_PodmanError(t *testing.T) {
	tmpDir := t.TempDir()
	credentialsFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credentialsFile, []byte("{}"), 0600))
	claudeClient := claude.NewClient(credentialsFile)
	authConfig := auth.Config{Enabled: false}
	rateLimitConfig := RateLimitConfig{Enabled: false}
	jobMgr := job.NewManager()

	// Create mock executor that returns error for secret ls
	mockSecretsExec := &errorMockExecutor{err: context.DeadlineExceeded}
	secretsRegistry := secrets.NewRegistry(mockSecretsExec)
	imagesRegistry := images.NewRegistry(&mockTestExecutor{})

	server := NewServer(nil, claudeClient, authConfig, rateLimitConfig, jobMgr, nil, nil, false, tmpDir, secretsRegistry, imagesRegistry, nil, "")

	req, err := http.NewRequest(http.MethodGet, "/secrets", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var response SecretsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response.Error, "failed to list secrets")
}

func TestListSecrets_EmptyList(t *testing.T) {
	// Use default test server which has mock that returns empty list
	server := newTestServer(t, nil, true)

	req, err := http.NewRequest(http.MethodGet, "/secrets", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response SecretsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Empty(t, response.Error)
	assert.Empty(t, response.Secrets)
}

// errorMockExecutor is a mock executor that always returns an error
type errorMockExecutor struct {
	err error
}

func (m *errorMockExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	return nil, m.err
}

func TestRun_PopulatesUsage(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	require.NoError(t, os.WriteFile(secretsFile, []byte("test-token"), 0600))

	const sessionID = "sess-usage-1"
	// Pre-write the session JSONL so the post-run aggregation has data to
	// consume. The reader looks under <sessionsDir>/<sid>/.claude/projects/*/<sid>.jsonl.
	jsonlDir := filepath.Join(sessionsDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(jsonlDir, 0755))
	jsonl := `{"type":"user","uuid":"u1","sessionId":"` + sessionID + `","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"hi"}}
{"type":"assistant","uuid":"a1","sessionId":"` + sessionID + `","timestamp":"2026-01-24T10:00:01.000Z","message":{"role":"assistant","model":"claude-sonnet-4-6","id":"m1","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1000,"output_tokens":200,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}
`
	require.NoError(t, os.WriteFile(filepath.Join(jsonlDir, sessionID+".jsonl"), []byte(jsonl), 0644))

	mockRunner := &runner.MockRunner{
		RunFunc: func(ctx context.Context, req runner.Request) (*runner.Result, error) {
			return &runner.Result{ID: "run-x", Output: "hello", SessionID: sessionID}, nil
		},
	}
	claudeClient := claude.NewClient(secretsFile)
	jobMgr := job.NewManager()
	server := NewServer(
		mockRunner, claudeClient,
		auth.Config{Enabled: false}, RateLimitConfig{Enabled: false},
		jobMgr, nil, nil, false, sessionsDir,
		secrets.NewRegistry(&mockTestExecutor{}),
		images.NewRegistry(&mockTestExecutor{}),
		nil,
		"",
	)

	body := bytes.NewBufferString(`{"prompt": "hi", "claude": {"session_id": "` + sessionID + `"}}`)
	req, err := http.NewRequest(http.MethodPost, "/run", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp RunResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Usage, "usage should be populated when session JSONL has data")
	assert.Equal(t, "claude-sonnet-4-6", resp.Usage.Model)
	assert.Equal(t, 1000, resp.Usage.InputTokens)
	assert.Equal(t, 200, resp.Usage.OutputTokens)
	assert.Equal(t, 1200, resp.Usage.TotalTokens)
	// 1000 input @ $3/1M + 200 output @ $15/1M = 0.003 + 0.003 = 0.006
	assert.InDelta(t, 0.006, resp.Usage.EstimatedCostUSD, 1e-9)
}

func TestListSessions_SurfacesTitleFromJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	sessionsDir := filepath.Join(tmpDir, "sessions")
	require.NoError(t, os.WriteFile(secretsFile, []byte("test-token"), 0600))

	const sessionID = "sess-titled"
	jsonlDir := filepath.Join(sessionsDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(jsonlDir, 0755))
	// Two custom-title entries — the API should surface the LATER one.
	jsonl := `{"type":"custom-title","customTitle":"first-title","sessionId":"sess-titled"}
{"type":"user","uuid":"u1","sessionId":"sess-titled","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"hi"}}
{"type":"custom-title","customTitle":"final-title","sessionId":"sess-titled"}
`
	require.NoError(t, os.WriteFile(filepath.Join(jsonlDir, sessionID+".jsonl"), []byte(jsonl), 0644))

	mockRunner := &runner.MockRunner{
		ListSessionsFunc: func() ([]string, error) {
			return []string{"sess-titled", "sess-untitled"}, nil
		},
	}
	claudeClient := claude.NewClient(secretsFile)
	jobMgr := job.NewManager()
	server := NewServer(
		mockRunner, claudeClient,
		auth.Config{Enabled: false}, RateLimitConfig{Enabled: false},
		jobMgr, nil, nil, false, sessionsDir,
		secrets.NewRegistry(&mockTestExecutor{}),
		images.NewRegistry(&mockTestExecutor{}),
		nil,
		"",
	)

	req, err := http.NewRequest(http.MethodGet, "/sessions", nil)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp SessionListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Sessions, 2)

	titles := map[string]string{}
	for _, s := range resp.Sessions {
		titles[s.ID] = s.Title
	}
	assert.Equal(t, "final-title", titles["sess-titled"], "latest custom-title entry should win")
	assert.Equal(t, "", titles["sess-untitled"], "session without a title gets empty Title")
}
