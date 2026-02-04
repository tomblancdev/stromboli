package session

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	strerrors "stromboli/internal/errors"
)

// InitMarkerFile is the name of the marker file that indicates lifecycle hooks have run.
// This file is checked on subsequent runs to skip re-initialization.
// The leading dot hides it from casual directory listings.
const InitMarkerFile = ".stromboli-initialized"

// initLockFile is used for flock-based mutual exclusion during initialization.
// The lock file persists after initialization but is harmless - it will be
// cleaned up when the session is destroyed.
const initLockFile = ".stromboli-init.lock"

// validateSessionID validates a session ID for security
// Returns an error if the session ID is empty, contains path traversal attempts,
// or contains null bytes
func validateSessionID(sessionID string) error {
	if sessionID == "" {
		return strerrors.ErrSessionIDRequired
	}
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") || strings.ContainsAny(sessionID, "\x00") {
		return strerrors.ErrInvalidSessionID
	}
	return nil
}

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

// Create creates a session directory and returns the sessionID and absolute path.
// If sessionID is empty, generates a new UUID.
// Also creates the .claude subdirectory for Claude Code config/state.
func (m *Manager) Create(sessionID string) (string, string, error) {
	if sessionID == "" {
		sessionID = GenerateID()
	}

	// Validate session ID to prevent path traversal and null byte injection
	if err := validateSessionID(sessionID); err != nil {
		return "", "", err
	}

	sessionPath := filepath.Join(m.sessionsDir, sessionID)
	if err := os.MkdirAll(sessionPath, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create .claude subdirectory for Claude Code config/state
	// This is needed because when mounting secrets to ~/.claude/.credentials.json,
	// the intermediate directory must exist with correct permissions
	claudeDir := filepath.Join(sessionPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create .claude directory: %w", err)
	}

	absSessionPath, err := filepath.Abs(sessionPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute session path: %w", err)
	}

	return sessionID, absSessionPath, nil
}

// Destroy removes a session and all its data.
//
// WARNING: If another process holds the init lock for this session,
// the directory will still be removed but the lock holder will not be
// notified. The lock file descriptor in the other process will become
// invalid, and any subsequent operations will fail.
//
// Callers should ensure no operations are in progress before destroying
// a session. In practice, this means:
//   - Waiting for any running containers to complete
//   - Not destroying sessions that may be used by other processes
func (m *Manager) Destroy(sessionID string) error {
	// Validate session ID to prevent path traversal and null byte injection
	if err := validateSessionID(sessionID); err != nil {
		return err
	}

	sessionPath := filepath.Join(m.sessionsDir, sessionID)

	// Check if session exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return strerrors.SessionNotFound(sessionID)
	}

	// Remove session directory
	if err := os.RemoveAll(sessionPath); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}

	return nil
}

// List returns all existing session IDs
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() {
			sessions = append(sessions, entry.Name())
		}
	}

	return sessions, nil
}

// IsInitialized checks if lifecycle init hooks have already run for a session
func (m *Manager) IsInitialized(sessionID string) bool {
	if err := validateSessionID(sessionID); err != nil {
		return false
	}
	markerPath := filepath.Join(m.sessionsDir, sessionID, InitMarkerFile)
	_, err := os.Stat(markerPath)
	return err == nil
}

// MarkInitialized marks a session as having run its lifecycle init hooks.
//
// IMPORTANT: Callers should hold the init lock (via TryAcquireInitLock) to prevent
// race conditions when multiple processes attempt initialization simultaneously.
//
// WARNING: The marker file must not be deleted while the session is in use.
// Deleting the marker file while the session exists may cause init hooks
// to run multiple times, violating the idempotency contract. The double-check
// locking pattern used by TryAcquireInitLock assumes marker files are immutable
// once created.
func (m *Manager) MarkInitialized(sessionID string) error {
	// Validate session ID to prevent path traversal and null byte injection
	if err := validateSessionID(sessionID); err != nil {
		return err
	}

	sessionPath := filepath.Join(m.sessionsDir, sessionID)

	// Check if session exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return strerrors.SessionNotFound(sessionID)
	}

	markerPath := filepath.Join(sessionPath, InitMarkerFile)

	// Write atomically using temp file + rename
	tmpFile, err := os.CreateTemp(sessionPath, ".stromboli-init-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()

	// Write content
	_, writeErr := tmpFile.WriteString(time.Now().Format(time.RFC3339))
	if writeErr != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write marker: %w", writeErr)
	}

	// Sync to ensure data is on disk before rename (durability on some filesystems)
	if syncErr := tmpFile.Sync(); syncErr != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to sync marker: %w", syncErr)
	}

	// Close the file
	if closeErr := tmpFile.Close(); closeErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close marker: %w", closeErr)
	}

	// Set secure permissions (0600 - owner read/write only)
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, markerPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename marker: %w", err)
	}

	return nil
}

// TryAcquireInitLock attempts to acquire an exclusive lock for initialization.
// Returns (acquired, cleanup, error). If acquired is true, caller MUST call cleanup.
// If another process holds the lock, returns (false, nil, nil).
func (m *Manager) TryAcquireInitLock(sessionID string) (bool, func(), error) {
	// Validate session ID
	if err := validateSessionID(sessionID); err != nil {
		return false, nil, err
	}

	sessionPath := filepath.Join(m.sessionsDir, sessionID)

	// Verify session directory exists before creating lock file
	// This prevents race conditions if session is destroyed between Create() and this call
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return false, nil, strerrors.SessionNotFound(sessionID)
	}

	lockPath := filepath.Join(sessionPath, initLockFile)

	// Open or create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFile.Close()
		// EWOULDBLOCK means another process has the lock
		if err == syscall.EWOULDBLOCK {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Create cleanup function that logs errors (non-fatal)
	cleanup := func() {
		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
			slog.Debug("Failed to release init lock", "session_id", sessionID, "error", err)
		}
		if err := lockFile.Close(); err != nil {
			slog.Debug("Failed to close lock file", "session_id", sessionID, "error", err)
		}
	}

	return true, cleanup, nil
}

// GenerateID creates a unique session ID in UUID v4 format
// Claude CLI expects UUIDs for --resume flag
func GenerateID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes) // Error ignored: crypto/rand.Read always succeeds on supported platforms
	// Set UUID version 4 bits
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	// Set UUID variant bits
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
