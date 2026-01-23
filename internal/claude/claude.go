package claude

import (
	"os"
	"strings"
	"sync"
	"time"

	strerrors "stromboli/internal/errors"
)

const (
	DefaultSecretsFile = ".claude-secrets"
	DefaultCacheTTL    = 5 * time.Minute
)

// tokenCache holds cached token data with expiration
type tokenCache struct {
	token     string
	expiresAt time.Time
}

// Client wraps Claude Code token operations with caching support
type Client struct {
	secretsFile  string
	cacheEnabled bool
	cacheTTL     time.Duration
	cache        *tokenCache
	mu           sync.RWMutex
}

// NewClient creates a new Claude client with default cache settings
// Cache is enabled by default with 5 minute TTL
func NewClient(secretsFile string) *Client {
	if secretsFile == "" {
		secretsFile = DefaultSecretsFile
	}
	return &Client{
		secretsFile:  secretsFile,
		cacheEnabled: true,
		cacheTTL:     DefaultCacheTTL,
	}
}

// NewClientWithCache creates a new Claude client with custom cache settings
func NewClientWithCache(secretsFile string, cacheEnabled bool, cacheTTL time.Duration) *Client {
	if secretsFile == "" {
		secretsFile = DefaultSecretsFile
	}
	return &Client{
		secretsFile:  secretsFile,
		cacheEnabled: cacheEnabled,
		cacheTTL:     cacheTTL,
	}
}

// IsConfigured checks if token file exists
func (c *Client) IsConfigured() bool {
	_, err := os.Stat(c.secretsFile)
	return err == nil
}

// GetToken reads the token from secrets file with caching support
// If cache is enabled and valid, returns cached token.
// Otherwise reads from file and updates cache.
func (c *Client) GetToken() (string, error) {
	// Try to get from cache first
	if c.cacheEnabled {
		c.mu.RLock()
		if c.cache != nil && time.Now().Before(c.cache.expiresAt) {
			token := c.cache.token
			c.mu.RUnlock()
			return token, nil
		}
		c.mu.RUnlock()
	}

	// Cache miss or disabled - read from file
	data, err := os.ReadFile(c.secretsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", strerrors.ErrTokenNotFound
		}
		return "", err
	}

	token := strings.TrimSpace(string(data))

	// Update cache if enabled
	if c.cacheEnabled {
		c.mu.Lock()
		c.cache = &tokenCache{
			token:     token,
			expiresAt: time.Now().Add(c.cacheTTL),
		}
		c.mu.Unlock()
	}

	return token, nil
}

// InvalidateCache clears the cached token, forcing next GetToken to read from file
func (c *Client) InvalidateCache() {
	c.mu.Lock()
	c.cache = nil
	c.mu.Unlock()
}
