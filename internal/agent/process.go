package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Spawner builds and starts the underlying process for an agent. Pulled out
// behind an interface so tests can substitute a fake (echo / fixture) process.
type Spawner interface {
	// Spawn starts the process and returns a handle. The returned process
	// must already have stdout streaming to lineSink (one Event per line).
	// onExit is invoked exactly once when the process terminates, with the
	// process's wait error (or nil on a clean exit).
	Spawn(ctx context.Context, req SpawnRequest, lineSink chan<- string, onExit func(error)) (agentProcess, error)
}

// SpawnRequest is the resolved configuration the Spawner needs.
type SpawnRequest struct {
	AgentID   string
	SessionID string
	Workdir   string
	// Args passed verbatim to the chosen container/claude command. The
	// runtime caller (manager.go) is responsible for assembling this into a
	// valid podman invocation; the spawner just exec's it.
	Argv []string
}

// processSpawner is the production Spawner. It exec's the supplied argv and
// keeps stdin/stdout pipes open for the lifetime of the process.
type processSpawner struct{}

// NewProcessSpawner returns a Spawner that spins up a real subprocess.
func NewProcessSpawner() Spawner { return &processSpawner{} }

// realProcess is the production agentProcess implementation.
type realProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex // serializes writes to stdin
	exited  chan struct{}
	exitErr error
}

func (s *processSpawner) Spawn(ctx context.Context, req SpawnRequest, lineSink chan<- string, onExit func(error)) (agentProcess, error) {
	if len(req.Argv) == 0 {
		return nil, errors.New("agent: empty argv")
	}

	cmd := exec.CommandContext(ctx, req.Argv[0], req.Argv[1:]...) // #nosec G204 -- argv built by trusted caller
	// Put the child in its own process group so we can SIGTERM the whole
	// tree (podman + claude) on Stop without orphaning the inner process.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("agent: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("agent: stdout pipe: %w", err)
	}

	// Stderr is captured separately so a Claude crash surfaces a useful
	// message instead of an opaque exit code. We tee it into the same line
	// sink prefixed with "[stderr] ", which the agent layer recognizes when
	// building error events.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("agent: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent: start: %w", err)
	}

	p := &realProcess{
		cmd:    cmd,
		stdin:  stdin,
		exited: make(chan struct{}),
	}

	// Pipe readers run concurrently; close the sink only once BOTH have
	// drained their respective pipes so the dispatcher's range loop sees
	// EOF cleanly.
	var pipesDone sync.WaitGroup
	pipesDone.Add(2)
	go func() {
		defer pipesDone.Done()
		forwardLines(stdout, lineSink, "")
	}()
	go func() {
		defer pipesDone.Done()
		forwardLines(stderr, lineSink, "[stderr] ")
	}()

	// Wait goroutine — fires onExit exactly once when the process is done.
	go func() {
		err := cmd.Wait()
		// Make sure both pipe readers have flushed before signalling exit;
		// otherwise the manager.markExited() event can race with a final
		// stdout line that hasn't reached the dispatcher yet.
		pipesDone.Wait()
		close(lineSink)
		p.exitErr = err
		close(p.exited)
		if onExit != nil {
			onExit(err)
		}
	}()

	return p, nil
}

func forwardLines(r io.Reader, sink chan<- string, prefix string) {
	scanner := bufio.NewScanner(r)
	// Stream-json output can have very long lines (full tool results inline).
	scanner.Buffer(make([]byte, 0, 1<<16), 10<<20) // up to 10 MiB / line.
	for scanner.Scan() {
		// Block-send: the buffered sink (256 entries) absorbs bursts. If the
		// dispatcher is so far behind that the buffer fills, applying back-
		// pressure to Claude's stdout writer is the right move — it's far
		// better than dropping events we'd otherwise have to silently lose.
		sink <- prefix + scanner.Text()
	}
	// Scanner errors after process exit (closed pipe) are expected — ignore.
}

func (p *realProcess) Send(line string) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	select {
	case <-p.exited:
		return errors.New("agent: process has exited")
	default:
	}
	if _, err := io.WriteString(p.stdin, line); err != nil {
		return fmt.Errorf("agent: write stdin: %w", err)
	}
	if line == "" || line[len(line)-1] != '\n' {
		if _, err := io.WriteString(p.stdin, "\n"); err != nil {
			return fmt.Errorf("agent: write newline: %w", err)
		}
	}
	return nil
}

func (p *realProcess) Stop(gracePeriod time.Duration) error {
	// Closing stdin is the canonical "you're done" signal for stream-json
	// Claude. Most well-behaved processes will exit on their own from there.
	_ = p.stdin.Close()

	select {
	case <-p.exited:
		return p.exitErr
	case <-time.After(gracePeriod):
	}

	// Still alive — SIGTERM the whole process group.
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	}

	select {
	case <-p.exited:
		return p.exitErr
	case <-time.After(gracePeriod):
	}

	// Last resort: SIGKILL.
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	}
	<-p.exited
	return p.exitErr
}

func (p *realProcess) Wait() error {
	<-p.exited
	return p.exitErr
}
