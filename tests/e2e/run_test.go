//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunEndpoint(t *testing.T) {
	env := setupE2EEnv(t)

	t.Run("missing prompt returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"workdir": "/tmp",
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		assertStatusCode(t, resp, http.StatusBadRequest)
		assertErrorResponse(t, resp)
	})

	t.Run("empty prompt returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "",
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		assertStatusCode(t, resp, http.StatusBadRequest)
		assertErrorResponse(t, resp)
	})

	t.Run("invalid json returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("POST", env.BaseURL+"/run", nil)
		req.Header.Set("Content-Type", "application/json")
		resp, _ := httpClient.Do(req)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusBadRequest)
	})

	t.Run("valid prompt without claude", func(t *testing.T) {
		if env.HasClaude {
			t.Skip("Skipping: This test requires Claude to be unconfigured")
		}

		payload := map[string]interface{}{
			"prompt": "echo 'hello world'",
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		// Should succeed even without Claude (just container execution)
		if resp.StatusCode == http.StatusOK {
			var result map[string]interface{}
			readJSONResponse(t, resp, &result)

			assert.Equal(t, "success", result["status"])
			assert.NotEmpty(t, result["output"])
		} else {
			// Or return error if Claude required
			assertStatusCode(t, resp, http.StatusInternalServerError)
		}
	})

	t.Run("valid prompt with claude", func(t *testing.T) {
		skipIfNoClaude(t, env)

		payload := map[string]interface{}{
			"prompt": "What is 2+2? Reply with only the number.",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		assert.NotEmpty(t, result["output"])
		t.Logf("Claude response: %s", result["output"])
	})

	t.Run("execution with session persistence", func(t *testing.T) {
		skipIfNoClaude(t, env)

		payload := map[string]interface{}{
			"prompt": "Remember: my favorite color is blue",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		require.Equal(t, "success", result["status"])
		sessionID, ok := result["session_id"].(string)
		require.True(t, ok, "session_id should be present")
		require.NotEmpty(t, sessionID)

		t.Logf("Created session: %s", sessionID)
	})

	t.Run("execution with podman options", func(t *testing.T) {
		skipIfNoClaude(t, env)

		payload := map[string]interface{}{
			"prompt": "What is the date?",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
			"podman": map[string]interface{}{
				"memory": "256m",
				"cpus":   "0.5",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		assert.NotEmpty(t, result["output"])
	})
}

func TestRunEndpointWithClaudeOptions(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("with system prompt", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "What should I do?",
			"claude": map[string]interface{}{
				"model":         "claude-3-5-haiku-20241022",
				"system_prompt": "You are a helpful assistant. Always respond with 'I can help with that!'",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		t.Logf("Response with system prompt: %s", result["output"])
	})

	t.Run("with verbose output", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Say hello",
			"claude": map[string]interface{}{
				"model":   "claude-3-5-haiku-20241022",
				"verbose": true,
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		assert.NotEmpty(t, result["output"])
	})

	t.Run("with no persistence", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Quick test",
			"claude": map[string]interface{}{
				"model":          "claude-3-5-haiku-20241022",
				"no_persistence": true,
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		// Session ID might still be returned but session won't be persisted
	})
}
