package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockExecutor implements a simple executor for testing
type MockExecutor struct {
	RunFunc func(ctx context.Context, args []string) ([]byte, error)
	calls   [][]string
}

func (m *MockExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	m.calls = append(m.calls, args)
	if m.RunFunc != nil {
		return m.RunFunc(ctx, args)
	}
	return nil, nil
}

func (m *MockExecutor) RunStream(ctx context.Context, args []string) (stdout, stderr chan []byte, done chan error, err error) {
	return nil, nil, nil, errors.New("not implemented")
}

func (m *MockExecutor) GetCalls() [][]string {
	return m.calls
}

func (m *MockExecutor) Reset() {
	m.calls = nil
}

func TestNewRegistry(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	assert.NotNil(t, r)
	assert.Equal(t, exec, r.executor)
}

// Note: TestRegistry_Create_Success is an integration test since Create uses exec.Command directly
// for stdin support. Unit tests focus on validation; integration tests verify actual creation.

func TestRegistry_Create_InvalidName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	tests := []struct {
		name    string
		wantErr string
	}{
		{"", "secret name cannot be empty"},
		{"has spaces", "secret name contains invalid characters"},
		{"has/slash", "secret name contains invalid characters"},
		{"has:colon", "secret name contains invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Create(context.Background(), tt.name, "value")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRegistry_Create_ValidNames(t *testing.T) {
	// Test that valid names pass validation (actual creation is integration tested)
	validNames := []string{
		"simple",
		"with-dashes",
		"with_underscores",
		"With123Numbers",
		"a",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := validateName(name)
			require.NoError(t, err)
		})
	}
}

func TestRegistry_Create_EmptyValue(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	err := r.Create(context.Background(), "my-secret", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret value cannot be empty")
}

// Note: TestRegistry_Create_PodmanError is an integration test since Create uses exec.Command directly

func TestRegistry_List_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Podman outputs newline-delimited JSON with {{json .}} format
			return []byte(`{"ID":"abc123","CreatedAt":"2024-01-15T10:30:00Z","UpdatedAt":"2024-01-15T10:30:00Z","Name":"secret-one"}
{"ID":"def456","CreatedAt":"2024-01-16T11:00:00Z","UpdatedAt":"2024-01-16T11:00:00Z","Name":"secret-two"}`), nil
		},
	}
	r := NewRegistry(exec)

	secrets, err := r.List(context.Background())

	require.NoError(t, err)
	require.Len(t, secrets, 2)

	assert.Equal(t, "secret-one", secrets[0].Name)
	assert.Equal(t, "abc123", secrets[0].ID)

	assert.Equal(t, "secret-two", secrets[1].Name)
	assert.Equal(t, "def456", secrets[1].ID)

	// Should call: podman secret ls --format "{{json .}}"
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "secret", "ls", "--format", "{{json .}}"}, call)
}

func TestRegistry_List_Empty(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			// Podman returns empty output when no secrets exist
			return []byte(""), nil
		},
	}
	r := NewRegistry(exec)

	secrets, err := r.List(context.Background())

	require.NoError(t, err)
	assert.Empty(t, secrets)
}

func TestRegistry_List_PodmanError(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("podman not running")
		},
	}
	r := NewRegistry(exec)

	_, err := r.List(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list secrets")
}

func TestRegistry_Inspect_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte(`[{"ID":"abc123","CreatedAt":"2024-01-15T10:30:00Z","UpdatedAt":"2024-01-15T10:30:00Z","Spec":{"Name":"my-secret","Driver":{"Name":"file","Options":null}}}]`), nil
		},
	}
	r := NewRegistry(exec)

	info, err := r.Inspect(context.Background(), "my-secret")

	require.NoError(t, err)
	assert.Equal(t, "abc123", info.ID)
	assert.Equal(t, "my-secret", info.Name)

	// Should call: podman secret inspect my-secret (default JSON output)
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "secret", "inspect", "my-secret"}, call)
}

func TestRegistry_Inspect_NotFound(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: no secret with name or id \"nonexistent\""), errors.New("exit status 125")
		},
	}
	r := NewRegistry(exec)

	_, err := r.Inspect(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSecretNotFound)
}

func TestRegistry_Delete_Success(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("abc123\n"), nil
		},
	}
	r := NewRegistry(exec)

	err := r.Delete(context.Background(), "my-secret")

	require.NoError(t, err)

	// Should call: podman secret rm my-secret
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "secret", "rm", "my-secret"}, call)
}

func TestRegistry_Delete_NotFound(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return []byte("Error: no secret with name or id \"nonexistent\""), errors.New("exit status 1")
		},
	}
	r := NewRegistry(exec)

	err := r.Delete(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSecretNotFound)
}

func TestRegistry_Delete_EmptyName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	err := r.Delete(context.Background(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret name cannot be empty")
}

func TestRegistry_Exists_True(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, nil // exit code 0 = exists
		},
	}
	r := NewRegistry(exec)

	exists, err := r.Exists(context.Background(), "my-secret")

	require.NoError(t, err)
	assert.True(t, exists)

	// Should call: podman secret exists my-secret
	call := exec.GetCalls()[0]
	assert.Equal(t, []string{"podman", "secret", "exists", "my-secret"}, call)
}

func TestRegistry_Exists_False(t *testing.T) {
	exec := &MockExecutor{
		RunFunc: func(ctx context.Context, args []string) ([]byte, error) {
			return nil, errors.New("exit status 1")
		},
	}
	r := NewRegistry(exec)

	exists, err := r.Exists(context.Background(), "nonexistent")

	require.NoError(t, err)
	assert.False(t, exists)
}
