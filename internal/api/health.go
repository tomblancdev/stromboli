package api

import (
	"context"
	"fmt"
	"time"

	"stromboli/internal/runner"
)

// HealthConfig holds health check configuration
type HealthConfig struct {
	// Timeout is the maximum time to wait for each health check
	Timeout time.Duration
	// SecretName is the name of the Podman secret to check for
	SecretName string
}

// DefaultHealthConfig returns the default health check configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Timeout:    5 * time.Second,
		SecretName: "claude-token",
	}
}

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	// Name of the component
	Name string `json:"name" example:"podman"`
	// Status is "ok" or "error"
	Status string `json:"status" example:"ok"`
	// Error message if status is "error"
	Error string `json:"error,omitempty" example:""`
}

// DetailedHealth represents the overall health with component breakdown
type DetailedHealth struct {
	// Status is "ok" if all components healthy, "degraded" if any component unhealthy
	Status string `json:"status" example:"ok"`
	// Name of the service
	Name string `json:"name" example:"stromboli"`
	// Components contains individual component health statuses
	Components []ComponentHealth `json:"components"`
}

// HealthChecker performs health checks on system components
type HealthChecker struct {
	executor runner.Executor
	config   HealthConfig
}

// NewHealthChecker creates a new HealthChecker with the given executor and config
func NewHealthChecker(executor runner.Executor, config HealthConfig) *HealthChecker {
	return &HealthChecker{
		executor: executor,
		config:   config,
	}
}

// Check performs all health checks and returns the detailed health status
func (h *HealthChecker) Check(ctx context.Context) DetailedHealth {
	components := []ComponentHealth{
		h.checkPodman(ctx),
		h.checkSecret(ctx),
	}

	status := "ok"
	for _, c := range components {
		if c.Status != "ok" {
			status = "degraded"
			break
		}
	}

	return DetailedHealth{
		Status:     status,
		Name:       "stromboli",
		Components: components,
	}
}

// checkPodman verifies Podman connectivity by running "podman version"
func (h *HealthChecker) checkPodman(ctx context.Context) ComponentHealth {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	_, err := h.executor.Run(ctx, []string{"podman", "version"})
	if err != nil {
		return ComponentHealth{
			Name:   "podman",
			Status: "error",
			Error:  fmt.Sprintf("failed to connect to podman: %v", err),
		}
	}

	return ComponentHealth{
		Name:   "podman",
		Status: "ok",
	}
}

// checkSecret verifies the claude-token secret exists via "podman secret exists"
func (h *HealthChecker) checkSecret(ctx context.Context) ComponentHealth {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	_, err := h.executor.Run(ctx, []string{"podman", "secret", "exists", h.config.SecretName})
	if err != nil {
		return ComponentHealth{
			Name:   "claude-secret",
			Status: "error",
			Error:  fmt.Sprintf("secret '%s' not found: %v", h.config.SecretName, err),
		}
	}

	return ComponentHealth{
		Name:   "claude-secret",
		Status: "ok",
	}
}
