// Package errors defines domain error types for Stromboli.
//
// # Error Types
//
// Session errors:
//   - ErrSessionIDRequired: Session ID is required but not provided
//   - ErrInvalidSessionID: Session ID format is invalid
//   - ErrSessionNotFound: Session does not exist
//
// Workspace errors:
//   - ErrWorkspaceNotFound: Workspace directory does not exist
//   - ErrWorkspaceNotAllowed: Workspace is not in the allowed list
//
// # Usage
//
//	if errors.Is(err, strerrors.ErrSessionNotFound) {
//	    // Handle missing session
//	}
//
// These errors are designed to be used with errors.Is() for type checking
// and with fmt.Errorf("%w") for wrapping with additional context.
package errors
