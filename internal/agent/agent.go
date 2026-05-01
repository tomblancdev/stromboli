// Package agent implements long-lived "ambient" Claude sessions: a Claude
// process running with stream-json I/O, kept alive across many prompts so
// callers (sensor buses, on-call bots, anything event-driven) get sub-second
// turn latency instead of the 2-3s cold start of /run.
//
// One Agent = one container = one Claude process. The runtime registers each
// agent with a Manager, ferries POSTed prompts to the process's stdin, fans
// stdout events out to any subscribed SSE listeners, and tears the agent down
// when DELETE /agents/{id} fires or the idle timeout elapses.
package agent

import (
	"sync"
	"time"

	"stromboli/internal/types"
)

// Status captures the current lifecycle state of an Agent.
type Status string

const (
	// StatusStarting — container is being created and Claude hasn't yet
	// reported readiness (no stdout output observed).
	StatusStarting Status = "starting"
	// StatusIdle — Claude is alive and ready to accept a new turn.
	StatusIdle Status = "idle"
	// StatusGenerating — a turn is in flight; further /send calls return 409.
	StatusGenerating Status = "generating"
	// StatusExited — the Claude process or container has stopped (clean or
	// crashed). The agent record persists briefly so callers can read /status.
	StatusExited Status = "exited"
)

// CreateRequest is the operator-supplied configuration for a new Agent.
//
// @Description Configuration for spawning a long-lived Claude agent
type CreateRequest struct {
	// Initial prompt sent as the first turn. Optional — callers can also
	// create an empty agent and use /send for the first interaction.
	Prompt string `json:"prompt,omitempty" example:"You are an on-call assistant."`
	// Working directory inside the container.
	Workdir string `json:"workdir,omitempty" example:"/workspace"`
	// Idle-timeout override in seconds. Zero means "use the manager default
	// (DefaultIdleTimeout)". Negative values are rejected at parse time.
	IdleTimeoutSeconds int `json:"idle_timeout_seconds,omitempty" example:"1800"`
	// Claude CLI options forwarded to the command builder. Same shape as
	// RunRequest.claude so callers don't have to learn two schemas — the
	// field is fully typed in OpenAPI rather than a freeform object.
	Claude *types.ClaudeOptions `json:"claude,omitempty"`
}

// Event is one entry in the SSE stream the runtime pushes to subscribers.
// Stromboli wraps every Claude stdout line in an Event so subscribers always
// know which turn an output belongs to without re-parsing themselves.
type Event struct {
	// Type — turn_start, output, turn_end, error, exited.
	Type string `json:"type"`
	// TurnID — present on output / turn_start / turn_end.
	TurnID string `json:"turn_id,omitempty"`
	// Content — the raw line from Claude (already stripped of trailing \n).
	// For turn_start / turn_end / exited this is empty.
	Content string `json:"content,omitempty"`
	// Error — populated for type="error".
	Error string `json:"error,omitempty"`
	// Timestamp the runtime saw the event (server time).
	At time.Time `json:"at"`
}

// Agent is the in-memory record for one running session. Internals are guarded
// by `mu`; never read these fields without holding it.
type Agent struct {
	ID        string
	SessionID string

	// Lifecycle bookkeeping.
	mu             sync.Mutex
	status         Status
	createdAt      time.Time
	lastActivityAt time.Time
	idleTimeout    time.Duration
	turnsCompleted int
	currentTurnID  string
	exitErr        error

	// done is closed exactly once when the agent transitions to StatusExited.
	// Background goroutines (watchIdle) select on it to exit promptly instead
	// of waiting for their next tick.
	done     chan struct{}
	doneOnce sync.Once

	// Process plumbing.
	process agentProcess

	// SSE fan-out. Each subscriber gets a buffered channel; if the channel is
	// full (slow consumer), events are dropped for *that* subscriber so one
	// bad client can't stall the others.
	subsMu      sync.Mutex
	subscribers map[int]chan Event
	nextSubID   int
}

// agentProcess holds the os/exec handle and pipes for the running container.
// Lives in process.go; declared here only so Agent can reference it.
type agentProcess interface {
	// Send writes a single stream-json line to the process's stdin. Returns
	// an error if the process has exited or the write fails.
	Send(line string) error
	// Stop sends SIGTERM, waits up to gracePeriod for clean exit, then SIGKILL.
	Stop(gracePeriod time.Duration) error
	// Wait blocks until the process exits and returns its exit error (nil for
	// a clean exit).
	Wait() error
}

// Snapshot is the read-only view of an Agent that the API returns.
type Snapshot struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
	IdleTimeoutSec int       `json:"idle_timeout_seconds"`
	TurnsCompleted int       `json:"turns_completed"`
	ExitError      string    `json:"exit_error,omitempty"`
}

// snapshot returns a consistent read of the agent's metadata.
// Caller must NOT hold a.mu — this method takes the lock itself.
func (a *Agent) snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	exitErr := ""
	if a.exitErr != nil {
		exitErr = a.exitErr.Error()
	}
	return Snapshot{
		ID:             a.ID,
		SessionID:      a.SessionID,
		Status:         a.status,
		CreatedAt:      a.createdAt,
		LastActivityAt: a.lastActivityAt,
		IdleTimeoutSec: int(a.idleTimeout / time.Second),
		TurnsCompleted: a.turnsCompleted,
		ExitError:      exitErr,
	}
}
