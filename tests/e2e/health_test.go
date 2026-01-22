//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	env := setupE2EEnv(t)

	t.Run("health check returns 200", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/health", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var health map[string]interface{}
		readJSONResponse(t, resp, &health)

		assert.Equal(t, "ok", health["status"])
		assert.NotEmpty(t, health["timestamp"])
	})

	t.Run("metrics endpoint returns 200", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/metrics", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		body := readStringResponse(t, resp)
		assert.Contains(t, body, "# HELP")
	})
}

func TestClaudeStatus(t *testing.T) {
	env := setupE2EEnv(t)

	t.Run("claude status endpoint", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/claude/status", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var status map[string]interface{}
		readJSONResponse(t, resp, &status)

		configured, ok := status["configured"].(bool)
		assert.True(t, ok, "configured field should be boolean")

		// Should match our environment setup
		assert.Equal(t, env.HasClaude, configured)

		if configured {
			t.Log("Claude is configured and available")
		} else {
			t.Log("Claude is not configured (ANTHROPIC_API_KEY not set)")
		}
	})
}

func TestInvalidEndpoints(t *testing.T) {
	env := setupE2EEnv(t)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "404 for unknown endpoint",
			method:         "GET",
			path:           "/unknown",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "405 for wrong method on health",
			method:         "POST",
			path:           "/health",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "405 for wrong method on claude status",
			method:         "POST",
			path:           "/claude/status",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := makeRequest(t, tc.method, env.BaseURL+tc.path, nil, nil)
			defer resp.Body.Close()

			assertStatusCode(t, resp, tc.expectedStatus)
		})
	}
}
