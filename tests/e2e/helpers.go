//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// httpClient is a shared HTTP client for E2E tests
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// makeRequest is a helper to make HTTP requests in E2E tests
func makeRequest(t *testing.T, method, url string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err, "failed to marshal request body")
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err, "failed to create request")

	// Set default headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	require.NoError(t, err, "failed to execute request")

	return resp
}

// makeRequestWithContext is a helper to make HTTP requests with context
func makeRequestWithContext(t *testing.T, ctx context.Context, method, url string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err, "failed to marshal request body")
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	require.NoError(t, err, "failed to create request")

	// Set default headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	require.NoError(t, err, "failed to execute request")

	return resp
}

// readJSONResponse reads and unmarshals JSON response
func readJSONResponse(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")

	err = json.Unmarshal(body, target)
	require.NoError(t, err, "failed to unmarshal response: %s", string(body))
}

// readStringResponse reads response body as string
func readStringResponse(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")

	return string(body)
}

// assertStatusCode checks the HTTP status code
func assertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
	}
}

// waitForJobCompletion polls a job until it reaches a terminal state
func waitForJobCompletion(t *testing.T, env *testEnv, jobID string, timeout time.Duration) map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for job %s to complete", jobID)
		case <-ticker.C:
			resp := makeRequest(t, "GET", env.BaseURL+"/jobs/"+jobID, nil, nil)
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}

			var job map[string]interface{}
			readJSONResponse(t, resp, &job)

			status, ok := job["status"].(string)
			if !ok {
				t.Fatalf("invalid job response: missing status")
			}

			if status == "completed" || status == "failed" || status == "cancelled" {
				return job
			}
		}
	}
}

// readSSEStream reads Server-Sent Events from a response
func readSSEStream(t *testing.T, resp *http.Response, timeout time.Duration) []string {
	t.Helper()
	defer resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var events []string
	buffer := make([]byte, 4096)
	accumulated := ""

	done := make(chan struct{})
	go func() {
		for {
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				accumulated += string(buffer[:n])

				// Split by double newline (end of SSE message)
				parts := strings.Split(accumulated, "\n\n")
				for i := 0; i < len(parts)-1; i++ {
					if parts[i] != "" {
						events = append(events, parts[i])
					}
				}
				accumulated = parts[len(parts)-1]
			}
			if err != nil {
				if err != io.EOF {
					t.Logf("error reading SSE stream: %v", err)
				}
				close(done)
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return events
	case <-done:
		return events
	}
}

// parseSSEEvent parses a single SSE event
func parseSSEEvent(event string) (eventType string, data string) {
	lines := strings.Split(event, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	return
}

// skipIfNoClaude skips the test if Claude is not configured
func skipIfNoClaude(t *testing.T, env *testEnv) {
	t.Helper()
	if !env.HasClaude {
		t.Skip("Skipping test: Claude token not available (set ANTHROPIC_API_KEY)")
	}
}

// requireClaude requires Claude to be configured or fails the test
func requireClaude(t *testing.T, env *testEnv) {
	t.Helper()
	if !env.HasClaude {
		t.Fatal("Test requires Claude token (set ANTHROPIC_API_KEY)")
	}
}

// assertErrorResponse checks that response contains an error field
func assertErrorResponse(t *testing.T, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	require.NoError(t, err, "failed to unmarshal error response")

	_, hasError := errorResp["error"]
	require.True(t, hasError, "response should contain error field: %s", string(body))
}

// createTestWorkspace creates a temporary workspace directory for tests
func createTestWorkspace(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Create some test files
	err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test content"), 0644)
	require.NoError(t, err)

	return dir
}

// pollUntil polls a condition function until it returns true or timeout
func pollUntil(t *testing.T, condition func() bool, interval, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for condition")
		case <-ticker.C:
			continue
		}
	}
}
