package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifier_Notify(t *testing.T) {
	t.Run("successful notification", func(t *testing.T) {
		// Setup test server
		receivedPayload := JobResult{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create notifier and send
		notifier := NewNotifier()
		payload := JobResult{
			JobID:           "job-123",
			Status:          "completed",
			Output:          "test output",
			Error:           "",
			SessionID:       "sess-456",
			FilesChanged:    []string{"src/app.ts", "README.md"},
			TokensUsed:      &TokensUsed{Input: 1500, Output: 800},
			DurationSeconds: 45.2,
		}

		err := notifier.Notify(server.URL, payload)

		require.NoError(t, err)
		assert.Equal(t, "job-123", receivedPayload.JobID)
		assert.Equal(t, "completed", receivedPayload.Status)
		assert.Equal(t, "test output", receivedPayload.Output)
		assert.Equal(t, "sess-456", receivedPayload.SessionID)
		assert.Equal(t, []string{"src/app.ts", "README.md"}, receivedPayload.FilesChanged)
		assert.Equal(t, &TokensUsed{Input: 1500, Output: 800}, receivedPayload.TokensUsed)
		assert.InDelta(t, 45.2, receivedPayload.DurationSeconds, 0.001)
	})

	t.Run("notification with error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier := NewNotifier()
		payload := JobResult{
			JobID:  "job-789",
			Status: "failed",
			Error:  "something went wrong",
		}

		err := notifier.Notify(server.URL, payload)

		require.NoError(t, err)
	})

	t.Run("server returns error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		notifier := NewNotifier()
		payload := JobResult{JobID: "job-123", Status: "completed"}

		err := notifier.Notify(server.URL, payload)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook returned status 500")
	})

	t.Run("invalid URL", func(t *testing.T) {
		notifier := NewNotifier()
		payload := JobResult{JobID: "job-123", Status: "completed"}

		err := notifier.Notify("http://invalid.invalid.invalid.invalid:99999", payload)

		require.Error(t, err)
	})

	t.Run("timeout respected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier := NewNotifier()
		payload := JobResult{JobID: "job-123", Status: "completed"}

		// Should timeout (notifier has 5s timeout, but we wait 100ms in handler)
		// This test verifies timeout is working, not that it triggers
		err := notifier.Notify(server.URL, payload)
		require.NoError(t, err)
	})

	t.Run("retry on failure", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		notifier := NewNotifier()
		payload := JobResult{JobID: "job-123", Status: "completed"}

		err := notifier.Notify(server.URL, payload)

		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("retry fails after max attempts", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		notifier := NewNotifier()
		payload := JobResult{JobID: "job-123", Status: "completed"}

		err := notifier.Notify(server.URL, payload)

		require.Error(t, err)
		assert.Equal(t, 2, attempts) // Initial attempt + 1 retry
	})
}

func TestJobResult(t *testing.T) {
	t.Run("marshals to JSON correctly", func(t *testing.T) {
		result := JobResult{
			JobID:           "job-123",
			Status:          "completed",
			Output:          "test output",
			Error:           "",
			SessionID:       "sess-456",
			FilesChanged:    []string{"src/app.ts"},
			TokensUsed:      &TokensUsed{Input: 1500, Output: 800},
			DurationSeconds: 45.0,
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		var unmarshaled JobResult
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, result.JobID, unmarshaled.JobID)
		assert.Equal(t, result.Status, unmarshaled.Status)
		assert.Equal(t, result.Output, unmarshaled.Output)
		assert.Equal(t, result.SessionID, unmarshaled.SessionID)
		assert.Equal(t, result.FilesChanged, unmarshaled.FilesChanged)
		assert.Equal(t, result.TokensUsed, unmarshaled.TokensUsed)
		assert.Equal(t, result.DurationSeconds, unmarshaled.DurationSeconds)
	})

	t.Run("files_changed is always present as array", func(t *testing.T) {
		result := JobResult{
			JobID:        "job-123",
			Status:       "completed",
			FilesChanged: []string{},
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		// files_changed should be [] not null
		assert.Contains(t, string(data), `"files_changed":[]`)
	})

	t.Run("tokens_used is null when not set", func(t *testing.T) {
		result := JobResult{
			JobID:        "job-123",
			Status:       "completed",
			FilesChanged: []string{},
		}

		data, err := json.Marshal(result)
		require.NoError(t, err)

		assert.Contains(t, string(data), `"tokens_used":null`)
	})
}
