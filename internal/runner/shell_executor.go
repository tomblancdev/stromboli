package runner

import (
	"context"
	"io"
	"os/exec"
)

// ShellExecutor executes commands using os/exec
type ShellExecutor struct{}

// NewShellExecutor creates a new ShellExecutor
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

// Run executes a command and returns the combined output
func (e *ShellExecutor) Run(ctx context.Context, args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	return cmd.CombinedOutput()
}

// RunStream executes a command and returns pipes for streaming output
func (e *ShellExecutor) RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error) {
	if len(args) == 0 {
		return nil, nil, nil, nil, nil
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	startFn := func() error {
		return cmd.Start()
	}

	waitFn := func() error {
		return cmd.Wait()
	}

	return stdoutPipe, stderrPipe, startFn, waitFn, nil
}
