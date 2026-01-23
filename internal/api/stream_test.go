package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/runner"
)

func TestRunStream_MissingPrompt_ReturnsBadRequest(t *testing.T) {
	server := newTestServer(t, nil, true)

	req, err := http.NewRequest(http.MethodGet, "/run/stream", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRunStream_NotConfigured_ReturnsServiceUnavailable(t *testing.T) {
	server := newTestServer(t, nil, false)

	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=hello", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestRunStream_Success(t *testing.T) {
	mockRunner := &runner.MockRunner{
		RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
			// Simulate streaming output
			output <- "Line 1"
			output <- "Line 2"
			output <- "Line 3"
			close(output)

			return &runner.Result{
				ID:        "run-123",
				Output:    "Line 1\nLine 2\nLine 3",
				SessionID: "session-456",
			}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=hello", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	// Read response body
	body := rec.Body.String()
	assert.Contains(t, body, "data: Line 1")
	assert.Contains(t, body, "data: Line 2")
	assert.Contains(t, body, "data: Line 3")
	assert.Contains(t, body, "event: done")
	assert.Contains(t, body, `"session_id":"session-456"`)
}

func TestRunStream_WithWorkspaceAndSessionID(t *testing.T) {
	var capturedReq runner.Request
	mockRunner := &runner.MockRunner{
		RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
			capturedReq = req
			output <- "Processing"
			close(output)

			return &runner.Result{
				ID:        "run-1",
				Output:    "Processing",
				SessionID: "sess-123",
			}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=analyze&workspace=/project&session_id=sess-123", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "analyze", capturedReq.Prompt)
	assert.Equal(t, "/project", capturedReq.Workspace)
	assert.Equal(t, "sess-123", capturedReq.Claude.SessionID)
}

func TestRunStream_ClientDisconnect(t *testing.T) {
	blockChan := make(chan struct{})
	mockRunner := &runner.MockRunner{
		RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
			output <- "Line 1"

			// Wait for context cancellation
			<-ctx.Done()
			close(output)

			return nil, ctx.Err()
		},
	}

	server := newTestServer(t, mockRunner, true)

	// Create a request with a cancellable context
	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=hello", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)

	// Start the handler in a goroutine
	rec := httptest.NewRecorder()
	go func() {
		server.router.ServeHTTP(rec, req)
		close(blockChan)
	}()

	// Cancel the request (simulate client disconnect)
	cancel()

	// Wait for handler to finish
	<-blockChan

	// The handler should have stopped gracefully
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRunStream_ExecutionError(t *testing.T) {
	mockRunner := &runner.MockRunner{
		RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
			output <- "Starting..."
			close(output)
			return nil, assert.AnError
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=fail", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	server.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: error")
}

func TestSSEFormat(t *testing.T) {
	// Test SSE formatting directly
	tests := []struct {
		name     string
		lines    []string
		expected []string
	}{
		{
			name:  "single line",
			lines: []string{"Hello"},
			expected: []string{
				"data: Hello\n\n",
			},
		},
		{
			name:  "multiple lines",
			lines: []string{"Line 1", "Line 2", "Line 3"},
			expected: []string{
				"data: Line 1\n\n",
				"data: Line 2\n\n",
				"data: Line 3\n\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &runner.MockRunner{
				RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
					for _, line := range tt.lines {
						output <- line
					}
					close(output)

					return &runner.Result{
						ID:        "run-1",
						Output:    strings.Join(tt.lines, "\n"),
						SessionID: "sess-1",
					}, nil
				},
			}

			server := newTestServer(t, mockRunner, true)

			req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=test", nil)
			require.NoError(t, err)

			rec := httptest.NewRecorder()
			server.router.ServeHTTP(rec, req)

			body := rec.Body.String()
			for _, expected := range tt.expected {
				assert.Contains(t, body, expected)
			}
		})
	}
}

func TestRunStream_StreamingFlusher(t *testing.T) {
	// Verify that the handler uses flusher for real-time streaming
	mockRunner := &runner.MockRunner{
		RunStreamFunc: func(ctx context.Context, req runner.Request, output chan<- string) (*runner.Result, error) {
			output <- "Test line"
			close(output)
			return &runner.Result{ID: "run-1", SessionID: "sess-1"}, nil
		},
	}

	server := newTestServer(t, mockRunner, true)

	req, err := http.NewRequest(http.MethodGet, "/run/stream?prompt=test", nil)
	require.NoError(t, err)

	// Use a custom ResponseRecorder that implements Flusher
	rec := &flushingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}

	server.router.ServeHTTP(rec, req)

	assert.True(t, rec.flushed, "Response should have been flushed")
}

// flushingRecorder is a custom recorder that tracks Flush calls
type flushingRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (r *flushingRecorder) Flush() {
	r.flushed = true
}

func (r *flushingRecorder) Write(b []byte) (int, error) {
	return r.ResponseRecorder.Write(b)
}

func (r *flushingRecorder) WriteString(s string) (int, error) {
	return io.WriteString(r.ResponseRecorder, s)
}
