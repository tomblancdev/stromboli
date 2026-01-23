package runner

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellExecutor_Run_SimpleCommand tests basic command execution
func TestShellExecutor_Run_SimpleCommand(t *testing.T) {
	executor := NewShellExecutor()
	output, err := executor.Run(context.Background(), []string{"echo", "hello"})

	require.NoError(t, err)
	assert.Contains(t, string(output), "hello")
}

// TestShellExecutor_Run_EmptyArgs tests handling of empty args
func TestShellExecutor_Run_EmptyArgs(t *testing.T) {
	executor := NewShellExecutor()
	output, err := executor.Run(context.Background(), []string{})

	require.NoError(t, err)
	assert.Empty(t, output)
}

// TestShellExecutor_Run_ContextCancellation tests context cancellation
func TestShellExecutor_Run_ContextCancellation(t *testing.T) {
	executor := NewShellExecutor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := executor.Run(ctx, []string{"sleep", "10"})
	assert.Error(t, err)
}

// TestShellExecutor_RunStream_SimpleCommand tests streaming execution
func TestShellExecutor_RunStream_SimpleCommand(t *testing.T) {
	executor := NewShellExecutor()
	stdout, stderr, start, wait, err := executor.RunStream(context.Background(), []string{"echo", "test"})

	require.NoError(t, err)
	require.NotNil(t, stdout)
	require.NotNil(t, stderr)
	require.NotNil(t, start)
	require.NotNil(t, wait)

	// Start the command
	err = start()
	require.NoError(t, err)

	// Read output
	output, err := io.ReadAll(stdout)
	require.NoError(t, err)
	assert.Contains(t, string(output), "test")

	// Wait for completion
	err = wait()
	require.NoError(t, err)
}

// TestShellExecutor_RunStream_EmptyArgs tests streaming with empty args
func TestShellExecutor_RunStream_EmptyArgs(t *testing.T) {
	executor := NewShellExecutor()
	stdout, stderr, start, wait, err := executor.RunStream(context.Background(), []string{})

	require.NoError(t, err)
	assert.Nil(t, stdout)
	assert.Nil(t, stderr)
	assert.Nil(t, start)
	assert.Nil(t, wait)
}

// TestMockExecutor_Run_DefaultBehavior tests mock default behavior
func TestMockExecutor_Run_DefaultBehavior(t *testing.T) {
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("mocked output")

	output, err := mock.Run(context.Background(), []string{"test", "command"})

	require.NoError(t, err)
	assert.Equal(t, "mocked output", string(output))
	assert.Len(t, mock.GetCalls(), 1)
	assert.Equal(t, []string{"test", "command"}, mock.GetCalls()[0])
}

// TestMockExecutor_Run_CustomFunction tests mock custom function
func TestMockExecutor_Run_CustomFunction(t *testing.T) {
	mock := NewMockExecutor()
	mock.RunFunc = func(ctx context.Context, args []string) ([]byte, error) {
		return []byte("custom: " + strings.Join(args, " ")), nil
	}

	output, err := mock.Run(context.Background(), []string{"foo", "bar"})

	require.NoError(t, err)
	assert.Equal(t, "custom: foo bar", string(output))
}

// TestMockExecutor_Run_Error tests mock error behavior
func TestMockExecutor_Run_Error(t *testing.T) {
	mock := NewMockExecutor()
	mock.DefaultError = errors.New("mock error")

	_, err := mock.Run(context.Background(), []string{"fail"})

	require.Error(t, err)
	assert.Equal(t, "mock error", err.Error())
}

// TestMockExecutor_RunStream_DefaultBehavior tests mock streaming
func TestMockExecutor_RunStream_DefaultBehavior(t *testing.T) {
	mock := NewMockExecutor()
	mock.StreamOutput = "stdout content"
	mock.StreamError = "stderr content"

	stdout, stderr, start, wait, err := mock.RunStream(context.Background(), []string{"test"})

	require.NoError(t, err)
	require.NotNil(t, stdout)
	require.NotNil(t, stderr)

	// Start command
	err = start()
	require.NoError(t, err)

	// Read stdout
	stdoutData, err := io.ReadAll(stdout)
	require.NoError(t, err)
	assert.Equal(t, "stdout content", string(stdoutData))

	// Read stderr
	stderrData, err := io.ReadAll(stderr)
	require.NoError(t, err)
	assert.Equal(t, "stderr content", string(stderrData))

	// Wait for completion
	err = wait()
	require.NoError(t, err)

	// Verify call was recorded
	assert.Len(t, mock.GetCalls(), 1)
}

// TestMockExecutor_RunStream_StartError tests mock stream start error
func TestMockExecutor_RunStream_StartError(t *testing.T) {
	mock := NewMockExecutor()
	mock.StreamStartError = errors.New("start failed")

	_, _, start, _, err := mock.RunStream(context.Background(), []string{"test"})

	require.NoError(t, err)

	err = start()
	require.Error(t, err)
	assert.Equal(t, "start failed", err.Error())
}

// TestMockExecutor_RunStream_WaitError tests mock stream wait error
func TestMockExecutor_RunStream_WaitError(t *testing.T) {
	mock := NewMockExecutor()
	mock.StreamWaitError = errors.New("wait failed")

	_, _, start, wait, err := mock.RunStream(context.Background(), []string{"test"})

	require.NoError(t, err)

	err = start()
	require.NoError(t, err)

	err = wait()
	require.Error(t, err)
	assert.Equal(t, "wait failed", err.Error())
}

// TestMockExecutor_Reset tests resetting mock calls
func TestMockExecutor_Reset(t *testing.T) {
	mock := NewMockExecutor()

	_, _ = mock.Run(context.Background(), []string{"cmd1"})
	_, _ = mock.Run(context.Background(), []string{"cmd2"})

	assert.Len(t, mock.GetCalls(), 2)

	mock.Reset()

	assert.Len(t, mock.GetCalls(), 0)
}

// TestMockExecutor_ConcurrentCalls tests thread safety
func TestMockExecutor_ConcurrentCalls(t *testing.T) {
	mock := NewMockExecutor()
	mock.DefaultOutput = []byte("ok")

	// Run multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.Run(context.Background(), []string{"test"})
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Len(t, mock.GetCalls(), 10)
}
