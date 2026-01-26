package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	cachedHash      string     // SHA256 hash of last synced credentials file
	mu              sync.Mutex // Protects cachedHash
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
// It also initializes the cached hash for subsequent SyncIfChanged calls
func (m *Manager) EnsureExists(ctx context.Context, filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

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
		if err := m.CreateSecret(ctx); err != nil {
			return err
		}
	} else {
		// Secret exists - update it to ensure it has latest content
		if err := m.UpdateSecret(ctx); err != nil {
			return err
		}
	}

	// Initialize cached hash after successful sync
	hash, err := m.GetFileHash()
	if err != nil {
		return fmt.Errorf("failed to initialize credentials hash: %w", err)
	}
	m.cachedHash = hash

	return nil
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

// GetFileHash computes the SHA256 hash of the credentials file
func (m *Manager) GetFileHash() (string, error) {
	f, err := os.Open(m.credentialsFile)
	if err != nil {
		return "", fmt.Errorf("failed to open credentials file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash credentials file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// SyncIfChanged updates the Podman secret if the credentials file has changed
// Returns true if the secret was updated, false if no change was detected
func (m *Manager) SyncIfChanged(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current file hash
	currentHash, err := m.GetFileHash()
	if err != nil {
		return false, err
	}

	// If hash matches cached value, no sync needed
	if currentHash == m.cachedHash {
		return false, nil
	}

	// Hash changed - update the secret
	if err := m.UpdateSecret(ctx); err != nil {
		return false, fmt.Errorf("failed to sync secret: %w", err)
	}

	// Update cached hash after successful sync
	m.cachedHash = currentHash
	return true, nil
}
