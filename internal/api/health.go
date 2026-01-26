package api

import (
	"context"
	"fmt"
	"os"
	"time"

	"stromboli/internal/runner"
	"stromboli/internal/secrets"
	"stromboli/internal/version"
)

// HealthConfig holds health check configuration
type HealthConfig struct {
	// Timeout is the maximum time to wait for each health check
	Timeout time.Duration
	// CredentialsFile is the path to the Claude credentials file to check
	CredentialsFile string
	// SecretName is the name of the Podman secret for credentials
	SecretName string
}

// DefaultHealthConfig returns the default health check configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Timeout:         5 * time.Second,
		CredentialsFile: "~/.claude/.credentials.json",
		SecretName:      secrets.DefaultSecretName,
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
	// Version of the service
	Version string `json:"version" example:"0.1.4"`
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
		h.checkCredentials(),
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
		Version:    version.Version,
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

// checkCredentials verifies the Claude credentials file exists
func (h *HealthChecker) checkCredentials() ComponentHealth {
	_, err := os.Stat(h.config.CredentialsFile)
	if err != nil {
		return ComponentHealth{
			Name:   "claude-credentials-file",
			Status: "error",
			Error:  fmt.Sprintf("credentials file not found at %s: run 'claude' to authenticate", h.config.CredentialsFile),
		}
	}

	return ComponentHealth{
		Name:   "claude-credentials-file",
		Status: "ok",
	}
}

// checkSecret verifies the Podman secret exists
func (h *HealthChecker) checkSecret(ctx context.Context) ComponentHealth {
	ctx, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	_, err := h.executor.Run(ctx, []string{"podman", "secret", "exists", h.config.SecretName})
	if err != nil {
		return ComponentHealth{
			Name:   "claude-credentials-secret",
			Status: "error",
			Error:  fmt.Sprintf("podman secret '%s' not found: run stromboli to create it", h.config.SecretName),
		}
	}

	return ComponentHealth{
		Name:   "claude-credentials-secret",
		Status: "ok",
	}
}
