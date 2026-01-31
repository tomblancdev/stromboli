//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncEndpoint(t *testing.T) {
	env := setupE2EEnv(t)

	t.Run("missing prompt returns 400", func(t *testing.T) {
		payload := map[string]interface{}{
			"workdir": "/tmp",
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		assertStatusCode(t, resp, http.StatusBadRequest)
		assertErrorResponse(t, resp)
	})

	t.Run("async execution returns job id", func(t *testing.T) {
		skipIfNoClaude(t, env)

		payload := map[string]interface{}{
			"prompt": "What is 5+5? Reply with only the number.",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusAccepted)

		var result map[string]interface{}
		readJSONResponse(t, resp, &result)

		jobID, ok := result["job_id"].(string)
		require.True(t, ok, "job_id should be present")
		require.NotEmpty(t, jobID)

		t.Logf("Created async job: %s", jobID)
	})
}

func TestJobManagement(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("list jobs returns empty array initially", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/jobs", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var jobs []map[string]interface{}
		readJSONResponse(t, resp, &jobs)

		// Jobs array may not be empty if previous tests ran
		assert.NotNil(t, jobs)
	})

	t.Run("create and retrieve job", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Count to 3",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		// Create job
		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusAccepted)

		var createResult map[string]interface{}
		readJSONResponse(t, resp, &createResult)

		jobID := createResult["job_id"].(string)

		// Wait a moment for job to start
		time.Sleep(500 * time.Millisecond)

		// Retrieve job status
		resp = makeRequest(t, "GET", env.BaseURL+"/jobs/"+jobID, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		var job map[string]interface{}
		readJSONResponse(t, resp, &job)

		assert.Equal(t, jobID, job["id"])
		assert.Contains(t, []string{"pending", "running", "completed", "failed"}, job["status"])
		assert.NotEmpty(t, job["created_at"])
	})

	t.Run("wait for job completion", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "What is 7+3? Reply with only the number.",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		// Create job
		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		var createResult map[string]interface{}
		readJSONResponse(t, resp, &createResult)

		jobID := createResult["job_id"].(string)

		// Wait for completion (with timeout)
		job := waitForJobCompletion(t, env, jobID, 2*time.Minute)

		assert.Equal(t, "completed", job["status"])
		assert.NotEmpty(t, job["result"])

		result := job["result"].(map[string]interface{})
		assert.NotEmpty(t, result["output"])
		assert.Equal(t, "success", result["status"])

		t.Logf("Job completed with output: %s", result["output"])
	})

	t.Run("get non-existent job returns 404", func(t *testing.T) {
		resp := makeRequest(t, "GET", env.BaseURL+"/jobs/non-existent-id", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusNotFound)
		assertErrorResponse(t, resp)
	})

	t.Run("cancel running job", func(t *testing.T) {
		payload := map[string]interface{}{
			"prompt": "Count to 1000 slowly",
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		// Create job
		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		var createResult map[string]interface{}
		readJSONResponse(t, resp, &createResult)

		jobID := createResult["job_id"].(string)

		// Wait for job to start
		time.Sleep(1 * time.Second)

		// Cancel job
		resp = makeRequest(t, "DELETE", env.BaseURL+"/jobs/"+jobID, nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusOK)

		// Verify job is cancelled
		time.Sleep(500 * time.Millisecond)

		resp = makeRequest(t, "GET", env.BaseURL+"/jobs/"+jobID, nil, nil)
		defer resp.Body.Close()

		var job map[string]interface{}
		readJSONResponse(t, resp, &job)

		// Job should be cancelled or completed (if it finished before cancel)
		assert.Contains(t, []string{"cancelled", "completed"}, job["status"])
	})

	t.Run("cancel non-existent job returns 404", func(t *testing.T) {
		resp := makeRequest(t, "DELETE", env.BaseURL+"/jobs/non-existent-id", nil, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusNotFound)
		assertErrorResponse(t, resp)
	})
}

func TestAsyncWithWebhook(t *testing.T) {
	env := setupE2EEnv(t)
	skipIfNoClaude(t, env)

	t.Run("webhook called on completion", func(t *testing.T) {
		// Create webhook server
		var receivedPayload map[string]interface{}
		webhookCalled := make(chan bool, 1)

		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedPayload)
			webhookCalled <- true
			w.WriteHeader(http.StatusOK)
		}))
		defer webhookServer.Close()

		// Create async job with webhook
		payload := map[string]interface{}{
			"prompt":      "Say 'webhook test'",
			"webhook_url": webhookServer.URL,
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusAccepted)

		var createResult map[string]interface{}
		readJSONResponse(t, resp, &createResult)

		jobID := createResult["job_id"].(string)

		// Wait for webhook (with timeout)
		select {
		case <-webhookCalled:
			t.Log("Webhook was called")

			// Verify payload
			assert.NotNil(t, receivedPayload)
			assert.Equal(t, jobID, receivedPayload["job_id"])
			assert.NotEmpty(t, receivedPayload["status"])
			assert.NotEmpty(t, receivedPayload["result"])

		case <-time.After(2 * time.Minute):
			t.Fatal("Webhook was not called within timeout")
		}
	})

	t.Run("webhook failure does not affect job", func(t *testing.T) {
		// Create webhook server that returns error
		webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer webhookServer.Close()

		// Create async job with webhook
		payload := map[string]interface{}{
			"prompt":      "Test with failing webhook",
			"webhook_url": webhookServer.URL,
			"claude": map[string]interface{}{
				"model": "claude-3-5-haiku-20241022",
			},
		}

		resp := makeRequest(t, "POST", env.BaseURL+"/run/async", payload, nil)
		defer resp.Body.Close()

		assertStatusCode(t, resp, http.StatusAccepted)

		var createResult map[string]interface{}
		readJSONResponse(t, resp, &createResult)

		jobID := createResult["job_id"].(string)

		// Job should still complete successfully
		job := waitForJobCompletion(t, env, jobID, 2*time.Minute)
		assert.Equal(t, "completed", job["status"])
	})
}
