package errors

import (
	"errors"
	"fmt"
)

// Domain errors
var (
	ErrTokenNotFound       = errors.New("token not found")
	ErrContainerNotFound   = errors.New("container not found")
	ErrCommandFailed       = errors.New("podman command failed")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionIDRequired   = errors.New("session ID is required")
	ErrInvalidSessionID    = errors.New("invalid session ID")
	ErrWorkspaceNotAllowed = errors.New("workspace path not allowed")
	ErrInitInProgress      = errors.New("session initialization in progress")

	// Compose errors
	ErrComposeFileNotFound    = errors.New("compose file not found")
	ErrComposeValidation      = errors.New("compose file validation failed")
	ErrComposeServiceNotFound = errors.New("service not found in compose file")
	ErrComposeHealthTimeout   = errors.New("compose services did not become healthy")
	ErrComposeNotConfigured   = errors.New("compose support not configured")
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

// ComposeFileNotFound returns a compose file not found error
func ComposeFileNotFound(path string) error {
	return fmt.Errorf("%w: %s", ErrComposeFileNotFound, path)
}

// ComposeServiceNotFound returns a service not found error
func ComposeServiceNotFound(service string) error {
	return fmt.Errorf("%w: %s", ErrComposeServiceNotFound, service)
}

// ComposeHealthTimeout returns a health timeout error with service details
func ComposeHealthTimeout(details string) error {
	return fmt.Errorf("%w: %s", ErrComposeHealthTimeout, details)
}
