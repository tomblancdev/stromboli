package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenBlacklist_ReturnsInitializedBlacklist(t *testing.T) {
	bl := NewTokenBlacklist()
	require.NotNil(t, bl)

	// Should not be blacklisted by default
	assert.False(t, bl.IsBlacklisted("any-jti"))
}

func TestTokenBlacklist_Add_BlacklistsToken(t *testing.T) {
	bl := NewTokenBlacklist()

	jti := "test-jti-12345"
	expiresAt := time.Now().Add(time.Hour)

	bl.Add(jti, expiresAt)

	assert.True(t, bl.IsBlacklisted(jti))
}

func TestTokenBlacklist_IsBlacklisted_ReturnsFalseForUnknownToken(t *testing.T) {
	bl := NewTokenBlacklist()

	// Add one token
	bl.Add("known-jti", time.Now().Add(time.Hour))

	// Check for a different token
	assert.False(t, bl.IsBlacklisted("unknown-jti"))
}

func TestTokenBlacklist_Add_MultipleTokens(t *testing.T) {
	bl := NewTokenBlacklist()

	tokens := []string{"jti-1", "jti-2", "jti-3"}
	expiresAt := time.Now().Add(time.Hour)

	for _, jti := range tokens {
		bl.Add(jti, expiresAt)
	}

	for _, jti := range tokens {
		assert.True(t, bl.IsBlacklisted(jti), "Token %s should be blacklisted", jti)
	}
}

func TestTokenBlacklist_Cleanup_RemovesExpiredTokens(t *testing.T) {
	bl := NewTokenBlacklist()

	// Add an already expired token
	expiredJTI := "expired-jti"
	bl.Add(expiredJTI, time.Now().Add(-time.Hour)) // Expired 1 hour ago

	// Add a valid token
	validJTI := "valid-jti"
	bl.Add(validJTI, time.Now().Add(time.Hour)) // Expires in 1 hour

	// Run cleanup manually
	bl.cleanup()

	// Expired token should be removed
	assert.False(t, bl.IsBlacklisted(expiredJTI), "Expired token should be removed after cleanup")

	// Valid token should still be blacklisted
	assert.True(t, bl.IsBlacklisted(validJTI), "Valid token should still be blacklisted")
}

func TestTokenBlacklist_StartCleanup_IsIdempotent(t *testing.T) {
	bl := NewTokenBlacklist()

	// Start cleanup multiple times - should not panic or create multiple goroutines
	bl.StartCleanup(time.Hour)
	bl.StartCleanup(time.Hour)
	bl.StartCleanup(time.Hour)

	// Should still work normally
	bl.Add("test-jti", time.Now().Add(time.Hour))
	assert.True(t, bl.IsBlacklisted("test-jti"))

	bl.StopCleanup()
}

func TestTokenBlacklist_StopCleanup_IsIdempotent(t *testing.T) {
	bl := NewTokenBlacklist()

	// Stop without starting - should not panic
	bl.StopCleanup()

	// Start and stop multiple times - should not panic
	bl.StartCleanup(time.Hour)
	bl.StopCleanup()
	bl.StopCleanup()
}

func TestTokenBlacklist_CleanupLoop_CleansExpiredTokens(t *testing.T) {
	bl := NewTokenBlacklist()

	// Add a token that will expire very soon
	shortLivedJTI := "short-lived-jti"
	bl.Add(shortLivedJTI, time.Now().Add(50*time.Millisecond))

	// Add a longer-lived token
	longLivedJTI := "long-lived-jti"
	bl.Add(longLivedJTI, time.Now().Add(time.Hour))

	// Both should be blacklisted initially
	assert.True(t, bl.IsBlacklisted(shortLivedJTI))
	assert.True(t, bl.IsBlacklisted(longLivedJTI))

	// Start cleanup with short interval
	bl.StartCleanup(100 * time.Millisecond)
	defer bl.StopCleanup()

	// Wait for expiration and cleanup
	time.Sleep(200 * time.Millisecond)

	// Short-lived token should be cleaned up
	assert.False(t, bl.IsBlacklisted(shortLivedJTI), "Expired token should be removed by cleanup")

	// Long-lived token should still be blacklisted
	assert.True(t, bl.IsBlacklisted(longLivedJTI), "Valid token should still be blacklisted")
}

func TestTokenBlacklist_ConcurrentAccess(t *testing.T) {
	bl := NewTokenBlacklist()

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent Add operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			jti := "jti-" + string(rune(id))
			bl.Add(jti, time.Now().Add(time.Hour))
		}(i)
	}

	// Concurrent IsBlacklisted operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			jti := "jti-" + string(rune(id))
			bl.IsBlacklisted(jti)
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions
}

func TestTokenBlacklist_Size(t *testing.T) {
	bl := NewTokenBlacklist()

	assert.Equal(t, 0, bl.Size())

	bl.Add("jti-1", time.Now().Add(time.Hour))
	assert.Equal(t, 1, bl.Size())

	bl.Add("jti-2", time.Now().Add(time.Hour))
	assert.Equal(t, 2, bl.Size())

	// Adding same token should not increase size
	bl.Add("jti-1", time.Now().Add(time.Hour))
	assert.Equal(t, 2, bl.Size())
}
