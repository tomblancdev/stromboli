//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamEndpoint(t *testing.T) {
	env := setupE2EEnv(t)

	t.Run("missing prompt returns 400", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/run/stream?workdir=/tmp", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusBadRequest)
	})

	t.Run("empty prompt returns 400", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/run/stream?prompt=", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusBadRequest)
	})

	t.Run("stream execution with claude", func(t *testing.T) {
		skipIfNoClaude(t, env)

		url := env.BaseURL + "/run/stream?prompt=What+is+3%2B3?+Reply+with+only+the+number."
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		// Verify SSE headers
		assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
		assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))

		// Read SSE events with timeout
		events := readSSEStream(t, resp, 2*time.Minute)

		require.NotEmpty(t, events, "should receive at least one SSE event")

		// Parse events
		var hasOutput bool
		var hasComplete bool

		for _, event := range events {
			eventType, data := parseSSEEvent(event)

			t.Logf("Received SSE event: type=%s, data=%s", eventType, data)

			switch eventType {
			case "output":
				hasOutput = true
				assert.NotEmpty(t, data)
			case "complete":
				hasComplete = true
				assert.NotEmpty(t, data)
			case "error":
				t.Fatalf("Received error event: %s", data)
			}
		}

		assert.True(t, hasOutput, "should receive at least one output event")
		assert.True(t, hasComplete, "should receive complete event")
	})

	t.Run("stream with claude options", func(t *testing.T) {
		skipIfNoClaude(t, env)

		url := env.BaseURL + "/run/stream?prompt=Say+hello&claude.model=claude-3-5-haiku-20241022&claude.verbose=true"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		// Read stream
		events := readSSEStream(t, resp, 2*time.Minute)
		require.NotEmpty(t, events)

		t.Logf("Received %d SSE events", len(events))
	})

	t.Run("stream client disconnect", func(t *testing.T) {
		skipIfNoClaude(t, env)

		// Create request with short timeout to simulate disconnect
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		url := env.BaseURL + "/run/stream?prompt=Count+to+100+slowly"
		resp := makeRequestWithContext(t, ctx, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		// Try to read stream (will be interrupted by context cancellation)
		events := readSSEStream(t, resp, 3*time.Second)

		t.Logf("Received %d events before disconnect", len(events))
		// Should receive some events but not necessarily complete
	})

	t.Run("stream with system prompt", func(t *testing.T) {
		skipIfNoClaude(t, env)

		url := env.BaseURL + "/run/stream?prompt=Help+me&claude.system_prompt=Always+respond+with+exactly+the+word+TEST&claude.model=claude-3-5-haiku-20241022"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		events := readSSEStream(t, resp, 2*time.Minute)
		require.NotEmpty(t, events)

		// Check that we received output
		var outputData string
		for _, event := range events {
			eventType, data := parseSSEEvent(event)
			if eventType == "output" {
				outputData += data
			}
		}

		assert.NotEmpty(t, outputData)
		t.Logf("Stream output: %s", outputData)
	})
}

func TestStreamSSEFormat(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("verify SSE message format", func(t *testing.T) {
		url := env.BaseURL + "/run/stream?prompt=Hi&claude.model=claude-3-5-haiku-20241022"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		events := readSSEStream(t, resp, 2*time.Minute)
		require.NotEmpty(t, events)

		for _, event := range events {
			// SSE format should have "event:" and "data:" lines
			assert.True(t, strings.Contains(event, "event:") || strings.Contains(event, "data:"),
				"Event should contain 'event:' or 'data:' prefix")

			eventType, data := parseSSEEvent(event)

			// Event type should be one of the expected types
			assert.Contains(t, []string{"output", "complete", "error", "session"}, eventType,
				"Event type should be recognized")

			// Data should not be empty for most events
			if eventType != "session" {
				assert.NotEmpty(t, data, "Data should not be empty for %s event", eventType)
			}
		}
	})

	t.Run("complete event contains result", func(t *testing.T) {
		url := env.BaseURL + "/run/stream?prompt=Say+OK&claude.model=claude-3-5-haiku-20241022"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		events := readSSEStream(t, resp, 2*time.Minute)
		require.NotEmpty(t, events)

		// Find complete event
		var completeData string
		for _, event := range events {
			eventType, data := parseSSEEvent(event)
			if eventType == "complete" {
				completeData = data
				break
			}
		}

		require.NotEmpty(t, completeData, "should have complete event")

		// Complete event should contain JSON with result
		assert.True(t, strings.Contains(completeData, "output") || strings.Contains(completeData, "status"),
			"Complete event should contain result data")
	})
}

func TestStreamWithPodmanOptions(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("stream with memory limit", func(t *testing.T) {
		url := env.BaseURL + "/run/stream?prompt=Hello&podman.memory=256m&claude.model=claude-3-5-haiku-20241022"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		events := readSSEStream(t, resp, 2*time.Minute)
		require.NotEmpty(t, events)

		// Should complete successfully with memory limit
		var hasComplete bool
		for _, event := range events {
			eventType, _ := parseSSEEvent(event)
			if eventType == "complete" {
				hasComplete = true
				break
			}
		}

		assert.True(t, hasComplete, "should complete successfully")
	})

	t.Run("stream with timeout", func(t *testing.T) {
		// Use a very short timeout to force timeout
		url := env.BaseURL + "/run/stream?prompt=Count+to+1000&podman.timeout=1s&claude.model=claude-3-5-haiku-20241022"
		resp := makeRequest(t, "GET", url, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		events := readSSEStream(t, resp, 10*time.Second)

		// Should receive events (might timeout or complete depending on prompt execution speed)
		t.Logf("Received %d events with short timeout", len(events))
	})
}
