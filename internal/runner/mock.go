package runner

import (
	"context"
)

// MockRunner implements Runner for testing
type MockRunner struct {
	RunFunc            func(ctx context.Context, req Request) (*Result, error)
	RunStreamFunc      func(ctx context.Context, req Request, output chan<- string) (*Result, error)
	RunAsyncFunc       func(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
	DestroySessionFunc func(sessionID string) error
	ListSessionsFunc   func() ([]string, error)
}

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error) {
	return m.RunFunc(ctx, req)
}

func (m *MockRunner) RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error) {
	if m.RunStreamFunc != nil {
		return m.RunStreamFunc(ctx, req, output)
	}
	// Default implementation: run normally and close channel
	defer close(output)
	return m.Run(ctx, req)
}

func (m *MockRunner) RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error)) {
	if m.RunAsyncFunc != nil {
		m.RunAsyncFunc(ctx, req, jobID, onComplete)
		return
	}
	// Default implementation: run synchronously in goroutine
	go func() {
		result, err := m.Run(ctx, req)
		onComplete(result, err)
	}()
}

func (m *MockRunner) DestroySession(sessionID string) error {
	if m.DestroySessionFunc != nil {
		return m.DestroySessionFunc(sessionID)
	}
	return nil
}

func (m *MockRunner) ListSessions() ([]string, error) {
	if m.ListSessionsFunc != nil {
		return m.ListSessionsFunc()
	}
	return []string{}, nil
}
