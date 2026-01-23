package session

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	strerrors "stromboli/internal/errors"
)

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
func (m *Manager) Create(sessionID string) (string, string, error) {
	if sessionID == "" {
		sessionID = GenerateID()
	}

	// Validate session ID to prevent path traversal
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") {
		return "", "", strerrors.ErrInvalidSessionID
	}

	sessionPath := filepath.Join(m.sessionsDir, sessionID)
	if err := os.MkdirAll(sessionPath, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create session directory: %w", err)
	}

	absSessionPath, err := filepath.Abs(sessionPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to get absolute session path: %w", err)
	}

	return sessionID, absSessionPath, nil
}

// Destroy removes a session and all its data
func (m *Manager) Destroy(sessionID string) error {
	if sessionID == "" {
		return strerrors.ErrSessionIDRequired
	}

	// Validate session ID to prevent path traversal
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") {
		return strerrors.ErrInvalidSessionID
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

// GenerateID creates a unique session ID in UUID v4 format
// Claude CLI expects UUIDs for --resume flag
func GenerateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	// Set UUID version 4 bits
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	// Set UUID variant bits
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
