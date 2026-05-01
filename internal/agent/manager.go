package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultIdleTimeout is applied when CreateRequest.IdleTimeoutSeconds is zero.
// Picked to match the cost-control posture elsewhere in the codebase: long
// enough for "ambient" use cases that wake up every few minutes, short enough
// that a forgotten agent doesn't burn budget overnight.
const DefaultIdleTimeout = 30 * time.Minute

// Errors returned by the Manager. Callers (the API handlers) translate these
// to HTTP status codes — keeping them as concrete sentinels avoids string
// matching in the handler layer.
var (
	ErrAgentNotFound  = errors.New("agent: not found")
	ErrAgentBusy      = errors.New("agent: a turn is already in progress")
	ErrAgentNotReady  = errors.New("agent: not ready to accept input")
	ErrAgentExited    = errors.New("agent: process has exited")
	ErrIdleTimedOut   = errors.New("agent: idle timeout reached")
	ErrInvalidRequest = errors.New("agent: invalid request")
)

// Manager owns the live agent registry and the spawn/stop lifecycle.
type Manager struct {
	spawner Spawner

	mu     sync.RWMutex
	agents map[string]*Agent
}

// NewManager returns an empty Manager backed by the given Spawner. Pass
// NewProcessSpawner() in production; tests can supply a fake to avoid
// actually starting podman.
func NewManager(spawner Spawner) *Manager {
	return &Manager{
		spawner: spawner,
		agents:  map[string]*Agent{},
	}
}

// ArgvBuilder turns a CreateRequest into the argv that Spawner.Spawn will
// exec. Pulled into a function variable so the test suite can substitute an
// "echo" command without depending on podman.
type ArgvBuilder func(req CreateRequest, agentID, sessionID string) ([]string, error)

