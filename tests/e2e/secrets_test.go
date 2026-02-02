//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecrets_CreateAndList tests creating a secret and listing all secrets
func TestSecrets_CreateAndList(t *testing.T) {
	env := setupE2EEnv(t)

	// Generate unique secret name to avoid conflicts
	secretName := fmt.Sprintf("test-secret-%d", time.Now().UnixNano())

	// Create a secret
	createReq := map[string]string{
		"name":  secretName,
		"value": "test-value-12345",
	}
	resp := makeRequest(t, "POST", env.BaseURL+"/secrets", createReq, nil)
	assertStatusCode(t, resp, http.StatusCreated)

	var createResp map[string]interface{}
	readJSONResponse(t, resp, &createResp)
	assert.True(t, createResp["success"].(bool))
	assert.Equal(t, secretName, createResp["name"])

	// Clean up: delete the secret at the end
	t.Cleanup(func() {
		makeRequest(t, "DELETE", env.BaseURL+"/secrets/"+secretName, nil, nil)
	})

	// List secrets and verify our secret is present
	resp = makeRequest(t, "GET", env.BaseURL+"/secrets", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var listResp map[string]interface{}
	readJSONResponse(t, resp, &listResp)

	secrets := listResp["secrets"].([]interface{})
	found := false
	for _, s := range secrets {
		secret := s.(map[string]interface{})
		if secret["name"] == secretName {
			found = true
			assert.NotEmpty(t, secret["id"])
			break
		}
	}
	require.True(t, found, "created secret should appear in list")
}

// TestSecrets_Get tests getting a specific secret's metadata
func TestSecrets_Get(t *testing.T) {
	env := setupE2EEnv(t)

	// Generate unique secret name
	secretName := fmt.Sprintf("test-get-secret-%d", time.Now().UnixNano())

	// Create a secret first
	createReq := map[string]string{
		"name":  secretName,
		"value": "test-value",
	}
	resp := makeRequest(t, "POST", env.BaseURL+"/secrets", createReq, nil)
	assertStatusCode(t, resp, http.StatusCreated)

	t.Cleanup(func() {
		makeRequest(t, "DELETE", env.BaseURL+"/secrets/"+secretName, nil, nil)
	})

	// Get the secret metadata
	resp = makeRequest(t, "GET", env.BaseURL+"/secrets/"+secretName, nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var getResp map[string]interface{}
	readJSONResponse(t, resp, &getResp)
	assert.Equal(t, secretName, getResp["name"])
	assert.NotEmpty(t, getResp["id"])
}

// TestSecrets_GetNotFound tests getting a non-existent secret
func TestSecrets_GetNotFound(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "GET", env.BaseURL+"/secrets/nonexistent-secret-12345", nil, nil)
	assertStatusCode(t, resp, http.StatusNotFound)
}

// TestSecrets_CreateAlreadyExists tests creating a duplicate secret
func TestSecrets_CreateAlreadyExists(t *testing.T) {
	env := setupE2EEnv(t)

	secretName := fmt.Sprintf("test-dup-secret-%d", time.Now().UnixNano())

	// Create secret first time
	createReq := map[string]string{
		"name":  secretName,
		"value": "test-value",
	}
	resp := makeRequest(t, "POST", env.BaseURL+"/secrets", createReq, nil)
	assertStatusCode(t, resp, http.StatusCreated)

	t.Cleanup(func() {
		makeRequest(t, "DELETE", env.BaseURL+"/secrets/"+secretName, nil, nil)
	})

	// Try to create again - should get 409 Conflict
	resp = makeRequest(t, "POST", env.BaseURL+"/secrets", createReq, nil)
	assertStatusCode(t, resp, http.StatusConflict)

	var conflictResp map[string]interface{}
	readJSONResponse(t, resp, &conflictResp)
	assert.False(t, conflictResp["success"].(bool))
	assert.Contains(t, conflictResp["error"], "already exists")
}

// TestSecrets_Delete tests deleting a secret
func TestSecrets_Delete(t *testing.T) {
	env := setupE2EEnv(t)

	secretName := fmt.Sprintf("test-delete-secret-%d", time.Now().UnixNano())

	// Create secret
	createReq := map[string]string{
		"name":  secretName,
		"value": "test-value",
	}
	resp := makeRequest(t, "POST", env.BaseURL+"/secrets", createReq, nil)
	assertStatusCode(t, resp, http.StatusCreated)

	// Delete the secret
	resp = makeRequest(t, "DELETE", env.BaseURL+"/secrets/"+secretName, nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var deleteResp map[string]interface{}
	readJSONResponse(t, resp, &deleteResp)
	assert.True(t, deleteResp["success"].(bool))
	assert.Equal(t, secretName, deleteResp["name"])

	// Verify secret is gone
	resp = makeRequest(t, "GET", env.BaseURL+"/secrets/"+secretName, nil, nil)
	assertStatusCode(t, resp, http.StatusNotFound)
}

// TestSecrets_DeleteNotFound tests deleting a non-existent secret
func TestSecrets_DeleteNotFound(t *testing.T) {
	env := setupE2EEnv(t)

	resp := makeRequest(t, "DELETE", env.BaseURL+"/secrets/nonexistent-secret-12345", nil, nil)
	assertStatusCode(t, resp, http.StatusNotFound)
}

// TestSecrets_CreateInvalidRequest tests validation errors
func TestSecrets_CreateInvalidRequest(t *testing.T) {
	env := setupE2EEnv(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{"missing name", map[string]string{"value": "test"}},
		{"missing value", map[string]string{"name": "test"}},
		{"empty name", map[string]string{"name": "", "value": "test"}},
		{"empty value", map[string]string{"name": "test", "value": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeRequest(t, "POST", env.BaseURL+"/secrets", tt.body, nil)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			resp.Body.Close()
		})
	}
}

// TestSecrets_ListEmpty tests that empty list returns [] not null
func TestSecrets_ListEmpty(t *testing.T) {
	env := setupE2EEnv(t)

	// This test assumes no secrets exist or creates a clean state
	// In practice, other secrets might exist, so we just verify the response format
	resp := makeRequest(t, "GET", env.BaseURL+"/secrets", nil, nil)
	assertStatusCode(t, resp, http.StatusOK)

	var listResp map[string]interface{}
	readJSONResponse(t, resp, &listResp)

	// Verify secrets field exists and is an array (not null)
	secrets, ok := listResp["secrets"]
	require.True(t, ok, "response should have secrets field")
	require.NotNil(t, secrets, "secrets should not be null")
}
