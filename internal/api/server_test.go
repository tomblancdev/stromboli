package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	server := NewServer()

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
	server := NewServer()

	req, err := http.NewRequest(http.MethodGet, "/claude/status", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, false, response["configured"])
	assert.Contains(t, response["message"], "stromboli claude init")
}
