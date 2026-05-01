package auth

import (
	"sync"
	"time"
)

// Blacklist tracks JWT IDs (JTIs) that have been explicitly invalidated
// before their natural expiry — typically because the user logged out.
//
// Two implementations ship with Stromboli:
//
//   - MemoryBlacklist (default): an in-memory map. Fastest, no external deps,
//     but every blacklisted JTI is forgotten when the server restarts. Fine
//     for short access-token TTLs (e.g. 1 h) and single-instance deployments
//     where the operational impact of "logout takes effect after restart"
//     is acceptable.
//
//   - BoltBlacklist (opt-in via STROMBOLI_AUTH_BLACKLIST_BACKEND=bolt): a
//     bbolt-backed file. Survives restarts, single-process. Use when you
//     need durable logout on a single Stromboli instance.
//
// Future implementations (Redis, Postgres) plug in via this interface
// without touching the API or auth-middleware layers.
//
// All methods must be safe for concurrent use. IsBlacklisted is on the hot
// path (every authenticated request) — implementations should aim for sub-
// millisecond reads.
type Blacklist interface {
	// Add registers a JTI as blacklisted until expiresAt. Calling Add for an
	// already-blacklisted JTI overwrites the prior expiry — this is what we
	// want for token rotation: a fresh issue extends the deny window.
	Add(jti string, expiresAt time.Time) error

	// IsBlacklisted reports whether jti is currently denied. Implementations
	// MAY return stale results immediately after a concurrent Add (eventually
	// consistent under heavy contention) — middleware callers fail closed on
	// errors, so a brief read-after-write gap is acceptable.
	IsBlacklisted(jti string) (bool, error)

	// Size returns the current number of tracked entries. Used by health
	// checks and tests; not on the auth hot path.
	Size() (int, error)

	// Close flushes any pending writes and stops background goroutines.
	// Idempotent.
	Close() error
}

// MemoryBlacklist is the default in-memory Blacklist implementation. Backed
// by a map + RWMutex with a periodic cleanup goroutine that reaps expired
// entries.
//
// Trade-off: zero dependencies and microsecond-scale reads, but every entry
// is lost on restart. Operators that need logout to survive restart should
// configure the bolt backend instead.
type MemoryBlacklist struct {
	tokens    map[string]time.Time // jti -> expiresAt
	mu        sync.RWMutex
	stopChan  chan struct{}
	cleanupMu sync.Mutex
}

// NewMemoryBlacklist returns a fresh in-memory blacklist.
func NewMemoryBlacklist() *MemoryBlacklist {
	return &MemoryBlacklist{
		tokens: make(map[string]time.Time),
	}
}

// Add registers a token as blacklisted until expiresAt.
func (b *MemoryBlacklist) Add(jti string, expiresAt time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens[jti] = expiresAt
	return nil
}

// IsBlacklisted reports whether jti is currently in the deny set.
func (b *MemoryBlacklist) IsBlacklisted(jti string) (bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, exists := b.tokens[jti]
	return exists, nil
}

// Size returns the current number of blacklisted JTIs.
func (b *MemoryBlacklist) Size() (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.tokens), nil
}

// Close stops the cleanup goroutine.
func (b *MemoryBlacklist) Close() error {
	b.StopCleanup()
	return nil
}

// StartCleanup starts a background goroutine that removes expired tokens
// at the specified interval. Idempotent — calling it multiple times will
// not create multiple goroutines.
func (b *MemoryBlacklist) StartCleanup(interval time.Duration) {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()

	if b.stopChan != nil {
		return
	}

	stop := make(chan struct{})
	b.stopChan = stop
	go b.cleanupLoop(interval, stop)
}

// StopCleanup stops the cleanup goroutine. Idempotent.
func (b *MemoryBlacklist) StopCleanup() {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()

	if b.stopChan != nil {
		close(b.stopChan)
		b.stopChan = nil
	}
}

func (b *MemoryBlacklist) cleanupLoop(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.cleanup()
		case <-stop:
			return
		}
	}
}

func (b *MemoryBlacklist) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for jti, expiresAt := range b.tokens {
		if now.After(expiresAt) {
			delete(b.tokens, jti)
		}
	}
}

// TokenBlacklist is retained as an alias for MemoryBlacklist so existing
// callers in other branches don't break during the cutover. New code should
// program against the Blacklist interface instead.
//
// Deprecated: use Blacklist + NewMemoryBlacklist directly.
type TokenBlacklist = MemoryBlacklist

// NewTokenBlacklist is retained as an alias for NewMemoryBlacklist.
//
// Deprecated: use NewMemoryBlacklist.
func NewTokenBlacklist() *MemoryBlacklist {
	return NewMemoryBlacklist()
}
