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

func TestRegistry_Create_NameTooLong(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	// Create a name that exceeds max length
	longName := string(make([]byte, MaxSecretNameLength+1))
	for i := range longName {
		longName = longName[:i] + "a" + longName[i+1:]
	}

	err := r.Create(context.Background(), longName, "value")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestRegistry_Create_ValueTooLarge(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	// Create a value that exceeds max size
	largeValue := string(make([]byte, MaxSecretValueSize+1))

	err := r.Create(context.Background(), "valid-name", largeValue)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")
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

func TestRegistry_Inspect_InvalidName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	tests := []struct {
		name    string
		wantErr string
	}{
		{"", "secret name cannot be empty"},
		{"has spaces", "secret name contains invalid characters"},
		{"has/slash", "secret name contains invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Inspect(context.Background(), tt.name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

func TestRegistry_Delete_InvalidName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	tests := []struct {
		name    string
		wantErr string
	}{
		{"", "secret name cannot be empty"},
		{"has spaces", "secret name contains invalid characters"},
		{"has/slash", "secret name contains invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Delete(context.Background(), tt.name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

func TestRegistry_Exists_InvalidName(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	tests := []struct {
		name    string
		wantErr string
	}{
		{"", "secret name cannot be empty"},
		{"has spaces", "secret name contains invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r.Exists(context.Background(), tt.name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
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

// Edge case tests for validation

func TestValidateName_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// Valid edge cases
		{"single char", "a", false, ""},
		{"single digit", "1", false, ""},
		{"single underscore leading", "_", false, ""},
		{"single dash leading", "-", false, ""},
		{"all numbers", "123456", false, ""},
		{"mixed case", "AbCdEf", false, ""},
		{"max length exactly", string(make([]byte, MaxSecretNameLength)), false, ""},

		// Invalid characters
		{"space in middle", "hello world", true, "invalid characters"},
		{"leading space", " hello", true, "invalid characters"},
		{"trailing space", "hello ", true, "invalid characters"},
		{"tab character", "hello\tworld", true, "invalid characters"},
		{"newline character", "hello\nworld", true, "invalid characters"},
		{"forward slash", "hello/world", true, "invalid characters"},
		{"backslash", "hello\\world", true, "invalid characters"},
		{"colon", "hello:world", true, "invalid characters"},
		{"dot", "hello.world", true, "invalid characters"},
		{"at sign", "hello@world", true, "invalid characters"},
		{"hash", "hello#world", true, "invalid characters"},
		{"dollar", "hello$world", true, "invalid characters"},
		{"percent", "hello%world", true, "invalid characters"},
		{"ampersand", "hello&world", true, "invalid characters"},
		{"asterisk", "hello*world", true, "invalid characters"},
		{"parentheses", "hello(world)", true, "invalid characters"},
		{"brackets", "hello[world]", true, "invalid characters"},
		{"braces", "hello{world}", true, "invalid characters"},
		{"pipe", "hello|world", true, "invalid characters"},
		{"semicolon", "hello;world", true, "invalid characters"},
		{"quote", "hello'world", true, "invalid characters"},
		{"double quote", "hello\"world", true, "invalid characters"},
		{"backtick", "hello`world", true, "invalid characters"},
		{"tilde", "hello~world", true, "invalid characters"},
		{"caret", "hello^world", true, "invalid characters"},
		{"equals", "hello=world", true, "invalid characters"},
		{"plus", "hello+world", true, "invalid characters"},
		{"less than", "hello<world", true, "invalid characters"},
		{"greater than", "hello>world", true, "invalid characters"},
		{"question mark", "hello?world", true, "invalid characters"},

		// Unicode characters (should be invalid)
		{"unicode emoji", "hello🎉world", true, "invalid characters"},
		{"unicode accent", "héllo", true, "invalid characters"},
		{"unicode chinese", "你好", true, "invalid characters"},
		{"unicode cyrillic", "привет", true, "invalid characters"},

		// Length edge cases
		{"empty string", "", true, "cannot be empty"},
		{"one over max", string(make([]byte, MaxSecretNameLength+1)), true, "exceeds maximum length"},

		// Command injection attempts
		{"shell command", "$(whoami)", true, "invalid characters"},
		{"backtick command", "`whoami`", true, "invalid characters"},
		{"pipe injection", "name|cat /etc/passwd", true, "invalid characters"},
		{"semicolon injection", "name;cat /etc/passwd", true, "invalid characters"},
		{"newline injection", "name\ncat /etc/passwd", true, "invalid characters"},
		{"null byte", "name\x00value", true, "invalid characters"},
	}

	// Fill the max length and over max strings with valid chars
	maxLenBytes := make([]byte, MaxSecretNameLength)
	for i := range maxLenBytes {
		maxLenBytes[i] = 'a'
	}
	overMaxLenBytes := make([]byte, MaxSecretNameLength+1)
	for i := range overMaxLenBytes {
		overMaxLenBytes[i] = 'a'
	}

	// Update the test inputs by name to avoid index errors
	for i := range tests {
		switch tests[i].name {
		case "max length exactly":
			tests[i].input = string(maxLenBytes)
		case "one over max":
			tests[i].input = string(overMaxLenBytes)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if tt.wantErr {
				require.Error(t, err, "expected error for input: %q", tt.input)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err, "unexpected error for input: %q", tt.input)
			}
		})
	}
}

func TestValidateName_BoundaryConditions(t *testing.T) {
	// Test exactly at max length (should pass)
	maxLengthName := make([]byte, MaxSecretNameLength)
	for i := range maxLengthName {
		maxLengthName[i] = 'x'
	}
	err := validateName(string(maxLengthName))
	require.NoError(t, err, "name at max length should be valid")

	// Test one over max length (should fail)
	overMaxName := make([]byte, MaxSecretNameLength+1)
	for i := range overMaxName {
		overMaxName[i] = 'x'
	}
	err = validateName(string(overMaxName))
	require.Error(t, err, "name over max length should be invalid")
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestRegistry_Create_ValueBoundaryConditions(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	// Test exactly at max size (this will fail at Podman call, but should pass validation)
	// We can't fully test this without mocking exec.Command, but we can verify the boundary

	// Test one over max size
	overMaxValue := make([]byte, MaxSecretValueSize+1)
	err := r.Create(context.Background(), "valid-name", string(overMaxValue))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum size")

	// Test empty value
	err = r.Create(context.Background(), "valid-name", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidationErrors_AreWrappedWithErrValidation(t *testing.T) {
	exec := &MockExecutor{}
	r := NewRegistry(exec)

	tests := []struct {
		name      string
		operation func() error
	}{
		{"empty name in Create", func() error { return r.Create(context.Background(), "", "value") }},
		{"invalid name in Create", func() error { return r.Create(context.Background(), "invalid/name", "value") }},
		{"empty value in Create", func() error { return r.Create(context.Background(), "valid", "") }},
		{"value too large", func() error { return r.Create(context.Background(), "valid", string(make([]byte, MaxSecretValueSize+1))) }},
		{"empty name in Inspect", func() error { _, err := r.Inspect(context.Background(), ""); return err }},
		{"invalid name in Inspect", func() error { _, err := r.Inspect(context.Background(), "invalid/name"); return err }},
		{"empty name in Delete", func() error { return r.Delete(context.Background(), "") }},
		{"invalid name in Delete", func() error { return r.Delete(context.Background(), "invalid/name") }},
		{"empty name in Exists", func() error { _, err := r.Exists(context.Background(), ""); return err }},
		{"invalid name in Exists", func() error { _, err := r.Exists(context.Background(), "invalid/name"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrValidation, "validation errors should wrap ErrValidation")
		})
	}
}