// Create starts a new agent and returns its initial snapshot. The agent runs
// in the background; callers reach it again via Get / Send / Subscribe / Stop.
//
// build is called synchronously to translate the CreateRequest into argv. We
// accept it as a parameter (rather than a Manager field) so the API layer
// can inject configuration that lives there (image names, credentials path,
// resource limits) without coupling the agent package to viper config.
func (m *Manager) Create(ctx context.Context, req CreateRequest, build ArgvBuilder) (Snapshot, error) {
	if req.IdleTimeoutSeconds < 0 {
		return Snapshot{}, fmt.Errorf("%w: idle_timeout_seconds must be non-negative", ErrInvalidRequest)
	}
	idleTimeout := DefaultIdleTimeout
	if req.IdleTimeoutSeconds > 0 {
		idleTimeout = time.Duration(req.IdleTimeoutSeconds) * time.Second
	}

	agentID := "agent-" + uuid.NewString()
	sessionID := uuid.NewString()

	argv, err := build(req, agentID, sessionID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	now := time.Now()
	a := &Agent{
		ID:             agentID,
		SessionID:      sessionID,
		status:         StatusStarting,
		createdAt:      now,
		lastActivityAt: now,
		idleTimeout:    idleTimeout,
		subscribers:    map[int]chan Event{},
		done:           make(chan struct{}),
	}

	// Buffered: stdout reader runs ahead of the dispatcher; the buffer absorbs
	// micro-bursts. Larger than necessary on purpose — Claude can emit many
	// lines per second during tool calls.
	lineSink := make(chan string, 256)

	proc, err := m.spawner.Spawn(ctx, SpawnRequest{
		AgentID:   agentID,
		SessionID: sessionID,
		Workdir:   req.Workdir,
		Argv:      argv,
	}, lineSink, func(err error) {
		a.markExited(err)
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("agent: spawn: %w", err)
	}
	a.process = proc

	m.mu.Lock()
	m.agents[agentID] = a
	m.mu.Unlock()

	// Background dispatcher: read from lineSink, fan out to subscribers,
	// detect turn boundaries, advance status.
	go a.dispatch(lineSink)
	// Background idle watchdog: stop the agent when no activity for the
	// configured timeout.
	go a.watchIdle(m)

	// Optional initial prompt — the same path as POST /send, just inlined
	// during creation so callers don't need a second request for the common
	// "create + immediately ask something" flow.
	if strings.TrimSpace(req.Prompt) != "" {
		if _, err := m.Send(agentID, req.Prompt); err != nil {
			// Don't fail Create on a Send error — the agent is alive, the
			// caller can retry via /send. Surface the issue via the snapshot
			// status (it'll be StatusIdle once the agent is ready).
			_ = err
		}
	}

	return a.snapshot(), nil
}

// Get returns a snapshot of the agent, or ErrAgentNotFound.
func (m *Manager) Get(id string) (Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[id]
	if !ok {
		return Snapshot{}, ErrAgentNotFound
	}
	return a.snapshot(), nil
}

// List returns snapshots of every registered agent. Order is not guaranteed.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Snapshot, 0, len(m.agents))
	for _, a := range m.agents {
		out = append(out, a.snapshot())
	}
	return out
}

// SendResult is what /send returns to the caller.
type SendResult struct {
	TurnID string `json:"turn_id"`
}

// Send writes a user prompt as a stream-json line on the agent's stdin.
// Returns ErrAgentBusy if a turn is currently in flight.
func (m *Manager) Send(id, prompt string) (SendResult, error) {
	m.mu.RLock()
	a, ok := m.agents[id]
	m.mu.RUnlock()
	if !ok {
		return SendResult{}, ErrAgentNotFound
	}
	if strings.TrimSpace(prompt) == "" {
		return SendResult{}, fmt.Errorf("%w: prompt is required", ErrInvalidRequest)
	}

	a.mu.Lock()
	switch a.status {
	case StatusGenerating:
		a.mu.Unlock()
		return SendResult{}, ErrAgentBusy
	case StatusExited:
		a.mu.Unlock()
		return SendResult{}, ErrAgentExited
	case StatusStarting, StatusIdle:
		// fine, fall through
	default:
		a.mu.Unlock()
		return SendResult{}, ErrAgentNotReady
	}
	turnID := "turn-" + uuid.NewString()
	a.currentTurnID = turnID
	a.status = StatusGenerating
	a.lastActivityAt = time.Now()
	a.mu.Unlock()

	a.fanout(Event{Type: "turn_start", TurnID: turnID, At: time.Now()})

	// Stream-json input: one JSON object per line, role=user, content=prompt.
	// Schema matches what the Claude CLI expects on stdin under
	// `--input-format stream-json`.
	payload, err := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	})
	if err != nil {
		return SendResult{}, fmt.Errorf("agent: marshal prompt: %w", err)
	}
	if err := a.process.Send(string(payload)); err != nil {
		// Roll back generating state — the turn never actually started.
		a.mu.Lock()
		a.status = StatusIdle
		a.currentTurnID = ""
		a.mu.Unlock()
		a.fanout(Event{Type: "error", TurnID: turnID, Error: err.Error(), At: time.Now()})
		return SendResult{}, err
	}

	return SendResult{TurnID: turnID}, nil
}

// Subscribe registers an SSE subscriber. The returned channel receives Events
// until the caller invokes the cancel function (typically on HTTP request
// disconnect) or the agent exits.
//
// Channel is buffered (32). If the subscriber falls behind, events are
// dropped FOR THAT SUBSCRIBER ONLY — one slow client must not block the
// whole agent.
func (m *Manager) Subscribe(id string) (<-chan Event, func(), error) {
	m.mu.RLock()
	a, ok := m.agents[id]
	m.mu.RUnlock()
	if !ok {
		return nil, nil, ErrAgentNotFound
	}

	ch := make(chan Event, 32)
	a.subsMu.Lock()
	subID := a.nextSubID
	a.nextSubID++
	a.subscribers[subID] = ch
	a.subsMu.Unlock()

	cancel := func() {
		a.subsMu.Lock()
		if existing, ok := a.subscribers[subID]; ok {
			delete(a.subscribers, subID)
			close(existing)
		}
		a.subsMu.Unlock()
	}
	return ch, cancel, nil
}

// Stop terminates the agent's process gracefully and removes it from the
// registry. Idempotent — calling Stop on an already-stopped agent returns nil.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	a, ok := m.agents[id]
	if ok {
		delete(m.agents, id)
	}
	m.mu.Unlock()
	if !ok {
		return ErrAgentNotFound
	}
	// 5s for graceful, then SIGTERM, then SIGKILL — handled inside Stop.
	_ = a.process.Stop(5 * time.Second)
	a.markExited(nil)
	return nil
}

