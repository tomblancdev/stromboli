package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/runner"
)

func TestNewHealthChecker(t *testing.T) {
	executor := runner.NewMockExecutor()
	config := DefaultHealthConfig()

	checker := NewHealthChecker(executor, config)

	require.NotNil(t, checker)
	assert.Equal(t, executor, checker.executor)
	assert.Equal(t, config.Timeout, checker.config.Timeout)
	assert.Equal(t, config.SecretName, checker.config.SecretName)
}

func TestDefaultHealthConfig(t *testing.T) {
	config := DefaultHealthConfig()

	assert.Equal(t, 5*time.Second, config.Timeout)
	assert.Equal(t, "claude-token", config.SecretName)
}

func TestHealthChecker_Check_AllHealthy(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.DefaultOutput = []byte("podman version 5.0.0")
	executor.DefaultError = nil

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, "stromboli", result.Name)
	require.Len(t, result.Components, 2)

	// Check podman component
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "ok", result.Components[0].Status)
	assert.Empty(t, result.Components[0].Error)

	// Check secret component
	assert.Equal(t, "claude-secret", result.Components[1].Name)
	assert.Equal(t, "ok", result.Components[1].Status)
	assert.Empty(t, result.Components[1].Error)

	// Verify correct commands were called
	calls := executor.GetCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"podman", "version"}, calls[0])
	assert.Equal(t, []string{"podman", "secret", "exists", "claude-token"}, calls[1])
}

func TestHealthChecker_Check_PodmanUnhealthy(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "podman" && args[1] == "version" {
			return nil, errors.New("connection refused")
		}
		return []byte(""), nil
	}

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	assert.Equal(t, "stromboli", result.Name)
	require.Len(t, result.Components, 2)

	// Podman should be unhealthy
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "error", result.Components[0].Status)
	assert.Contains(t, result.Components[0].Error, "failed to connect to podman")
	assert.Contains(t, result.Components[0].Error, "connection refused")

	// Secret should be healthy
	assert.Equal(t, "claude-secret", result.Components[1].Name)
	assert.Equal(t, "ok", result.Components[1].Status)
}

func TestHealthChecker_Check_SecretMissing(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "podman" && len(args) > 2 && args[1] == "secret" {
			return nil, errors.New("Error: no secret with name or id \"claude-token\": no such secret")
		}
		return []byte("podman version 5.0.0"), nil
	}

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	assert.Equal(t, "stromboli", result.Name)
	require.Len(t, result.Components, 2)

	// Podman should be healthy
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "ok", result.Components[0].Status)

	// Secret should be unhealthy
	assert.Equal(t, "claude-secret", result.Components[1].Name)
	assert.Equal(t, "error", result.Components[1].Status)
	assert.Contains(t, result.Components[1].Error, "secret 'claude-token' not found")
}

func TestHealthChecker_Check_AllUnhealthy(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.DefaultError = errors.New("all systems down")

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	require.Len(t, result.Components, 2)

	// Both components should be unhealthy
	assert.Equal(t, "error", result.Components[0].Status)
	assert.Equal(t, "error", result.Components[1].Status)
}

func TestHealthChecker_Check_CustomSecretName(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.DefaultOutput = []byte("")
	executor.DefaultError = nil

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "my-custom-secret",
	}
	checker := NewHealthChecker(executor, config)

	checker.Check(context.Background())

	// Verify custom secret name was used
	calls := executor.GetCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"podman", "secret", "exists", "my-custom-secret"}, calls[1])
}

func TestHealthChecker_Check_Timeout(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		// Simulate a slow command that respects context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return []byte("slow result"), nil
		}
	}

	config := HealthConfig{
		Timeout:    100 * time.Millisecond, // Short timeout for testing
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	start := time.Now()
	result := checker.Check(context.Background())
	elapsed := time.Since(start)

	// Should complete within reasonable time (2 * timeout + buffer)
	assert.Less(t, elapsed, 500*time.Millisecond)

	// Both should be degraded due to timeout
	assert.Equal(t, "degraded", result.Status)
	assert.Equal(t, "error", result.Components[0].Status)
	assert.Contains(t, result.Components[0].Error, "context deadline exceeded")
}

func TestHealthChecker_Check_ContextCancellation(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return []byte("result"), nil
		}
	}

	config := HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
	checker := NewHealthChecker(executor, config)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := checker.Check(ctx)

	// Should be degraded due to cancelled context
	assert.Equal(t, "degraded", result.Status)
}

func TestComponentHealth_JSON(t *testing.T) {
	tests := []struct {
		name     string
		health   ComponentHealth
		wantName string
		wantErr  bool
	}{
		{
			name: "healthy component",
			health: ComponentHealth{
				Name:   "podman",
				Status: "ok",
			},
			wantName: "podman",
			wantErr:  false,
		},
		{
			name: "unhealthy component with error",
			health: ComponentHealth{
				Name:   "claude-secret",
				Status: "error",
				Error:  "secret not found",
			},
			wantName: "claude-secret",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.health.Name)
			if tt.wantErr {
				assert.NotEmpty(t, tt.health.Error)
			} else {
				assert.Empty(t, tt.health.Error)
			}
		})
	}
}

func TestDetailedHealth_Status(t *testing.T) {
	tests := []struct {
		name       string
		components []ComponentHealth
		wantStatus string
	}{
		{
			name: "all healthy",
			components: []ComponentHealth{
				{Name: "podman", Status: "ok"},
				{Name: "claude-secret", Status: "ok"},
			},
			wantStatus: "ok",
		},
		{
			name: "first component unhealthy",
			components: []ComponentHealth{
				{Name: "podman", Status: "error", Error: "connection failed"},
				{Name: "claude-secret", Status: "ok"},
			},
			wantStatus: "degraded",
		},
		{
			name: "second component unhealthy",
			components: []ComponentHealth{
				{Name: "podman", Status: "ok"},
				{Name: "claude-secret", Status: "error", Error: "not found"},
			},
			wantStatus: "degraded",
		},
		{
			name: "all unhealthy",
			components: []ComponentHealth{
				{Name: "podman", Status: "error", Error: "connection failed"},
				{Name: "claude-secret", Status: "error", Error: "not found"},
			},
			wantStatus: "degraded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := runner.NewMockExecutor()
			config := DefaultHealthConfig()

			// Set up executor to return appropriate results
			callIndex := 0
			executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
				idx := callIndex
				callIndex++
				if idx < len(tt.components) && tt.components[idx].Status != "ok" {
					return nil, errors.New(tt.components[idx].Error)
				}
				return []byte("ok"), nil
			}

			checker := NewHealthChecker(executor, config)
			result := checker.Check(context.Background())

			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}
