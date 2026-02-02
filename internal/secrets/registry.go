package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// ErrSecretNotFound is returned when a secret doesn't exist
var ErrSecretNotFound = errors.New("secret not found")

// validNameRegex matches valid secret names (alphanumeric, dashes, underscores)
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// SecretInfo contains metadata about a Podman secret
type SecretInfo struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	CreatedAt string `json:"CreatedAt"` // May be RFC3339 or relative time like "10 days ago"
	UpdatedAt string `json:"UpdatedAt"` // May be RFC3339 or relative time like "10 days ago"
}

// secretInspectResult represents the output of podman secret inspect
type secretInspectResult struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
	UpdatedAt string `json:"UpdatedAt"`
	Spec      struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

// Executor defines the interface for running commands
type Executor interface {
	Run(ctx context.Context, args []string) ([]byte, error)
}

// Registry provides CRUD operations for Podman secrets
type Registry struct {
	executor Executor
}

// NewRegistry creates a new secrets registry with the given executor
func NewRegistry(executor Executor) *Registry {
	return &Registry{
		executor: executor,
	}
}

// Create creates a new Podman secret with the given name and value
func (r *Registry) Create(ctx context.Context, name, value string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if value == "" {
		return errors.New("secret value cannot be empty")
	}

	// Use exec.Command directly to support stdin for passing secret value
	// This avoids exposing the secret in process arguments
	cmd := exec.CommandContext(ctx, "podman", "secret", "create", name, "-")
	cmd.Stdin = strings.NewReader(value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create secret: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// List returns all Podman secrets
func (r *Registry) List(ctx context.Context) ([]SecretInfo, error) {
	// Use Go template format for JSON - Podman outputs one JSON object per line
	output, err := r.executor.Run(ctx, []string{"podman", "secret", "ls", "--format", "{{json .}}"})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Parse newline-delimited JSON
	var secrets []SecretInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var info SecretInfo
		if err := json.Unmarshal([]byte(line), &info); err != nil {
			return nil, fmt.Errorf("failed to parse secret info: %w", err)
		}
		secrets = append(secrets, info)
	}

	return secrets, nil
}

// Inspect returns detailed information about a specific secret
func (r *Registry) Inspect(ctx context.Context, name string) (*SecretInfo, error) {
	// Note: podman secret inspect returns JSON array by default (no --format needed)
	output, err := r.executor.Run(ctx, []string{"podman", "secret", "inspect", name})
	if err != nil {
		if strings.Contains(string(output), "no secret with name or id") {
			return nil, ErrSecretNotFound
		}
		return nil, fmt.Errorf("failed to inspect secret: %w", err)
	}

	var results []secretInspectResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse secret info: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrSecretNotFound
	}

	result := results[0]
	return &SecretInfo{
		ID:        result.ID,
		Name:      result.Spec.Name,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
	}, nil
}

// Delete removes a Podman secret
func (r *Registry) Delete(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("secret name cannot be empty")
	}

	output, err := r.executor.Run(ctx, []string{"podman", "secret", "rm", name})
	if err != nil {
		if strings.Contains(string(output), "no secret with name or id") {
			return ErrSecretNotFound
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

// Exists checks if a secret with the given name exists
func (r *Registry) Exists(ctx context.Context, name string) (bool, error) {
	_, err := r.executor.Run(ctx, []string{"podman", "secret", "exists", name})
	if err != nil {
		// Exit code 1 means secret doesn't exist (not an error)
		return false, nil
	}
	return true, nil
}

// validateName checks if a secret name is valid
func validateName(name string) error {
	if name == "" {
		return errors.New("secret name cannot be empty")
	}
	if !validNameRegex.MatchString(name) {
		return errors.New("secret name contains invalid characters (only alphanumeric, dashes, and underscores allowed)")
	}
	return nil
}
