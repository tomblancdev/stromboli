// Package secrets provides a thin wrapper around Podman's secret management commands.
// It enables secure storage and retrieval of sensitive data like API keys and tokens
// that can be injected into agent containers at runtime.
//
// Secrets are stored using Podman's native secret store, which encrypts them at rest.
// This package never exposes secret values through its API - only metadata is accessible.
package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ErrSecretNotFound is returned when a secret doesn't exist
var ErrSecretNotFound = errors.New("secret not found")

// ErrSecretAlreadyExists is returned when trying to create a secret that already exists
var ErrSecretAlreadyExists = errors.New("secret already exists")

// ErrValidation is returned when input validation fails (invalid name, empty value, etc.)
var ErrValidation = errors.New("validation error")

// validNameRegex matches valid secret names (alphanumeric, dashes, underscores)
var validNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
	// MaxSecretNameLength is the maximum allowed length for a secret name
	MaxSecretNameLength = 253
	// MaxSecretValueSize is the maximum allowed size for a secret value (1MB)
	MaxSecretValueSize = 1 * 1024 * 1024
	// DefaultOperationTimeout is the default timeout for Podman operations
	DefaultOperationTimeout = 30 * time.Second

	// Command constants
	cmdPodman = "podman"
	cmdSecret = "secret"
	cmdCreate = "create"
	cmdList   = "ls"
	cmdInspect = "inspect"
	cmdDelete = "rm"
	cmdExists = "exists"

	// Format strings
	jsonFormat = "{{json .}}"

	// Error detection patterns
	errPatternNotFound      = "no secret with name or id"
	errPatternAlreadyExists = "already exists"
)

// withTimeout wraps a context with DefaultOperationTimeout if no deadline is set
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		// Context already has a deadline, use it as-is
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, DefaultOperationTimeout)
}

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

// Executor defines the interface for running shell commands.
// This abstraction enables testing without requiring actual Podman installation.
type Executor interface {
	// Run executes a command with the given arguments and returns its output.
	Run(ctx context.Context, args []string) ([]byte, error)
}

// Registry provides CRUD operations for Podman secrets.
// It wraps the podman secret command-line interface and provides a safe,
// validated API for managing secrets.
type Registry struct {
	executor Executor
}

// NewRegistry creates a new secrets registry with the given executor.
// The executor is used for running Podman commands (except Create, which
// uses exec.Command directly to support stdin for secure value passing).
func NewRegistry(executor Executor) *Registry {
	return &Registry{
		executor: executor,
	}
}

// Create creates a new Podman secret with the given name and value.
// The secret value is passed via stdin to avoid exposing it in process arguments.
//
// Returns ErrSecretAlreadyExists if a secret with the same name already exists.
// Returns validation errors if the name contains invalid characters or exceeds length limits.
func (r *Registry) Create(ctx context.Context, name, value string) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("secrets: create %q: %w", name, err)
	}
	if value == "" {
		return fmt.Errorf("%w: secret value cannot be empty", ErrValidation)
	}
	if len(value) > MaxSecretValueSize {
		return fmt.Errorf("%w: secret value exceeds maximum size of %d bytes", ErrValidation, MaxSecretValueSize)
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Use exec.Command directly to support stdin for passing secret value
	// This avoids exposing the secret in process arguments
	cmd := exec.CommandContext(ctx, cmdPodman, cmdSecret, cmdCreate, name, "-")
	cmd.Stdin = strings.NewReader(value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), errPatternAlreadyExists) {
			return ErrSecretAlreadyExists
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

// List returns metadata for all Podman secrets.
// Returns an empty slice (never nil) if no secrets exist.
// The returned SecretInfo contains only metadata - never the secret value.
func (r *Registry) List(ctx context.Context) ([]SecretInfo, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Use Go template format for JSON - Podman outputs one JSON object per line
	output, err := r.executor.Run(ctx, []string{cmdPodman, cmdSecret, cmdList, "--format", jsonFormat})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	// Parse newline-delimited JSON - initialize as empty slice to return [] not null
	secrets := make([]SecretInfo, 0)
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

// Inspect returns detailed metadata about a specific secret.
// Returns ErrSecretNotFound if no secret with the given name exists.
// The returned SecretInfo contains only metadata - never the secret value.
func (r *Registry) Inspect(ctx context.Context, name string) (*SecretInfo, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Note: podman secret inspect returns JSON array by default (no --format needed)
	output, err := r.executor.Run(ctx, []string{cmdPodman, cmdSecret, cmdInspect, name})
	if err != nil {
		if strings.Contains(string(output), errPatternNotFound) {
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

// Delete removes a Podman secret by name.
// Returns ErrSecretNotFound if no secret with the given name exists.
// The operation is idempotent - calling Delete on a non-existent secret returns an error,
// but calling it multiple times on the same secret will succeed once and then return ErrSecretNotFound.
func (r *Registry) Delete(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	output, err := r.executor.Run(ctx, []string{cmdPodman, cmdSecret, cmdDelete, name})
	if err != nil {
		if strings.Contains(string(output), errPatternNotFound) {
			return ErrSecretNotFound
		}
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	return nil
}

// Exists checks if a secret with the given name exists.
// Returns (true, nil) if the secret exists, (false, nil) if it doesn't.
// Only returns an error if the name validation fails.
func (r *Registry) Exists(ctx context.Context, name string) (bool, error) {
	if err := validateName(name); err != nil {
		return false, err
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := r.executor.Run(ctx, []string{cmdPodman, cmdSecret, cmdExists, name})
	if err != nil {
		// Exit code 1 means secret doesn't exist (not an error)
		return false, nil
	}
	return true, nil
}

// validateName validates a secret name to prevent command injection
// and ensure compatibility with Podman's naming requirements.
// Returns an error wrapping ErrValidation if validation fails.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: secret name cannot be empty", ErrValidation)
	}
	if len(name) > MaxSecretNameLength {
		return fmt.Errorf("%w: secret name exceeds maximum length of %d characters", ErrValidation, MaxSecretNameLength)
	}
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("%w: secret name contains invalid characters (only alphanumeric, dashes, and underscores allowed)", ErrValidation)
	}
	return nil
}