// StopAll tears every agent down. Used for graceful server shutdown so we
// don't leave orphan podman containers behind.
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

// dispatch is the per-agent fan-out loop. It runs in a goroutine spawned by
// Create and exits when the process closes its stdout (sink is closed).
func (a *Agent) dispatch(sink <-chan string) {
	for line := range sink {
		// Recognize stderr lines (prefix injected by forwardLines) and emit
		// them as error events without affecting turn state.
		if strings.HasPrefix(line, "[stderr] ") {
			a.fanout(Event{
				Type:    "error",
				TurnID:  a.readCurrentTurn(),
				Error:   strings.TrimPrefix(line, "[stderr] "),
				At:      time.Now(),
				Content: "",
			})
			continue
		}

		// First non-stderr line ⇒ Claude is ready for prompts.
		a.markIdleIfStarting()

		// Forward the raw line as an output event. We deliberately do not
		// re-parse the JSON here — subscribers that want structured fields
		// can json.Unmarshal Content themselves.
		a.fanout(Event{
			Type:    "output",
			TurnID:  a.readCurrentTurn(),
			Content: line,
			At:      time.Now(),
		})

		// Heuristic: a stream-json "result" message marks turn end.
		// We parse just enough to detect the "type":"result" envelope; if
		// parsing fails (line wasn't JSON), we keep going.
		if isTurnEnd(line) {
			a.completeTurn()
		}
	}
}

func (a *Agent) watchIdle(m *Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.done:
			// Agent already exited (clean stop, crash, idle timeout from a
			// previous tick). Bail immediately instead of holding a goroutine
			// alive for up to 30s waiting for the next tick.
			return
		case <-ticker.C:
			a.mu.Lock()
			if a.status == StatusExited {
				a.mu.Unlock()
				return
			}
			idle := time.Since(a.lastActivityAt)
			timeout := a.idleTimeout
			a.mu.Unlock()
			if idle >= timeout {
				a.fanout(Event{Type: "exited", Error: ErrIdleTimedOut.Error(), At: time.Now()})
				_ = m.Stop(a.ID)
				return
			}
		}
	}
}

func (a *Agent) fanout(ev Event) {
	a.subsMu.Lock()
	defer a.subsMu.Unlock()
	for _, ch := range a.subscribers {
		select {
		case ch <- ev:
		default:
			// Slow subscriber — drop one to make room and re-try once.
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
				// Still full somehow — give up rather than block. Subscriber
				// will see a gap; that's the contract.
			}
		}
	}
}

func (a *Agent) markIdleIfStarting() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status == StatusStarting {
		a.status = StatusIdle
		a.lastActivityAt = time.Now()
	}
}

func (a *Agent) readCurrentTurn() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentTurnID
}

func (a *Agent) completeTurn() {
	a.mu.Lock()
	turnID := a.currentTurnID
	a.currentTurnID = ""
	if a.status == StatusGenerating {
		a.status = StatusIdle
	}
	a.turnsCompleted++
	a.lastActivityAt = time.Now()
	a.mu.Unlock()
	a.fanout(Event{Type: "turn_end", TurnID: turnID, At: time.Now()})
}

func (a *Agent) markExited(err error) {
	a.mu.Lock()
	if a.status == StatusExited {
		a.mu.Unlock()
		return
	}
	a.status = StatusExited
	a.exitErr = err
	a.mu.Unlock()

	// Signal background goroutines (watchIdle) that they should exit. Done
	// before the fan-out so the next tick after this call sees the channel
	// already closed.
	a.doneOnce.Do(func() { close(a.done) })

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	a.fanout(Event{Type: "exited", Error: errStr, At: time.Now()})

	// Close every subscriber so their HTTP handlers can finish.
	a.subsMu.Lock()
	for id, ch := range a.subscribers {
		delete(a.subscribers, id)
		close(ch)
	}
	a.subsMu.Unlock()
}

// isTurnEnd looks at one stream-json line and returns true when it's the
// "result" envelope Claude emits when a turn is complete. Conservative — if
// anything looks off, we leave the agent in StatusGenerating until the next
// turn-end actually arrives. The watchIdle goroutine catches the worst case.
func isTurnEnd(line string) bool {
	var probe struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
	}
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		return false
	}
	return probe.Type == "result"
}
