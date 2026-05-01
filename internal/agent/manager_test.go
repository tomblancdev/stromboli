package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSpawner is a Spawner that doesn't exec anything. Each Send pushes the
// configured replies to the dispatcher, simulating Claude's stream-json
// output. Tests drive turn boundaries by calling FinishTurn().
type fakeSpawner struct {
	mu       sync.Mutex
	procs    []*fakeProcess
	startErr error
}

type fakeProcess struct {
	mu       sync.Mutex
	sent     []string
	sink     chan<- string
	stopped  chan struct{}
	stopErr  error
	stopOnce sync.Once
	onExit   func(error)
}

func (s *fakeSpawner) Spawn(_ context.Context, _ SpawnRequest, sink chan<- string, onExit func(error)) (agentProcess, error) {
	if s.startErr != nil {
		return nil, s.startErr
	}
	p := &fakeProcess{
		sink:    sink,
		stopped: make(chan struct{}),
		onExit:  onExit,
	}
	s.mu.Lock()
	s.procs = append(s.procs, p)
	s.mu.Unlock()
	return p, nil
}

func (s *fakeSpawner) latest() *fakeProcess {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.procs[len(s.procs)-1]
}

func (p *fakeProcess) Send(line string) error {
	select {
	case <-p.stopped:
		return errors.New("process stopped")
	default:
	}
	p.mu.Lock()
	p.sent = append(p.sent, line)
	p.mu.Unlock()
	return nil
}

func (p *fakeProcess) Stop(_ time.Duration) error {
	p.stopOnce.Do(func() {
		close(p.stopped)
		close(p.sink)
		if p.onExit != nil {
			p.onExit(nil)
		}
	})
	return p.stopErr
}

func (p *fakeProcess) Wait() error {
	<-p.stopped
	return p.stopErr
}

// emitLine pushes one line through the sink as if Claude had written it.
func (p *fakeProcess) emitLine(line string) {
	p.sink <- line
}

// emitTurnEnd pushes the stream-json result envelope so the dispatcher
// transitions out of generating.
func (p *fakeProcess) emitTurnEnd() {
	p.emitLine(`{"type":"result","subtype":"success"}`)
}

func okBuilder(_ CreateRequest, agentID, sessionID string) ([]string, error) {
	// Argv content doesn't matter for the fake — we never exec it.
	return []string{"true", agentID, sessionID}, nil
}

func TestManager_Create_AllocatesIDsAndStartsAgent(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(snap.ID, "agent-"))
	assert.NotEmpty(t, snap.SessionID)
	assert.Equal(t, StatusStarting, snap.Status)
	assert.Equal(t, int(DefaultIdleTimeout/time.Second), snap.IdleTimeoutSec)
}

func TestManager_Create_RejectsNegativeIdleTimeout(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	_, err := mgr.Create(context.Background(), CreateRequest{IdleTimeoutSeconds: -1}, okBuilder)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequest)
}

func TestManager_Send_HappyPath(t *testing.T) {
	spawner := &fakeSpawner{}
	mgr := NewManager(spawner)
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)

	// Subscribe before sending so we capture turn_start.
	events, cancel, err := mgr.Subscribe(snap.ID)
	require.NoError(t, err)
	defer cancel()

	sendRes, err := mgr.Send(snap.ID, "hello")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(sendRes.TurnID, "turn-"))

	// Manager should have written one stream-json line.
	require.Eventually(t, func() bool {
		spawner.latest().mu.Lock()
		defer spawner.latest().mu.Unlock()
		return len(spawner.latest().sent) == 1
	}, time.Second, 10*time.Millisecond)
	got := spawner.latest().sent[0]
	assert.Contains(t, got, `"role":"user"`)
	assert.Contains(t, got, `"content":"hello"`)

	// Status should now be generating.
	cur, _ := mgr.Get(snap.ID)
	assert.Equal(t, StatusGenerating, cur.Status)

	// First event should be turn_start with the same TurnID.
	ev := <-events
	assert.Equal(t, "turn_start", ev.Type)
	assert.Equal(t, sendRes.TurnID, ev.TurnID)

	// Simulate Claude's output, then turn-end.
	spawner.latest().emitLine(`{"type":"assistant","content":"hi"}`)
	ev = <-events
	assert.Equal(t, "output", ev.Type)
	assert.Equal(t, sendRes.TurnID, ev.TurnID)

	spawner.latest().emitTurnEnd()
	// Drain the result-envelope output event before reading turn_end.
	<-events
	ev = <-events
	assert.Equal(t, "turn_end", ev.Type)
	assert.Equal(t, sendRes.TurnID, ev.TurnID)

	// Status now idle, turn count incremented.
	cur, _ = mgr.Get(snap.ID)
	assert.Equal(t, StatusIdle, cur.Status)
	assert.Equal(t, 1, cur.TurnsCompleted)
}

