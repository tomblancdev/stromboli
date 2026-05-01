package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBoltForTest opens a fresh bolt blacklist in a t.TempDir-scoped path so
// every test gets its own file and bbolt's exclusive file lock doesn't
// cross-contaminate parallel runs.
func newBoltForTest(t *testing.T) *BoltBlacklist {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blacklist.db")
	bl, err := NewBoltBlacklist(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bl.Close() })
	return bl
}

func TestBoltBlacklist_AddAndCheck(t *testing.T) {
	bl := newBoltForTest(t)

	require.NoError(t, bl.Add("jti-1", time.Now().Add(time.Hour)))

	got, err := bl.IsBlacklisted("jti-1")
	require.NoError(t, err)
	assert.True(t, got)

	got, err = bl.IsBlacklisted("jti-unknown")
	require.NoError(t, err)
	assert.False(t, got)
}

func TestBoltBlacklist_RejectsEmptyJTI(t *testing.T) {
	bl := newBoltForTest(t)
	err := bl.Add("", time.Now().Add(time.Hour))
	require.Error(t, err)
}

func TestBoltBlacklist_LazilyFiltersExpired(t *testing.T) {
	// IsBlacklisted should NOT return true for an entry whose expiry is in
	// the past, even before the cleanup goroutine has reaped it. Without
	// lazy filtering, a re-issued token reusing the same JTI (after a clock
	// skew event) would be incorrectly rejected.
	bl := newBoltForTest(t)
	require.NoError(t, bl.Add("expired", time.Now().Add(-time.Hour)))

	got, err := bl.IsBlacklisted("expired")
	require.NoError(t, err)
	assert.False(t, got, "expired entry must not appear blacklisted on read")
}

func TestBoltBlacklist_PersistsAcrossOpen(t *testing.T) {
	// The whole point of the bolt backend: logout survives restart.
	path := filepath.Join(t.TempDir(), "blacklist.db")

	bl, err := NewBoltBlacklist(path)
	require.NoError(t, err)
	require.NoError(t, bl.Add("persistent-jti", time.Now().Add(time.Hour)))
	require.NoError(t, bl.Close())

	// Re-open and confirm the JTI is still present.
	bl2, err := NewBoltBlacklist(path)
	require.NoError(t, err)
	defer bl2.Close()

	got, err := bl2.IsBlacklisted("persistent-jti")
	require.NoError(t, err)
	assert.True(t, got, "persistent backend must survive close/reopen")
}

func TestBoltBlacklist_CleanupRemovesExpired(t *testing.T) {
	bl := newBoltForTest(t)

	require.NoError(t, bl.Add("expired-1", time.Now().Add(-time.Hour)))
	require.NoError(t, bl.Add("expired-2", time.Now().Add(-time.Minute)))
	require.NoError(t, bl.Add("valid", time.Now().Add(time.Hour)))

	require.NoError(t, bl.cleanup())

	n, err := bl.Size()
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the still-valid entry should remain")

	got, err := bl.IsBlacklisted("valid")
	require.NoError(t, err)
	assert.True(t, got)
}

func TestBoltBlacklist_StartupCleansOldEntries(t *testing.T) {
	// Open the DB, add an expired entry directly, close, re-open. The
	// re-open path runs cleanup synchronously so the expired entry should
	// be gone before the first IsBlacklisted call returns.
	path := filepath.Join(t.TempDir(), "blacklist.db")

	bl, err := NewBoltBlacklist(path)
	require.NoError(t, err)
	require.NoError(t, bl.Add("ancient", time.Now().Add(-24*time.Hour)))
	require.NoError(t, bl.Add("fresh", time.Now().Add(time.Hour)))
	require.NoError(t, bl.Close())

	bl2, err := NewBoltBlacklist(path)
	require.NoError(t, err)
	defer bl2.Close()

	n, err := bl2.Size()
	require.NoError(t, err)
	assert.Equal(t, 1, n, "startup cleanup should have removed the ancient entry")
}

func TestBoltBlacklist_OverwritesPriorExpiry(t *testing.T) {
	// Re-Add for an existing JTI should overwrite the prior expiry. This
	// matters for token rotation: a fresh logout extends the deny window.
	bl := newBoltForTest(t)

	near := time.Now().Add(50 * time.Millisecond)
	require.NoError(t, bl.Add("jti", near))

	far := time.Now().Add(time.Hour)
	require.NoError(t, bl.Add("jti", far))

	// Wait past the original expiry; the entry must still be blacklisted
	// because the second Add overwrote the timestamp.
	time.Sleep(150 * time.Millisecond)
	got, err := bl.IsBlacklisted("jti")
	require.NoError(t, err)
	assert.True(t, got, "later Add must extend the deny window")
}

func TestBoltBlacklist_RejectsEmptyPath(t *testing.T) {
	_, err := NewBoltBlacklist("")
	require.Error(t, err)
}
