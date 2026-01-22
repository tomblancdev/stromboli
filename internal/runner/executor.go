package runner

import (
	"context"
	"io"
)

// Executor executes shell commands
// This interface abstracts command execution to enable testing without Podman
type Executor interface {
	// Run executes a command and returns the combined output
	Run(ctx context.Context, args []string) ([]byte, error)

	// RunStream executes a command and returns pipes for streaming output
	// Returns stdout and stderr readers, and a function to start and wait for the command
	RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
}
