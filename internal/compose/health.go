package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	strerrors "stromboli/internal/errors"
)

// HealthCheckResult represents the result of a health check poll
type HealthCheckResult struct {
	// AllHealthy is true if all services are running/healthy
	AllHealthy bool

	// Services contains the status of each service
	Services []ServiceStatus

	// Error contains any error that occurred during the check
	Error error
}

// PsOutput represents the JSON output from podman compose ps
type PsOutput struct {
	Name     string `json:"Name"`
	State    string `json:"State"`
	Health   string `json:"Health"`
	Service  string `json:"Service"`
	ExitCode int    `json:"ExitCode,omitempty"`
}

// HealthChecker provides health check functionality for compose stacks
type HealthChecker struct {
	executor commandExecutor
}

// commandExecutor is the interface for executing commands
type commandExecutor interface {
	Run(ctx context.Context, args []string) ([]byte, error)
}

// NewHealthChecker creates a new HealthChecker
func NewHealthChecker(executor commandExecutor) *HealthChecker {
	return &HealthChecker{executor: executor}
}

// WaitForHealthy polls the stack until all services are healthy or timeout
func (h *HealthChecker) WaitForHealthy(ctx context.Context, projectName string, timeout time.Duration) error {
	pollInterval := 2 * time.Second

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Track last successful result for error reporting
	var lastGoodResult HealthCheckResult

	// Do an initial check immediately
	result := h.checkHealth(ctx, projectName)
	if result.Error != nil {
		return fmt.Errorf("health check failed: %w", result.Error)
	}
	if result.AllHealthy {
		return nil
	}
	lastGoodResult = result

	for {
		select {
		case <-ctx.Done():
			// Calculate status timeout that scales with configured timeout
			// Use 1/4 of the configured timeout, bounded between 1s and 10s
			statusTimeout := timeout / 4
			if statusTimeout < 1*time.Second {
				statusTimeout = 1 * time.Second
			}
			if statusTimeout > 10*time.Second {
				statusTimeout = 10 * time.Second
			}

			// Get final status for error message
			statusCtx, statusCancel := context.WithTimeout(context.Background(), statusTimeout)
			lastResult := h.checkHealth(statusCtx, projectName)
			statusCancel()

			// Use last successful result if final check fails
			if lastResult.Error != nil {
				if len(lastGoodResult.Services) > 0 {
					return fmt.Errorf("%w: %s", strerrors.ErrComposeHealthTimeout, formatUnhealthyServices(lastGoodResult.Services))
				}
				return fmt.Errorf("%w: timeout waiting for services to become healthy", strerrors.ErrComposeHealthTimeout)
			}
			return fmt.Errorf("%w: %s", strerrors.ErrComposeHealthTimeout, formatUnhealthyServices(lastResult.Services))
		case <-ticker.C:
			result := h.checkHealth(ctx, projectName)
			if result.Error != nil {
				// Log but continue polling - transient errors are expected
				continue
			}
			// Track last good result for error reporting
			lastGoodResult = result
			if result.AllHealthy {
				return nil
			}
		}
	}
}

// checkHealth performs a single health check on all services
func (h *HealthChecker) checkHealth(ctx context.Context, projectName string) HealthCheckResult {
	cmd := NewCommandBuilder().
		WithProject(projectName).
		Ps().
		FormatJSON().
		Build()

	output, err := h.executor.Run(ctx, cmd)
	if err != nil {
		return HealthCheckResult{Error: err}
	}

	services, err := h.parseServicesStatus(output)
	if err != nil {
		return HealthCheckResult{Error: err}
	}

	allHealthy := true
	for _, svc := range services {
		if !isServiceHealthy(svc) {
			allHealthy = false
			break
		}
	}

	return HealthCheckResult{
		AllHealthy: allHealthy,
		Services:   services,
	}
}

// parseServicesStatus parses the JSON output from podman compose ps
func (h *HealthChecker) parseServicesStatus(output []byte) ([]ServiceStatus, error) {
	// Handle empty output
	if len(output) == 0 || strings.TrimSpace(string(output)) == "" {
		return nil, nil
	}

	// podman compose ps --format json returns an array of objects
	var psOutputs []PsOutput
	if err := json.Unmarshal(output, &psOutputs); err != nil {
		// Try parsing as single object (some versions may differ)
		var single PsOutput
		if err2 := json.Unmarshal(output, &single); err2 != nil {
			return nil, fmt.Errorf("failed to parse ps output: %w", err)
		}
		psOutputs = []PsOutput{single}
	}

	services := make([]ServiceStatus, 0, len(psOutputs))
	for _, ps := range psOutputs {
		services = append(services, ServiceStatus{
			Name:     ps.Service,
			State:    ps.State,
			Health:   ps.Health,
			ExitCode: ps.ExitCode,
		})
	}

	return services, nil
}

// isServiceHealthy determines if a service is considered healthy
func isServiceHealthy(svc ServiceStatus) bool {
	state := strings.ToLower(svc.State)

	// Check if container is in a running state
	isRunning := state == "running" || state == "up" || strings.HasPrefix(state, "up ")

	// If not running and has non-zero exit code, it's unhealthy
	if !isRunning && svc.ExitCode != 0 {
		return false
	}

	// Service must be running
	if !isRunning {
		return false
	}

	// If service has health check, it must be healthy
	health := strings.ToLower(svc.Health)
	if health != "" && health != "healthy" && health != "none" && health != "-" {
		// "starting" or "unhealthy" means not ready yet
		return false
	}

	return true
}

// formatUnhealthyServices creates a human-readable list of unhealthy services
func formatUnhealthyServices(services []ServiceStatus) string {
	var unhealthy []string
	for _, svc := range services {
		if !isServiceHealthy(svc) {
			status := fmt.Sprintf("%s (state=%s", svc.Name, svc.State)
			if svc.Health != "" && svc.Health != "-" {
				status += fmt.Sprintf(", health=%s", svc.Health)
			}
			if svc.ExitCode != 0 {
				status += fmt.Sprintf(", exit_code=%d", svc.ExitCode)
			}
			status += ")"
			unhealthy = append(unhealthy, status)
		}
	}
	if len(unhealthy) == 0 {
		return "unknown"
	}
	return strings.Join(unhealthy, ", ")
}
