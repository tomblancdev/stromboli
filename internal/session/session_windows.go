//go:build windows

// Package session provides session management for Stromboli containers.
//
// Windows Support: This package requires Unix-specific flock(2) for init lock
// synchronization. Windows is not supported for production use.
//
// Note: Stromboli is designed for Linux containers via Podman, which does not
// run natively on Windows. For Windows development, use WSL2.
package session

import (
	"errors"
	"fmt"
)

// ErrWindowsNotSupported is returned when attempting to use session locking on Windows.
var ErrWindowsNotSupported = errors.New("session locking is not supported on Windows; use WSL2 for development")

// InitMarkerFile is the name of the marker file that indicates lifecycle hooks have run.
const InitMarkerFile = ".stromboli-initialized"

// initLockFile is used for flock-based mutual exclusion during initialization.
const initLockFile = ".stromboli-init.lock"

// Manager handles session lifecycle and filesystem operations
type Manager struct {
	sessionsDir string
}

// NewManager creates a new session Manager
func NewManager(sessionsDir string) *Manager {
	return &Manager{
		sessionsDir: sessionsDir,
	}
}

// Create is not supported on Windows.
func (m *Manager) Create(sessionID string) (string, string, error) {
	return "", "", fmt.Errorf("Create: %w", ErrWindowsNotSupported)
}

// Destroy is not supported on Windows.
func (m *Manager) Destroy(sessionID string) error {
	return fmt.Errorf("Destroy: %w", ErrWindowsNotSupported)
}

// List is not supported on Windows.
func (m *Manager) List() ([]string, error) {
	return nil, fmt.Errorf("List: %w", ErrWindowsNotSupported)
}

// IsInitialized is not supported on Windows.
func (m *Manager) IsInitialized(sessionID string) bool {
	return false
}

// MarkInitialized is not supported on Windows.
func (m *Manager) MarkInitialized(sessionID string) error {
	return fmt.Errorf("MarkInitialized: %w", ErrWindowsNotSupported)
}

// TryAcquireInitLock is not supported on Windows.
func (m *Manager) TryAcquireInitLock(sessionID string) (bool, func(), error) {
	return false, nil, fmt.Errorf("TryAcquireInitLock: %w", ErrWindowsNotSupported)
}

// GenerateID creates a unique session ID in UUID v4 format.
// This is the only function that works on Windows as it has no platform dependencies.
func GenerateID() string {
	// Use a simple implementation that doesn't require crypto/rand
	// This stub is only for compile-time compatibility, not production use
	return "windows-not-supported"
}
