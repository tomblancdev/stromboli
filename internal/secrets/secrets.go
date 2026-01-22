package secrets

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// DefaultSecretName is the default name for the Claude token secret
	DefaultSecretName = "claude-token"
)

// Manager handles Podman secret operations
type Manager struct {
	secretName string
}

// NewManager creates a new secrets manager
func NewManager(secretName string) *Manager {
	if secretName == "" {
		secretName = DefaultSecretName
	}
	return &Manager{secretName: secretName}
}

// SecretName returns the name of the managed secret
func (m *Manager) SecretName() string {
	return m.secretName
}

// Exists checks if the secret exists in Podman
func (m *Manager) Exists(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "podman", "secret", "exists", m.secretName)
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means secret doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check secret: %w", err)
	}
	return true, nil
}

// Create creates the secret from a file
func (m *Manager) Create(ctx context.Context, filePath string) error {
	cmd := exec.CommandContext(ctx, "podman", "secret", "create", m.secretName, filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create secret: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// EnsureExists creates the secret if it doesn't exist
func (m *Manager) EnsureExists(ctx context.Context, filePath string) error {
	exists, err := m.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return m.Create(ctx, filePath)
}

// Remove removes the secret
func (m *Manager) Remove(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "podman", "secret", "rm", m.secretName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove secret: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Update removes and recreates the secret with new content
func (m *Manager) Update(ctx context.Context, filePath string) error {
	exists, err := m.Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		if err := m.Remove(ctx); err != nil {
			return err
		}
	}
	return m.Create(ctx, filePath)
}
