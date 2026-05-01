package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blocked is a tiny test helper that hides the (bool, error) signature
// noise — these tests target MemoryBlacklist whose IsBlacklisted never
// returns an error, so an err here is always a real test failure.
func blocked(t *testing.T, bl Blacklist, jti string) bool {
	t.Helper()
	ok, err := bl.IsBlacklisted(jti)
	require.NoError(t, err)
	return ok
}

func size(t *testing.T, bl Blacklist) int {
	t.Helper()
	n, err := bl.Size()
	require.NoError(t, err)
	return n
}

func TestNewTokenBlacklist_ReturnsInitializedBlacklist(t *testing.T) {
	bl := NewMemoryBlacklist()
	require.NotNil(t, bl)

	// Should not be blacklisted by default
	assert.False(t, blocked(t, bl, "any-jti"))
}

func TestTokenBlacklist_Add_BlacklistsToken(t *testing.T) {
	bl := NewMemoryBlacklist()

	jti := "test-jti-12345"
	expiresAt := time.Now().Add(time.Hour)

	require.NoError(t, bl.Add(jti, expiresAt))

	assert.True(t, blocked(t, bl, jti))
}

func TestTokenBlacklist_IsBlacklisted_ReturnsFalseForUnknownToken(t *testing.T) {
	bl := NewMemoryBlacklist()

	require.NoError(t, bl.Add("known-jti", time.Now().Add(time.Hour)))

	assert.False(t, blocked(t, bl, "unknown-jti"))
}

func TestTokenBlacklist_Add_MultipleTokens(t *testing.T) {
	bl := NewMemoryBlacklist()

	tokens := []string{"jti-1", "jti-2", "jti-3"}
	expiresAt := time.Now().Add(time.Hour)

	for _, jti := range tokens {
		require.NoError(t, bl.Add(jti, expiresAt))
	}

	for _, jti := range tokens {
		assert.True(t, blocked(t, bl, jti), "Token %s should be blacklisted", jti)
	}
}

func TestTokenBlacklist_Cleanup_RemovesExpiredTokens(t *testing.T) {
	bl := NewMemoryBlacklist()

	expiredJTI := "expired-jti"
	require.NoError(t, bl.Add(expiredJTI, time.Now().Add(-time.Hour)))

	validJTI := "valid-jti"
	require.NoError(t, bl.Add(validJTI, time.Now().Add(time.Hour)))

	bl.cleanup()

	assert.False(t, blocked(t, bl, expiredJTI), "Expired token should be removed after cleanup")
	assert.True(t, blocked(t, bl, validJTI), "Valid token should still be blacklisted")
}

func TestTokenBlacklist_StartCleanup_IsIdempotent(t *testing.T) {
	bl := NewMemoryBlacklist()

	bl.StartCleanup(time.Hour)
	bl.StartCleanup(time.Hour)
	bl.StartCleanup(time.Hour)

	require.NoError(t, bl.Add("test-jti", time.Now().Add(time.Hour)))
	assert.True(t, blocked(t, bl, "test-jti"))

	bl.StopCleanup()
}

func TestTokenBlacklist_StopCleanup_IsIdempotent(t *testing.T) {
	bl := NewMemoryBlacklist()

	bl.StopCleanup()

	bl.StartCleanup(time.Hour)
	bl.StopCleanup()
	bl.StopCleanup()
}

func TestTokenBlacklist_CleanupLoop_CleansExpiredTokens(t *testing.T) {
	bl := NewMemoryBlacklist()

	shortLivedJTI := "short-lived-jti"
	require.NoError(t, bl.Add(shortLivedJTI, time.Now().Add(50*time.Millisecond)))

	longLivedJTI := "long-lived-jti"
	require.NoError(t, bl.Add(longLivedJTI, time.Now().Add(time.Hour)))

	assert.True(t, blocked(t, bl, shortLivedJTI))
	assert.True(t, blocked(t, bl, longLivedJTI))

	bl.StartCleanup(50 * time.Millisecond)
	defer bl.StopCleanup()

	// Wait for the cleanup goroutine to reap the expired entry. Polling
	// instead of a fixed sleep keeps the test fast and tolerant of slow
	// CI schedulers.
	require.Eventually(t, func() bool {
		return !blocked(t, bl, shortLivedJTI)
	}, 2*time.Second, 10*time.Millisecond, "expired token never reaped")

	assert.True(t, blocked(t, bl, longLivedJTI), "Valid token should still be blacklisted")
}

func TestTokenBlacklist_ConcurrentAccess(t *testing.T) {
	bl := NewMemoryBlacklist()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			jti := "jti-" + string(rune(id))
			_ = bl.Add(jti, time.Now().Add(time.Hour))
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			jti := "jti-" + string(rune(id))
			_, _ = bl.IsBlacklisted(jti)
		}(i)
	}

	wg.Wait()
}

func TestTokenBlacklist_Size(t *testing.T) {
	bl := NewMemoryBlacklist()

	assert.Equal(t, 0, size(t, bl))

	require.NoError(t, bl.Add("jti-1", time.Now().Add(time.Hour)))
	assert.Equal(t, 1, size(t, bl))

	require.NoError(t, bl.Add("jti-2", time.Now().Add(time.Hour)))
	assert.Equal(t, 2, size(t, bl))

	// Adding same token should not increase size
	require.NoError(t, bl.Add("jti-1", time.Now().Add(time.Hour)))
	assert.Equal(t, 2, size(t, bl))
}
