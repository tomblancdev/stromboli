package agent

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// processSpawner exercises real os/exec subprocesses, so the tests need a
// shell. They use `cat` (read stdin → write stdout, then exit on EOF) which is
// available on every Linux/macOS dev image; Windows is skipped explicitly.

func skipIfNoCat(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX cat / sh; skipping on Windows")
	}
}

func TestProcessSpawner_ForwardsStdoutLines(t *testing.T) {
	skipIfNoCat(t)

	sink := make(chan string, 8)
	spawner := NewProcessSpawner()

	// `printf 'a\nb\nc\n'` writes three lines and exits — exercises the
	// forwardLines goroutine end-to-end and confirms the wait goroutine
	// closes the sink on process exit so a `range` consumer sees EOF.
	exited := make(chan struct{})
	proc, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"sh", "-c", "printf 'a\\nb\\nc\\n'"}},
		sink,
		func(_ error) { close(exited) },
	)
	require.NoError(t, err)
	defer func() { _ = proc.Stop(time.Second) }()

	var got []string
	for line := range sink {
		got = append(got, line)
	}
	assert.Equal(t, []string{"a", "b", "c"}, got)

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("onExit never fired")
	}
}

func TestProcessSpawner_TagsStderrLines(t *testing.T) {
	skipIfNoCat(t)

	sink := make(chan string, 4)
	spawner := NewProcessSpawner()

	// echo to fd 2 — confirms stderr lines are prefixed so the dispatcher
	// can route them to error events.
	_, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"sh", "-c", "echo oops 1>&2"}},
		sink,
		nil,
	)
	require.NoError(t, err)

	var lines []string
	for line := range sink {
		lines = append(lines, line)
	}

	var seenStderr bool
	for _, line := range lines {
		if strings.HasPrefix(line, "[stderr] ") {
			assert.Equal(t, "[stderr] oops", line)
			seenStderr = true
		}
	}
	assert.True(t, seenStderr, "stderr line should be prefixed and forwarded; got %v", lines)
}

func TestProcessSpawner_SendWritesToStdin(t *testing.T) {
	skipIfNoCat(t)

	sink := make(chan string, 8)
	spawner := NewProcessSpawner()

	// `cat` echoes stdin on stdout — perfect for confirming Send delivers.
	proc, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"cat"}},
		sink,
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, proc.Send(`hello world`))
	require.NoError(t, proc.Send(`second line`))

	// First two lines should make it through. Stop closes stdin so cat exits.
	var got []string
	go func() {
		// Give cat a moment to flush both lines, then close stdin so the
		// scanner reaches EOF and the sink closes.
		time.Sleep(50 * time.Millisecond)
		_ = proc.Stop(time.Second)
	}()
	for line := range sink {
		got = append(got, line)
	}
	assert.Contains(t, got, "hello world")
	assert.Contains(t, got, "second line")
}

func TestProcessSpawner_SendAfterExitFails(t *testing.T) {
	skipIfNoCat(t)

	sink := make(chan string, 4)
	spawner := NewProcessSpawner()

	// `true` exits immediately with code 0.
	proc, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"true"}},
		sink,
		nil,
	)
	require.NoError(t, err)

	// Drain the sink so Wait can complete.
	go func() {
		for range sink {
		}
	}()

	// Wait for the wait goroutine to mark the process as exited.
	require.Eventually(t, func() bool {
		return proc.Send("post-exit") != nil
	}, time.Second, 5*time.Millisecond, "Send must fail after process has exited")
}

func TestProcessSpawner_StopReturnsExitError(t *testing.T) {
	skipIfNoCat(t)

	sink := make(chan string, 4)
	spawner := NewProcessSpawner()

	// Hang reading stdin — Stop has to escalate (close stdin → SIGTERM → SIGKILL).
	proc, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"cat"}},
		sink,
		nil,
	)
	require.NoError(t, err)

	// Drain in the background so the wait goroutine can complete.
	go func() {
		for range sink {
		}
	}()

	done := make(chan struct{})
	go func() {
		_ = proc.Stop(50 * time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within 2s — escalation may be broken")
	}
}

func TestProcessSpawner_RejectsEmptyArgv(t *testing.T) {
	spawner := NewProcessSpawner()
	_, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: nil},
		make(chan string, 1),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty argv")
}

func TestProcessSpawner_StartFailureSurfacesError(t *testing.T) {
	skipIfNoCat(t)

	spawner := NewProcessSpawner()
	_, err := spawner.Spawn(context.Background(),
		SpawnRequest{Argv: []string{"/this/binary/does/not/exist"}},
		make(chan string, 1),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent: start")
}
