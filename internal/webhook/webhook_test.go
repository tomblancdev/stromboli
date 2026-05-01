package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
			// Unsigned notifiers must NOT add signature headers.
			assert.Empty(t, r.Header.Get(HeaderSignature))
			assert.Empty(t, r.Header.Get(HeaderTimestamp))

			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Create notifier and send
		notifier := NewNotifier()
		payload := JobResult{
			JobID:     "job-123",
			Status:    "completed",
			Output:    "test output",
			Error:     "",
			SessionID: "sess-456",
		}

		err := notifier.Notify(server.URL, payload)

		require.NoError(t, err)
		assert.Equal(t, "job-123", receivedPayload.JobID)
		assert.Equal(t, "completed", receivedPayload.Status)
		assert.Equal(t, "test output", receivedPayload.Output)
		assert.Equal(t, "sess-456", receivedPayload.SessionID)
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
		var attempts int
		done := make(chan struct{}, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			select {
			case done <- struct{}{}:
			default:
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
			JobID:     "job-123",
			Status:    "completed",
			Output:    "test output",
			Error:     "",
			SessionID: "sess-456",
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
	})
}

func TestNotifier_Signed(t *testing.T) {
	const secret = "shared-webhook-secret-please-rotate-me-32-bytes"

	t.Run("signed notification carries signature headers and verifies", func(t *testing.T) {
		var (
			gotSig  string
			gotTS   string
			gotBody []byte
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotSig = r.Header.Get(HeaderSignature)
			gotTS = r.Header.Get(HeaderTimestamp)
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier := NewSignedNotifier(secret)
		err := notifier.Notify(server.URL, JobResult{JobID: "j-1", Status: "completed"})
		require.NoError(t, err)

		require.NotEmpty(t, gotSig, "signed notifier must set X-Stromboli-Signature")
		require.NotEmpty(t, gotTS, "signed notifier must set X-Stromboli-Timestamp")
		require.True(t, len(gotSig) > len(signaturePrefix) && gotSig[:len(signaturePrefix)] == signaturePrefix,
			"signature must be prefixed with 'sha256='")

		// The receiver's verification path must accept the signature.
		require.NoError(t,
			Verify([]byte(secret), gotTS, gotSig, gotBody, 5*time.Minute))
	})

	t.Run("empty secret falls back to unsigned", func(t *testing.T) {
		var sig string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sig = r.Header.Get(HeaderSignature)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier := NewSignedNotifier("") // empty secret
		require.NoError(t, notifier.Notify(server.URL, JobResult{JobID: "j-1"}))
		assert.Empty(t, sig, "empty secret must not sign — caller's responsibility to validate config")
	})

	t.Run("retry reuses the same timestamp+signature", func(t *testing.T) {
		var (
			tsValues  []string
			sigValues []string
			attempts  int
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			tsValues = append(tsValues, r.Header.Get(HeaderTimestamp))
			sigValues = append(sigValues, r.Header.Get(HeaderSignature))
			if attempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		notifier := NewSignedNotifier(secret)
		require.NoError(t, notifier.Notify(server.URL, JobResult{JobID: "j-retry"}))

		require.Equal(t, 2, attempts)
		require.Equal(t, tsValues[0], tsValues[1], "retry must reuse the original timestamp")
		require.Equal(t, sigValues[0], sigValues[1], "retry must reuse the original signature")
	})
}

func TestVerify(t *testing.T) {
	const secret = "shared-webhook-secret-please-rotate-me-32-bytes"
	body := []byte(`{"job_id":"j-1","status":"completed"}`)
	now := strconv.FormatInt(time.Now().Unix(), 10)
	goodSig := signaturePrefix + sign([]byte(secret), now, body)

	t.Run("accepts a valid signature", func(t *testing.T) {
		require.NoError(t, Verify([]byte(secret), now, goodSig, body, time.Minute))
	})

	t.Run("rejects when the body is altered", func(t *testing.T) {
		err := Verify([]byte(secret), now, goodSig, append(body, '!'), time.Minute)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "signature mismatch")
	})

	t.Run("rejects with the wrong secret", func(t *testing.T) {
		err := Verify([]byte("a-totally-different-secret-that-is-32-chr"), now, goodSig, body, time.Minute)
		require.Error(t, err)
	})

	t.Run("rejects when the timestamp is too old", func(t *testing.T) {
		old := strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)
		oldSig := signaturePrefix + sign([]byte(secret), old, body)
		err := Verify([]byte(secret), old, oldSig, body, 10*time.Minute)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "freshness")
	})

	t.Run("rejects empty headers", func(t *testing.T) {
		require.Error(t, Verify([]byte(secret), "", goodSig, body, time.Minute))
		require.Error(t, Verify([]byte(secret), now, "", body, time.Minute))
	})

	t.Run("rejects empty secret", func(t *testing.T) {
		require.Error(t, Verify(nil, now, goodSig, body, time.Minute))
	})

	t.Run("accepts signatures without the sha256= prefix (interop)", func(t *testing.T) {
		raw := sign([]byte(secret), now, body)
		require.NoError(t, Verify([]byte(secret), now, raw, body, time.Minute))
	})
}
