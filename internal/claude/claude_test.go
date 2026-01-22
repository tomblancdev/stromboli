package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	strerrors "github.com/tomblanc/stromboli/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DefaultSecretsFile(t *testing.T) {
	client := NewClient("")
	assert.Equal(t, DefaultSecretsFile, client.secretsFile)
}

func TestNewClient_CustomSecretsFile(t *testing.T) {
	client := NewClient("/path/to/secrets")
	assert.Equal(t, "/path/to/secrets", client.secretsFile)
}

func TestIsConfigured_WhenFileDoesNotExist_ReturnsFalse(t *testing.T) {
	client := NewClient("/nonexistent/path/secrets")
	assert.False(t, client.IsConfigured())
}

func TestIsConfigured_WhenFileExists_ReturnsTrue(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	assert.True(t, client.IsConfigured())
}

func TestGetToken_WhenFileDoesNotExist_ReturnsError(t *testing.T) {
	client := NewClient("/nonexistent/path/secrets")
	token, err := client.GetToken()
	assert.ErrorIs(t, err, strerrors.ErrTokenNotFound)
	assert.Empty(t, token)
}

func TestGetToken_WhenFileExists_ReturnsToken(t *testing.T) {
	// Create temp file with token
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("my-secret-token\n"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	token, err := client.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "my-secret-token", token)
}

func TestGetToken_TrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("  token-with-spaces  \n\n"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	token, err := client.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "token-with-spaces", token)
}

func TestGetToken_CacheHit_DoesNotReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("cached-token"), 0600)
	require.NoError(t, err)

	client := NewClientWithCache(secretsFile, true, 5*time.Second)

	// First call should read and cache
	token1, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "cached-token", token1)

	// Modify file to verify second call uses cache
	err = os.WriteFile(secretsFile, []byte("new-token"), 0600)
	require.NoError(t, err)

	// Second call should return cached value
	token2, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "cached-token", token2, "should return cached token, not read new value")
}

func TestGetToken_CacheMiss_AfterTTL(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("initial-token"), 0600)
	require.NoError(t, err)

	// Set very short TTL for testing
	client := NewClientWithCache(secretsFile, true, 100*time.Millisecond)

	// First call should cache
	token1, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "initial-token", token1)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Update file
	err = os.WriteFile(secretsFile, []byte("updated-token"), 0600)
	require.NoError(t, err)

	// Second call should read fresh value
	token2, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "updated-token", token2, "should read new token after TTL expiration")
}

func TestGetToken_CacheDisabled_AlwaysReads(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("token1"), 0600)
	require.NoError(t, err)

	// Create client with cache disabled
	client := NewClientWithCache(secretsFile, false, 5*time.Second)

	// First call
	token1, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "token1", token1)

	// Update file
	err = os.WriteFile(secretsFile, []byte("token2"), 0600)
	require.NoError(t, err)

	// Second call should read new value (cache disabled)
	token2, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "token2", token2, "should read new token when cache disabled")
}

func TestGetToken_ConcurrentAccess_ThreadSafe(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("concurrent-token"), 0600)
	require.NoError(t, err)

	client := NewClientWithCache(secretsFile, true, 5*time.Second)

	// Run 100 concurrent GetToken calls
	const goroutines = 100
	done := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			token, err := client.GetToken()
			if err != nil {
				done <- ""
			} else {
				done <- token
			}
		}()
	}

	// Collect results
	for i := 0; i < goroutines; i++ {
		token := <-done
		assert.Equal(t, "concurrent-token", token, "all goroutines should get same cached token")
	}
}

func TestGetToken_CacheError_DoesNotCache(t *testing.T) {
	// Create client pointing to non-existent file
	client := NewClientWithCache("/nonexistent/path/secrets", true, 5*time.Second)

	// First call should fail
	_, err := client.GetToken()
	assert.Error(t, err)

	// Create the file now
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err = os.WriteFile(secretsFile, []byte("new-token"), 0600)
	require.NoError(t, err)

	// Update client to point to the new file
	client.secretsFile = secretsFile

	// Second call should succeed (error was not cached)
	token, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
}

func TestNewClient_DefaultCacheEnabled(t *testing.T) {
	client := NewClient("")
	assert.True(t, client.cacheEnabled, "cache should be enabled by default")
	assert.Equal(t, 5*time.Minute, client.cacheTTL, "default TTL should be 5 minutes")
}

func TestNewClientWithCache_CustomSettings(t *testing.T) {
	client := NewClientWithCache("/path/to/secrets", false, 10*time.Minute)
	assert.False(t, client.cacheEnabled, "should use provided cache enabled setting")
	assert.Equal(t, 10*time.Minute, client.cacheTTL, "should use provided TTL")
}

func TestInvalidateCache_ClearsCache(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("initial-token"), 0600)
	require.NoError(t, err)

	client := NewClientWithCache(secretsFile, true, 5*time.Minute)

	// First call should cache
	token1, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "initial-token", token1)

	// Update file
	err = os.WriteFile(secretsFile, []byte("updated-token"), 0600)
	require.NoError(t, err)

	// Invalidate cache
	client.InvalidateCache()

	// Next call should read fresh value
	token2, err := client.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "updated-token", token2, "should read new token after cache invalidation")
}