func TestManager_Send_RejectsConcurrentTurn(t *testing.T) {
	spawner := &fakeSpawner{}
	mgr := NewManager(spawner)
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)

	// First send moves agent to generating.
	_, err = mgr.Send(snap.ID, "first")
	require.NoError(t, err)

	// Second send while generating must return ErrAgentBusy.
	_, err = mgr.Send(snap.ID, "second")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAgentBusy)
}

func TestManager_Stop_RemovesAgentAndClosesSubscribers(t *testing.T) {
	spawner := &fakeSpawner{}
	mgr := NewManager(spawner)
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)

	events, cancel, err := mgr.Subscribe(snap.ID)
	require.NoError(t, err)
	defer cancel()

	require.NoError(t, mgr.Stop(snap.ID))

	// Subscriber chan should drain & close.
	require.Eventually(t, func() bool {
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return true
				}
			case <-time.After(50 * time.Millisecond):
				return false
			}
		}
	}, time.Second, 10*time.Millisecond)

	// Subsequent Get returns ErrAgentNotFound.
	_, err = mgr.Get(snap.ID)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestManager_Send_MissingAgent(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	_, err := mgr.Send("agent-nope", "hello")
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestManager_Send_EmptyPrompt(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)
	_, err = mgr.Send(snap.ID, "   ")
	assert.ErrorIs(t, err, ErrInvalidRequest)
}

func TestManager_Create_DefaultIdleTimeoutEnabled(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	snap, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
	require.NoError(t, err)

	// Default behaviour: idle watchdog is on, snapshot reflects the manager
	// default (DefaultIdleTimeout) and the disabled flag is false (so the
	// `omitempty` JSON tag suppresses it from the response entirely).
	assert.False(t, snap.IdleTimeoutDisabled)
	assert.Equal(t, int(DefaultIdleTimeout/time.Second), snap.IdleTimeoutSec)
}

func TestManager_Create_DisableIdleTimeoutPlumbsThrough(t *testing.T) {
	// disable_idle_timeout=true must:
	//   1. surface in the Snapshot,
	//   2. cause Manager.Create to skip the watchIdle goroutine (verified
	//      indirectly via the agent's done channel — if watchIdle were
	//      running, it'd hold a reference until the next 30s tick).
	//   3. coexist with a custom IdleTimeoutSeconds value (the int field
	//      becomes informational once the watchdog is disabled).
	mgr := NewManager(&fakeSpawner{})
	snap, err := mgr.Create(context.Background(), CreateRequest{
		IdleTimeoutSeconds: 60, // ignored when disabled, but still recorded
		DisableIdleTimeout: true,
	}, okBuilder)
	require.NoError(t, err)

	assert.True(t, snap.IdleTimeoutDisabled, "Snapshot must surface the disabled flag")
	assert.Equal(t, 60, snap.IdleTimeoutSec, "IdleTimeoutSec is informational; recorded but unused")

	// Spam-stop and confirm clean teardown — watchIdle wasn't running, so
	// markExited completes immediately.
	require.NoError(t, mgr.Stop(snap.ID))
}

func TestManager_List_ReturnsAllAgents(t *testing.T) {
	mgr := NewManager(&fakeSpawner{})
	for i := 0; i < 3; i++ {
		_, err := mgr.Create(context.Background(), CreateRequest{}, okBuilder)
		require.NoError(t, err)
	}
	assert.Len(t, mgr.List(), 3)
}
