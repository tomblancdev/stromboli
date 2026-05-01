package agent

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardLines_TruncatesOversizedLine confirms that a single line larger
// than maxAgentLineBytes does not deadlock the reader: forwardLines emits a
// truncation marker for the over-budget line and keeps reading the next one.
//
// Without this guard, a giant tool-result line caused bufio.Scanner to
// permanently fail, the OS pipe filled up, Claude's stdout write blocked, and
// cmd.Wait() hung forever.
func TestForwardLines_TruncatesOversizedLine(t *testing.T) {
	// Two lines: the first is comfortably over the cap; the second is small
	// and proves the reader keeps going after the overflow.
	pr, pw := io.Pipe()
	sink := make(chan string, 4)

	// Close the sink once forwardLines returns so the test's range loop
	// terminates. In production, Spawn's wait goroutine does this after
	// pipesDone.Wait — here we mimic that contract directly.
	done := make(chan struct{})
	go func() {
		forwardLines(pr, sink, "")
		close(sink)
		close(done)
	}()

	go func() {
		// 12 MiB of 'x', then \n, then a normal line, then EOF.
		// Stream the giant blob in 1 MiB chunks so the reader and writer
		// share progress instead of buffering 12 MiB on the heap at once.
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < 12; i++ {
			_, err := io.WriteString(pw, chunk)
			require.NoError(t, err)
		}
		_, err := io.WriteString(pw, "\nfollow-up\n")
		require.NoError(t, err)
		_ = pw.Close()
	}()

	got := make([]string, 0, 2)
	for line := range sink {
		got = append(got, line)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("forwardLines did not return — overflow handling is deadlocking")
	}

	require.Len(t, got, 2, "expected one marker line + one follow-up line, got %v", got)
	assert.Contains(t, got[0], "[stromboli: line truncated")
	assert.Equal(t, "follow-up", got[1])
}

// TestForwardLines_ForwardsNormalLines is a sanity check that the new
// implementation didn't regress the happy path.
func TestForwardLines_ForwardsNormalLines(t *testing.T) {
	pr, pw := io.Pipe()
	sink := make(chan string, 8)

	done := make(chan struct{})
	go func() {
		forwardLines(pr, sink, "[p] ")
		close(sink)
		close(done)
	}()

	go func() {
		_, _ = io.WriteString(pw, "alpha\nbeta\n\ngamma\n")
		_ = pw.Close()
	}()

	var got []string
	for line := range sink {
		got = append(got, line)
	}
	<-done

	// We deliberately preserve the empty line between beta and gamma —
	// stream-json sometimes uses blank separators and dropping them silently
	// would break downstream parsers.
	assert.Equal(t, []string{"[p] alpha", "[p] beta", "[p] ", "[p] gamma"}, got)
}

// TestForwardLines_HandlesNoTrailingNewline checks the corner case where a
// process exits without flushing a final newline. The buffered partial line
// should still be emitted (don't silently drop a meaningful payload).
func TestForwardLines_HandlesNoTrailingNewline(t *testing.T) {
	pr, pw := io.Pipe()
	sink := make(chan string, 2)

	done := make(chan struct{})
	go func() {
		forwardLines(pr, sink, "")
		close(sink)
		close(done)
	}()

	go func() {
		_, _ = io.WriteString(pw, "first\nlast-without-newline")
		_ = pw.Close()
	}()

	var got []string
	for line := range sink {
		got = append(got, line)
	}
	<-done

	assert.Equal(t, []string{"first", "last-without-newline"}, got)
}
