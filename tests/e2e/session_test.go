//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionLifecycle(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("list sessions initially", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var sessions []string
		readJSONResponse(t, resp, &sessions)

		// Should return an array (might be empty or have sessions from previous tests)
		assert.NotNil(t, sessions)
		t.Logf("Found %d existing sessions", len(sessions))
	})

	t.Run("create session via execution", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Remember: the secret word is 'banana'",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		sessionID, ok := result["session_id"].(string)
		require.True(t, ok, "session_id should be present")
		require.NotEmpty(t, sessionID)

		t.Logf("Created session: %s", sessionID)

		// Verify session appears in list
		resp = makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var sessions []string
		readJSONResponse(t, resp, &sessions)

		assert.Contains(t, sessions, sessionID, "session should appear in list")
	})

	t.Run("resume session", func(t *testing.T) {
		// Create initial session
		payload := map[string]interface{}{
			"prompt": "Remember: my name is Alice",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		sessionID := result["session_id"].(string)
		t.Logf("Created session: %s", sessionID)

		// Resume session and ask about previous context
		payload = map[string]interface{}{
			"prompt": "What is my name?",
			"claude": map[string]interface{}{
				"model":      "claude-3-5-haiku-20241022",
				"session_id": sessionID,
				"resume":     true,
			},
		}

		resp = makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		readJSONResponse(t, resp, &result)

		assert.Equal(t, "success", result["status"])
		assert.Equal(t, sessionID, result["session_id"])

		output := result["output"].(string)
		t.Logf("Response: %s", output)

		// Claude should remember the name from previous interaction
		// Note: We don't assert the exact output as Claude's response may vary
	})

	t.Run("continue session", func(t *testing.T) {
		// Create initial session
		payload := map[string]interface{}{
			"prompt": "Let's count together. I'll start: 1",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		sessionID := result["session_id"].(string)

		// Continue session
		payload = map[string]interface{}{
			"prompt": "What comes next?",
			"claude": map[string]interface{}{
				"model":      "claude-3-5-haiku-20241022",
				"session_id": sessionID,
				"continue":   true,
			},
		}

		resp = makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		readJSONResponse(t, resp, &result)
		assert.Equal(t, sessionID, result["session_id"])
		t.Logf("Continue response: %s", result["output"])
	})

	t.Run("destroy session", func(t *testing.T) {
		// Create session
		payload := map[string]interface{}{
			"prompt": "Test session to destroy",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		sessionID := result["session_id"].(string)
		t.Logf("Created session to destroy: %s", sessionID)

		// Verify it exists
		resp = makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
		defer resp.Body.Close()

		var sessions []string
		readJSONResponse(t, resp, &sessions)
		assert.Contains(t, sessions, sessionID)

		// Destroy session
		resp = makeRequest(t, "DELETE", env.BaseURL+"/sessions/"+sessionID, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var destroyResult map[string]interface{}
		readJSONResponse(t, resp, &destroyResult)
		assert.True(t, destroyResult["success"].(bool))

		// Verify it's gone
		resp = makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
		defer resp.Body.Close()

		readJSONResponse(t, resp, &sessions)
		assert.NotContains(t, sessions, sessionID, "session should be removed from list")
	})

	t.Run("destroy non-existent session", func(t *testing.T) {
		resp := makeRequest(t, "DELETE", env.BaseURL+"/sessions/non-existent-session", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusNotFound)
		assertErrorResponse(t, resp)
	})
}

func TestSessionWithNoPersistence(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("no persistence option", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Test with no persistence",
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

		// Session ID might still be returned
		sessionID, hasSession := result["session_id"].(string)

		if hasSession && sessionID != "" {
			// If session ID returned, verify it doesn't persist
			resp = makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
			defer resp.Body.Close()

			var sessions []string
			readJSONResponse(t, resp, &sessions)

			// Session should not be in the list (not persisted)
			// Note: This depends on implementation - some systems might still list it
			t.Logf("Session ID: %s, persisted: %v", sessionID, contains(sessions, sessionID))
		}
	})
}

func TestSessionForkAndOptions(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("fork session", func(t *testing.T) {
		// Create initial session
		payload := map[string]interface{}{
			"prompt": "Remember: color is red",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		originalSessionID := result["session_id"].(string)

		// Fork session
		payload = map[string]interface{}{
			"prompt": "What color did I mention?",
			"claude": map[string]interface{}{
				"model":        "claude-3-5-haiku-20241022",
				"session_id":   originalSessionID,
				"fork_session": true,
			},
		}

		resp = makeRequest(t, "POST", env.BaseURL+"/run", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		readJSONResponse(t, resp, &result)

		forkedSessionID := result["session_id"].(string)

		// Forked session should have different ID
		assert.NotEqual(t, originalSessionID, forkedSessionID, "forked session should have new ID")

		t.Logf("Original session: %s", originalSessionID)
		t.Logf("Forked session: %s", forkedSessionID)

		// Both sessions should exist
		resp = makeRequest(t, "GET", env.BaseURL+"/sessions", nil, nil)
		defer resp.Body.Close()

		var sessions []string
		readJSONResponse(t, resp, &sessions)

		assert.Contains(t, sessions, originalSessionID)
		assert.Contains(t, sessions, forkedSessionID)
	})
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
