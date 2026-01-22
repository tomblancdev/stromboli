package runner

import (
	"context"
	"io"
	"strings"
	"sync"
)

// MockExecutor is a test implementation of Executor
type MockExecutor struct {
	mu sync.Mutex

	// RunFunc is called when Run is invoked, if set
	RunFunc func(ctx context.Context, args []string) ([]byte, error)

	// RunStreamFunc is called when RunStream is invoked, if set
	RunStreamFunc func(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)

	// Calls tracks all command invocations
	Calls [][]string

	// DefaultOutput is returned if RunFunc is not set
	DefaultOutput []byte

	// DefaultError is returned if RunFunc is not set
	DefaultError error

	// StreamOutput is the content to stream if RunStreamFunc is not set
	StreamOutput string

	// StreamError is the error content to stream if RunStreamFunc is not set
	StreamError string

	// StreamStartError is returned from start() if set
	StreamStartError error

	// StreamWaitError is returned from wait() if set
	StreamWaitError error
}

// NewMockExecutor creates a new MockExecutor
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Calls: make([][]string, 0),
	}
}

// Run executes the mock command
func (m *MockExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, args)
	m.mu.Unlock()

	if m.RunFunc != nil {
		return m.RunFunc(ctx, args)
	}

	return m.DefaultOutput, m.DefaultError
}

// RunStream executes the mock command with streaming
func (m *MockExecutor) RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, args)
	m.mu.Unlock()

	if m.RunStreamFunc != nil {
		return m.RunStreamFunc(ctx, args)
	}

	// Create pipes from strings
	stdoutReader := io.NopCloser(strings.NewReader(m.StreamOutput))
	stderrReader := io.NopCloser(strings.NewReader(m.StreamError))

	startFn := func() error {
		return m.StreamStartError
	}

	waitFn := func() error {
		return m.StreamWaitError
	}

	return stdoutReader, stderrReader, startFn, waitFn, nil
}

// GetCalls returns all recorded command calls
func (m *MockExecutor) GetCalls() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls
}

// Reset clears all recorded calls
func (m *MockExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = make([][]string, 0)
}
