package compose

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthChecker_WaitForHealthy_Immediate(t *testing.T) {
	executor := NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte(`[{"Name":"dev-1","Service":"dev","State":"running","Health":"healthy"}]`), nil
	}

	checker := NewHealthChecker(executor)
	err := checker.WaitForHealthy(context.Background(), "test-project", 5*time.Second)
	require.NoError(t, err)
}

func TestHealthChecker_WaitForHealthy_EventuallyHealthy(t *testing.T) {
	executor := NewMockExecutor()
	callCount := 0
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		callCount++
		if callCount < 3 {
			// First two calls return starting
			return []byte(`[{"Name":"dev-1","Service":"dev","State":"running","Health":"starting"}]`), nil
		}
		// Third call returns healthy
		return []byte(`[{"Name":"dev-1","Service":"dev","State":"running","Health":"healthy"}]`), nil
	}

	checker := NewHealthChecker(executor)
	err := checker.WaitForHealthy(context.Background(), "test-project", 10*time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, callCount, 3)
}

func TestHealthChecker_WaitForHealthy_Timeout(t *testing.T) {
	executor := NewMockExecutor()
	executor.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		// Always return unhealthy
		return []byte(`[{"Name":"dev-1","Service":"dev","State":"starting","Health":"starting"}]`), nil
	}

	checker := NewHealthChecker(executor)
	err := checker.WaitForHealthy(context.Background(), "test-project", 100*time.Millisecond)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrComposeHealthTimeout)
}

func TestHealthChecker_ParseServicesStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []ServiceStatus
		wantErr  bool
	}{
		{
			name:  "single service healthy",
			input: `[{"Name":"dev-1","Service":"dev","State":"running","Health":"healthy"}]`,
			expected: []ServiceStatus{
				{Name: "dev", State: "running", Health: "healthy"},
			},
		},
		{
			name:  "multiple services",
			input: `[{"Name":"dev-1","Service":"dev","State":"running","Health":""},{"Name":"db-1","Service":"db","State":"running","Health":"healthy"}]`,
			expected: []ServiceStatus{
				{Name: "dev", State: "running", Health: ""},
				{Name: "db", State: "running", Health: "healthy"},
			},
		},
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
	}

	checker := NewHealthChecker(NewMockExecutor())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := checker.parseServicesStatus([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, services)
			}
		})
	}
}

func TestIsServiceHealthy(t *testing.T) {
	tests := []struct {
		name     string
		service  ServiceStatus
		expected bool
	}{
		{
			name:     "running with no health check",
			service:  ServiceStatus{Name: "dev", State: "running", Health: ""},
			expected: true,
		},
		{
			name:     "running healthy",
			service:  ServiceStatus{Name: "dev", State: "running", Health: "healthy"},
			expected: true,
		},
		{
			name:     "running health none",
			service:  ServiceStatus{Name: "dev", State: "running", Health: "none"},
			expected: true,
		},
		{
			name:     "running health dash",
			service:  ServiceStatus{Name: "dev", State: "running", Health: "-"},
			expected: true,
		},
		{
			name:     "up state",
			service:  ServiceStatus{Name: "dev", State: "Up", Health: ""},
			expected: true,
		},
		{
			name:     "up with time",
			service:  ServiceStatus{Name: "dev", State: "Up 5 minutes", Health: ""},
			expected: true,
		},
		{
			name:     "starting state",
			service:  ServiceStatus{Name: "dev", State: "starting", Health: ""},
			expected: false,
		},
		{
			name:     "exited state",
			service:  ServiceStatus{Name: "dev", State: "exited", Health: ""},
			expected: false,
		},
		{
			name:     "running but unhealthy",
			service:  ServiceStatus{Name: "dev", State: "running", Health: "unhealthy"},
			expected: false,
		},
		{
			name:     "running but starting health",
			service:  ServiceStatus{Name: "dev", State: "running", Health: "starting"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServiceHealthy(tt.service)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatUnhealthyServices(t *testing.T) {
	tests := []struct {
		name     string
		services []ServiceStatus
		contains []string
	}{
		{
			name: "single unhealthy",
			services: []ServiceStatus{
				{Name: "dev", State: "starting", Health: ""},
			},
			contains: []string{"dev", "starting"},
		},
		{
			name: "multiple unhealthy",
			services: []ServiceStatus{
				{Name: "dev", State: "starting", Health: ""},
				{Name: "db", State: "running", Health: "unhealthy"},
			},
			contains: []string{"dev", "starting", "db", "unhealthy"},
		},
		{
			name:     "empty",
			services: []ServiceStatus{},
			contains: []string{"unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatUnhealthyServices(tt.services)
			for _, s := range tt.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}
