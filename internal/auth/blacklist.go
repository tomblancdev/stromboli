package auth

import (
	"sync"
	"time"
)

// TokenBlacklist manages blacklisted JWT tokens by their JTI (JWT ID) claim.
// Tokens are stored with their expiration time and automatically cleaned up
// after they naturally expire.
type TokenBlacklist struct {
	tokens    map[string]time.Time // jti -> expiresAt
	mu        sync.RWMutex
	stopChan  chan struct{}
	cleanupMu sync.Mutex
}

// NewTokenBlacklist creates a new token blacklist
func NewTokenBlacklist() *TokenBlacklist {
	return &TokenBlacklist{
		tokens: make(map[string]time.Time),
	}
}

// Add adds a token to the blacklist. The token will remain blacklisted
// until its natural expiration time, after which it will be cleaned up.
func (b *TokenBlacklist) Add(jti string, expiresAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.tokens[jti] = expiresAt
}

// IsBlacklisted checks if a token with the given JTI is blacklisted
func (b *TokenBlacklist) IsBlacklisted(jti string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	_, exists := b.tokens[jti]
	return exists
}

// Size returns the current number of blacklisted tokens
func (b *TokenBlacklist) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.tokens)
}

// StartCleanup starts a background goroutine that removes expired tokens
// at the specified interval. This is idempotent - calling it multiple times
// will not create multiple goroutines.
func (b *TokenBlacklist) StartCleanup(interval time.Duration) {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()

	// If already running, don't start another
	if b.stopChan != nil {
		return
	}

	stop := make(chan struct{})
	b.stopChan = stop
	// Pass stop explicitly so the loop never reads b.stopChan without the lock.
	go b.cleanupLoop(interval, stop)
}

// StopCleanup stops the cleanup goroutine. This is idempotent.
func (b *TokenBlacklist) StopCleanup() {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()

	if b.stopChan != nil {
		close(b.stopChan)
		b.stopChan = nil
	}
}

// cleanupLoop runs the cleanup process at regular intervals
func (b *TokenBlacklist) cleanupLoop(interval time.Duration, stop <-chan struct{}) {
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

// cleanup removes expired tokens from the blacklist
func (b *TokenBlacklist) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for jti, expiresAt := range b.tokens {
		if now.After(expiresAt) {
			delete(b.tokens, jti)
		}
	}
}
