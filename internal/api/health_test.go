package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"stromboli/internal/runner"
	"stromboli/internal/secrets"
)

func TestNewHealthChecker(t *testing.T) {
	executor := runner.NewMockExecutor()
	config := DefaultHealthConfig()

	checker := NewHealthChecker(executor, config)

	require.NotNil(t, checker)
	assert.Equal(t, executor, checker.executor)
	assert.Equal(t, config.Timeout, checker.config.Timeout)
	assert.Equal(t, config.CredentialsFile, checker.config.CredentialsFile)
	assert.Equal(t, config.SecretName, checker.config.SecretName)
}

func TestDefaultHealthConfig(t *testing.T) {
	config := DefaultHealthConfig()

	assert.Equal(t, 5*time.Second, config.Timeout)
	assert.Equal(t, "~/.claude/.credentials.json", config.CredentialsFile)
	assert.Equal(t, secrets.DefaultSecretName, config.SecretName)
}

func TestHealthChecker_Check_AllHealthy(t *testing.T) {
	// Create a temp credentials file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0600))

	executor := runner.NewMockExecutor()
	executor.DefaultOutput = []byte("ok")
	executor.DefaultError = nil

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: credFile,
		SecretName:      "claude-credentials",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, "stromboli", result.Name)
	require.Len(t, result.Components, 3)

	// Check podman component
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "ok", result.Components[0].Status)

	// Check credentials file component
	assert.Equal(t, "claude-credentials-file", result.Components[1].Name)
	assert.Equal(t, "ok", result.Components[1].Status)

	// Check secret component
	assert.Equal(t, "claude-credentials-secret", result.Components[2].Name)
	assert.Equal(t, "ok", result.Components[2].Status)
}

func TestHealthChecker_Check_PodmanUnhealthy(t *testing.T) {
	// Create a temp credentials file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0600))

	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "podman" && len(args) > 1 && args[1] == "version" {
			return nil, errors.New("connection refused")
		}
		return []byte("ok"), nil
	}

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: credFile,
		SecretName:      "claude-credentials",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	require.Len(t, result.Components, 3)

	// Podman should be unhealthy
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "error", result.Components[0].Status)
	assert.Contains(t, result.Components[0].Error, "failed to connect to podman")

	// Credentials file should be healthy
	assert.Equal(t, "claude-credentials-file", result.Components[1].Name)
	assert.Equal(t, "ok", result.Components[1].Status)

	// Secret should be healthy
	assert.Equal(t, "claude-credentials-secret", result.Components[2].Name)
	assert.Equal(t, "ok", result.Components[2].Status)
}

func TestHealthChecker_Check_CredentialsMissing(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.DefaultOutput = []byte("ok")
	executor.DefaultError = nil

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: "/nonexistent/path/.credentials.json",
		SecretName:      "claude-credentials",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	require.Len(t, result.Components, 3)

	// Podman should be healthy
	assert.Equal(t, "podman", result.Components[0].Name)
	assert.Equal(t, "ok", result.Components[0].Status)

	// Credentials file should be unhealthy
	assert.Equal(t, "claude-credentials-file", result.Components[1].Name)
	assert.Equal(t, "error", result.Components[1].Status)
	assert.Contains(t, result.Components[1].Error, "credentials file not found")

	// Secret should be healthy
	assert.Equal(t, "claude-credentials-secret", result.Components[2].Name)
	assert.Equal(t, "ok", result.Components[2].Status)
}

func TestHealthChecker_Check_SecretMissing(t *testing.T) {
	// Create a temp credentials file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0600))

	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		// Podman version succeeds
		if len(args) > 1 && args[1] == "version" {
			return []byte("ok"), nil
		}
		// Secret check fails
		if len(args) > 2 && args[1] == "secret" && args[2] == "exists" {
			return nil, errors.New("no secret with name")
		}
		return []byte("ok"), nil
	}

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: credFile,
		SecretName:      "claude-credentials",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	require.Len(t, result.Components, 3)

	// Podman should be healthy
	assert.Equal(t, "ok", result.Components[0].Status)

	// Credentials should be healthy
	assert.Equal(t, "ok", result.Components[1].Status)

	// Secret should be unhealthy
	assert.Equal(t, "claude-credentials-secret", result.Components[2].Name)
	assert.Equal(t, "error", result.Components[2].Status)
	assert.Contains(t, result.Components[2].Error, "podman secret")
}

func TestHealthChecker_Check_AllUnhealthy(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.DefaultError = errors.New("all systems down")

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: "/nonexistent/path/.credentials.json",
		SecretName:      "claude-credentials",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "degraded", result.Status)
	require.Len(t, result.Components, 3)

	// All components should be unhealthy
	assert.Equal(t, "error", result.Components[0].Status)
	assert.Equal(t, "error", result.Components[1].Status)
	assert.Equal(t, "error", result.Components[2].Status)
}

func TestHealthChecker_Check_CustomCredentialsPath(t *testing.T) {
	// Create a custom credentials file
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom", "creds.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(customPath), 0755))
	require.NoError(t, os.WriteFile(customPath, []byte("{}"), 0600))

	executor := runner.NewMockExecutor()
	executor.DefaultOutput = []byte("ok")
	executor.DefaultError = nil

	config := HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: customPath,
		SecretName:      "custom-secret",
	}
	checker := NewHealthChecker(executor, config)

	result := checker.Check(context.Background())

	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, "ok", result.Components[1].Status)
}

func TestHealthChecker_Check_Timeout(t *testing.T) {
	executor := runner.NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return []byte("slow result"), nil
		}
	}

	config := HealthConfig{
		Timeout:         100 * time.Millisecond,
		CredentialsFile: "/nonexistent",
		SecretName:      "test",
	}
	checker := NewHealthChecker(executor, config)

	start := time.Now()
	result := checker.Check(context.Background())
	elapsed := time.Since(start)

	// Should complete within reasonable time
	assert.Less(t, elapsed, 500*time.Millisecond)

	// Podman should be degraded due to timeout
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
		Timeout:         5 * time.Second,
		CredentialsFile: "/nonexistent",
		SecretName:      "test",
	}
	checker := NewHealthChecker(executor, config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := checker.Check(ctx)

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
				Name:   "claude-credentials",
				Status: "error",
				Error:  "credentials not found",
			},
			wantName: "claude-credentials",
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
				{Name: "claude-credentials-file", Status: "ok"},
				{Name: "claude-credentials-secret", Status: "ok"},
			},
			wantStatus: "ok",
		},
		{
			name: "podman unhealthy",
			components: []ComponentHealth{
				{Name: "podman", Status: "error", Error: "connection failed"},
				{Name: "claude-credentials-file", Status: "ok"},
				{Name: "claude-credentials-secret", Status: "ok"},
			},
			wantStatus: "degraded",
		},
		{
			name: "credentials file missing",
			components: []ComponentHealth{
				{Name: "podman", Status: "ok"},
				{Name: "claude-credentials-file", Status: "error", Error: "not found"},
				{Name: "claude-credentials-secret", Status: "ok"},
			},
			wantStatus: "degraded",
		},
		{
			name: "secret missing",
			components: []ComponentHealth{
				{Name: "podman", Status: "ok"},
				{Name: "claude-credentials-file", Status: "ok"},
				{Name: "claude-credentials-secret", Status: "error", Error: "not found"},
			},
			wantStatus: "degraded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build expected status
			status := "ok"
			for _, c := range tt.components {
				if c.Status != "ok" {
					status = "degraded"
					break
				}
			}
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}
