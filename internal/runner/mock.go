package runner

import (
	"context"
)

// MockRunner implements Runner for testing
type MockRunner struct {
	RunFunc            func(ctx context.Context, req Request) (*Result, error)
	DestroySessionFunc func(sessionID string) error
	ListSessionsFunc   func() ([]string, error)
}

func (m *MockRunner) Run(ctx context.Context, req Request) (*Result, error) {
	return m.RunFunc(ctx, req)
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
