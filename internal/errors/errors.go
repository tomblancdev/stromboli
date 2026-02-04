package errors

import (
	"errors"
	"fmt"
)

// Domain errors
var (
	ErrTokenNotFound        = errors.New("token not found")
	ErrContainerNotFound    = errors.New("container not found")
	ErrCommandFailed        = errors.New("podman command failed")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionIDRequired    = errors.New("session ID is required")
	ErrInvalidSessionID     = errors.New("invalid session ID")
	ErrWorkspaceNotAllowed  = errors.New("workspace path not allowed")
	ErrInitInProgress       = errors.New("session initialization in progress")
)

// SessionNotFound returns a session not found error with the session ID
func SessionNotFound(id string) error {
	return fmt.Errorf("%w: %s", ErrSessionNotFound, id)
}

// InitInProgress returns an error indicating session initialization is in progress.
// Callers can use errors.Is(err, ErrInitInProgress) to detect this condition
// and implement retry logic.
func InitInProgress(sessionID string) error {
	return fmt.Errorf("%w: session %s; please retry", ErrInitInProgress, sessionID)
}

// TokenError wraps a token retrieval error
func TokenError(err error) error {
	return fmt.Errorf("failed to get token: %w", err)
}

// ContainerError wraps a container operation error
func ContainerError(action string, err error) error {
	return fmt.Errorf("failed to %s container: %w", action, err)
}
