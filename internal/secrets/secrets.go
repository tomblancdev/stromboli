package secrets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// DefaultCredentialsFile is the default path to Claude credentials
	DefaultCredentialsFile = "~/.claude/.credentials.json"
	// DefaultSecretName is the name of the Podman secret for Claude credentials
	DefaultSecretName = "claude-credentials"
)

// Manager handles Podman secret operations for Claude credentials
type Manager struct {
	credentialsFile string
	secretName      string
}

// NewManager creates a new credentials manager with default settings
func NewManager(secretName string) *Manager {
	if secretName == "" {
		secretName = DefaultSecretName
	}
	return &Manager{
		credentialsFile: expandPath(DefaultCredentialsFile),
		secretName:      secretName,
	}
}

// NewManagerWithPath creates a new credentials manager with custom credentials path
func NewManagerWithPath(credentialsFile string) *Manager {
	if credentialsFile == "" {
		credentialsFile = DefaultCredentialsFile
	}
	return &Manager{
		credentialsFile: expandPath(credentialsFile),
		secretName:      DefaultSecretName,
	}
}

// CredentialsFile returns the path to the credentials file
func (m *Manager) CredentialsFile() string {
	return m.credentialsFile
}

// SecretName returns the name of the Podman secret
func (m *Manager) SecretName() string {
	return m.secretName
}

// FileExists checks if the credentials file exists on the host
func (m *Manager) FileExists() bool {
	_, err := os.Stat(m.credentialsFile)
	return err == nil
}

// SecretExists checks if the Podman secret exists
func (m *Manager) SecretExists(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "podman", "secret", "exists", m.secretName)
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check secret: %w", err)
	}
	return true, nil
}

// CreateSecret creates the Podman secret from the credentials file
func (m *Manager) CreateSecret(ctx context.Context) error {
	if !m.FileExists() {
		return fmt.Errorf("credentials file not found: %s (run 'claude' to authenticate)", m.credentialsFile)
	}

	cmd := exec.CommandContext(ctx, "podman", "secret", "create", m.secretName, m.credentialsFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create secret: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// RemoveSecret removes the Podman secret
func (m *Manager) RemoveSecret(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "podman", "secret", "rm", m.secretName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove secret: %w, output: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// UpdateSecret removes and recreates the secret with current file content
func (m *Manager) UpdateSecret(ctx context.Context) error {
	exists, err := m.SecretExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		if err := m.RemoveSecret(ctx); err != nil {
			return err
		}
	}
	return m.CreateSecret(ctx)
}

// EnsureExists validates credentials file exists and creates/updates the Podman secret
func (m *Manager) EnsureExists(ctx context.Context, filePath string) error {
	// Use provided filePath if given
	if filePath != "" {
		m.credentialsFile = expandPath(filePath)
	}

	// Check if credentials file exists
	if !m.FileExists() {
		return fmt.Errorf("credentials file not found: %s (run 'claude' to authenticate)", m.credentialsFile)
	}

	// Check if secret exists
	exists, err := m.SecretExists(ctx)
	if err != nil {
		return err
	}

	if !exists {
		// Create new secret
		return m.CreateSecret(ctx)
	}

	// Secret exists - update it to ensure it has latest content
	return m.UpdateSecret(ctx)
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
