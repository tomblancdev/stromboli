package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomblanc/stromboli/internal/job"
	"github.com/tomblanc/stromboli/internal/runner"
	"github.com/tomblanc/stromboli/internal/webhook"
)

func TestCancelJob(t *testing.T) {
	t.Run("cancel pending job", func(t *testing.T) {
		server := newTestServer(t, nil, true)

		// Create a job
		jobID := "job-123"
		server.jobMgr.Create(jobID)

		// Cancel it
		req, err := http.NewRequest(http.MethodDelete, "/jobs/"+jobID, nil)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["cancelled"])
		assert.Equal(t, jobID, response["job_id"])

		// Verify job is cancelled
		j, ok := server.jobMgr.Get(jobID)
		require.True(t, ok)
		assert.Equal(t, job.StatusCancelled, j.Status)
	})

	t.Run("cancel running job", func(t *testing.T) {
		server := newTestServer(t, nil, true)

		jobID := "job-456"
		server.jobMgr.Create(jobID)
		server.jobMgr.Update(jobID, job.StatusRunning, "", "", "")

		req, err := http.NewRequest(http.MethodDelete, "/jobs/"+jobID, nil)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, true, response["cancelled"])
	})

	t.Run("cannot cancel completed job", func(t *testing.T) {
		server := newTestServer(t, nil, true)

		jobID := "job-789"
		server.jobMgr.Create(jobID)
		server.jobMgr.Update(jobID, job.StatusCompleted, "output", "", "sess-1")

		req, err := http.NewRequest(http.MethodDelete, "/jobs/"+jobID, nil)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)

		var response RunResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "error", response.Status)
		assert.Contains(t, response.Error, "cannot cancel")
	})

	t.Run("cannot cancel failed job", func(t *testing.T) {
		server := newTestServer(t, nil, true)

		jobID := "job-failed"
		server.jobMgr.Create(jobID)
		server.jobMgr.Update(jobID, job.StatusFailed, "", "error", "")

		req, err := http.NewRequest(http.MethodDelete, "/jobs/"+jobID, nil)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		server := newTestServer(t, nil, true)

		req, err := http.NewRequest(http.MethodDelete, "/jobs/non-existent", nil)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		var response RunResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "error", response.Status)
		assert.Contains(t, response.Error, "not found")
	})
}

func TestRunAsyncWithWebhook(t *testing.T) {
	t.Run("webhook called on success", func(t *testing.T) {
		// Setup webhook receiver
		var receivedPayload webhook.JobResult
		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer webhookServer.Close()

		// Setup mock runner that completes successfully
		mockRunner := &runner.MockRunner{
			RunAsyncFunc: func(ctx context.Context, req runner.Request, jobID string, onComplete func(*runner.Result, error)) {
				// Simulate async completion after a short delay
				go func() {
					time.Sleep(10 * time.Millisecond)
					onComplete(&runner.Result{
						ID:        "run-123",
						Output:    "test output",
						SessionID: "sess-456",
					}, nil)
				}()
			},
		}

		server := newTestServer(t, mockRunner, true)

		// Make async request with webhook
		reqBody := map[string]interface{}{
			"prompt":      "test",
			"webhook_url": webhookServer.URL,
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest(http.MethodPost, "/run/async", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var response AsyncRunResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Wait for webhook to be called
		time.Sleep(50 * time.Millisecond)

		// Verify webhook was called with correct data
		assert.Equal(t, response.JobID, receivedPayload.JobID)
		assert.Equal(t, "completed", receivedPayload.Status)
		assert.Equal(t, "test output", receivedPayload.Output)
		assert.Equal(t, "sess-456", receivedPayload.SessionID)
		assert.Empty(t, receivedPayload.Error)
	})

	t.Run("webhook called on failure", func(t *testing.T) {
		var receivedPayload webhook.JobResult
		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			require.NoError(t, err)
			w.WriteHeader(http.StatusOK)
		}))
		defer webhookServer.Close()

		mockRunner := &runner.MockRunner{
			RunAsyncFunc: func(ctx context.Context, req runner.Request, jobID string, onComplete func(*runner.Result, error)) {
				go func() {
					time.Sleep(10 * time.Millisecond)
					onComplete(nil, assert.AnError)
				}()
			},
		}

		server := newTestServer(t, mockRunner, true)

		reqBody := map[string]interface{}{
			"prompt":      "test",
			"webhook_url": webhookServer.URL,
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest(http.MethodPost, "/run/async", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var response AsyncRunResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Wait for webhook to be called
		time.Sleep(50 * time.Millisecond)

		// Verify webhook was called with error
		assert.Equal(t, response.JobID, receivedPayload.JobID)
		assert.Equal(t, "failed", receivedPayload.Status)
		assert.NotEmpty(t, receivedPayload.Error)
		assert.Empty(t, receivedPayload.Output)
	})

	t.Run("async works without webhook_url", func(t *testing.T) {
		mockRunner := &runner.MockRunner{
			RunAsyncFunc: func(ctx context.Context, req runner.Request, jobID string, onComplete func(*runner.Result, error)) {
				go func() {
					time.Sleep(10 * time.Millisecond)
					onComplete(&runner.Result{ID: "run-123", Output: "test"}, nil)
				}()
			},
		}

		server := newTestServer(t, mockRunner, true)

		reqBody := map[string]interface{}{
			"prompt": "test",
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest(http.MethodPost, "/run/async", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("webhook failure does not affect job", func(t *testing.T) {
		// Webhook that always fails
		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer webhookServer.Close()

		mockRunner := &runner.MockRunner{
			RunAsyncFunc: func(ctx context.Context, req runner.Request, jobID string, onComplete func(*runner.Result, error)) {
				go func() {
					time.Sleep(10 * time.Millisecond)
					onComplete(&runner.Result{ID: "run-123", Output: "test"}, nil)
				}()
			},
		}

		server := newTestServer(t, mockRunner, true)

		reqBody := map[string]interface{}{
			"prompt":      "test",
			"webhook_url": webhookServer.URL,
		}
		body, _ := json.Marshal(reqBody)

		req, err := http.NewRequest(http.MethodPost, "/run/async", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)

		var response AsyncRunResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		// Wait for async completion
		time.Sleep(50 * time.Millisecond)

		// Job should still be completed despite webhook failure
		j, ok := server.jobMgr.Get(response.JobID)
		require.True(t, ok)
		assert.Equal(t, job.StatusCompleted, j.Status)
	})
}
