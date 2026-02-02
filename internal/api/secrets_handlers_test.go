package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/secrets"
)

// mockSecretsExecutor implements secrets.Executor for testing
type mockSecretsExecutor struct {
	runFunc func(ctx context.Context, args []string) ([]byte, error)
}

func (m *mockSecretsExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, args)
	}
	return nil, nil
}

func setupSecretsRouter(registry *secrets.Registry) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewSecretsHandler(registry)

	router.GET("/secrets", handler.List)
	router.POST("/secrets", handler.Create)
	router.GET("/secrets/:name", handler.Get)
	router.DELETE("/secrets/:name", handler.Delete)

	return router
}

func TestSecretsHandler_List_Success(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Podman outputs newline-delimited JSON
			return []byte(`{"ID":"abc123","CreatedAt":"2024-01-15T10:30:00Z","UpdatedAt":"2024-01-15T10:30:00Z","Name":"secret-one"}
{"ID":"def456","CreatedAt":"2024-01-16T11:00:00Z","UpdatedAt":"2024-01-16T11:00:00Z","Name":"secret-two"}`), nil
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SecretsListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Secrets, 2)
	assert.Equal(t, "secret-one", response.Secrets[0].Name)
	assert.Equal(t, "abc123", response.Secrets[0].ID)
}

func TestSecretsHandler_List_Empty(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Podman returns empty output when no secrets exist
			return []byte(``), nil
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SecretsListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response.Secrets)
}

func TestSecretsHandler_List_Error(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("podman not running")
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response SecretsListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response.Error, "failed to list secrets")
}

// Note: TestSecretsHandler_Create_Success is skipped as unit test because Create() uses exec.Command
// directly for stdin support. This should be tested as an integration test with real Podman.

// Note: TestSecretsHandler_Create_AlreadyExists is an integration test since Create() uses exec.Command
// directly for stdin support. The "already exists" detection relies on Podman's actual error output.
// See tests/e2e/ for integration tests that verify this scenario with real Podman.

func TestSecretsHandler_Create_InvalidRequest(t *testing.T) {
	exec := &mockSecretsExecutor{}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"value":"secret"}`},
		{"missing value", `{"name":"my-secret"}`},
		{"invalid json", `{invalid}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/secrets", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestSecretsHandler_Get_Success(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[{"ID":"abc123","CreatedAt":"2024-01-15T10:30:00Z","UpdatedAt":"2024-01-15T10:30:00Z","Spec":{"Name":"my-secret"}}]`), nil
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets/my-secret", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SecretInfoResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "my-secret", response.Name)
	assert.Equal(t, "abc123", response.ID)
}

func TestSecretsHandler_Get_NotFound(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`Error: no secret with name or id "nonexistent"`), errors.New("exit status 125")
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSecretsHandler_Delete_Success(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("abc123\n"), nil
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("DELETE", "/secrets/my-secret", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response DeleteSecretResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "my-secret", response.Name)
}

func TestSecretsHandler_Delete_NotFound(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`Error: no secret with name or id "nonexistent"`), errors.New("exit status 1")
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("DELETE", "/secrets/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response DeleteSecretResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "not found")
}

func TestNewSecretsHandler(t *testing.T) {
	exec := &mockSecretsExecutor{}
	registry := secrets.NewRegistry(exec)
	handler := NewSecretsHandler(registry)

	assert.NotNil(t, handler)
	assert.Equal(t, registry, handler.registry)
}

// TestSecretsHandler_List_PassesThroughTime verifies time is passed through as-is
func TestSecretsHandler_List_PassesThroughTime(t *testing.T) {
	exec := &mockSecretsExecutor{
		runFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Podman outputs relative time strings in list format
			return []byte(`{"ID":"abc123","CreatedAt":"10 days ago","Name":"test-secret"}`), nil
		},
	}
	registry := secrets.NewRegistry(exec)
	router := setupSecretsRouter(registry)

	req, _ := http.NewRequest("GET", "/secrets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SecretsListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	// Time is passed through as-is (Podman uses relative times in list format)
	assert.Equal(t, "10 days ago", response.Secrets[0].CreatedAt)
}
