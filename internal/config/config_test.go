package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validJWTSecret is a 44-char base64 string used in tests where we need auth to
// pass validation. Generated with `openssl rand -base64 32`.
const validJWTSecret = "qY3FvR0xUd5HxN6tA8bC9eK2mZ4pL7vJ8sQ1wE3rT5o="

func TestLoad_Defaults(t *testing.T) {
	// Clean environment, then provide a valid JWT secret so the secure-by-default
	// auth check passes — we still want to verify the *other* defaults below.
	cleanEnv(t)
	t.Setenv("STROMBOLI_JWT_SECRET", validJWTSecret)

	cfg, err := Load()
	require.NoError(t, err)

	// Test server defaults
	assert.Equal(t, ":8080", cfg.Server.Address)

	// Test agent defaults
	assert.Equal(t, "stromboli-agent", cfg.Agent.Image)
	assert.Equal(t, "~/.claude/.credentials.json", cfg.Agent.CredentialsFile)
	assert.Equal(t, ".stromboli/sessions", cfg.Agent.SessionsDir)

	// Test resource defaults
	assert.Equal(t, "512m", cfg.Resources.Memory)
	assert.Equal(t, "1", cfg.Resources.CPUs)
	assert.Equal(t, "30m", cfg.Resources.Timeout)

	// Test auth defaults (enabled — secure by default)
	assert.True(t, cfg.Auth.Enabled)
	assert.Empty(t, cfg.Auth.ValidTokens)

	// Test rate limit defaults (disabled)
	assert.False(t, cfg.RateLimit.Enabled)
	assert.Equal(t, 10, cfg.RateLimit.Rate)
	assert.Equal(t, 20, cfg.RateLimit.Burst)
	assert.Equal(t, time.Second, cfg.RateLimit.Period)

	// Test JWT defaults (secret was injected above; durations come from defaults)
	assert.Equal(t, validJWTSecret, cfg.JWT.Secret)
	assert.Equal(t, 24*time.Hour, cfg.JWT.AccessExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.JWT.RefreshExpiry)

	// Test job cleanup defaults
	assert.Equal(t, time.Hour, cfg.Jobs.CleanupTTL)
	assert.Equal(t, 5*time.Minute, cfg.Jobs.CleanupInterval)

	// Test token cache defaults
	assert.True(t, cfg.Agent.TokenCache.Enabled)
	assert.Equal(t, 5*time.Minute, cfg.Agent.TokenCache.TTL)
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	cleanEnv(t)

	// Set environment variables
	os.Setenv("STROMBOLI_SERVER_ADDRESS", ":9090")
	os.Setenv("STROMBOLI_AGENT_IMAGE", "custom-agent:v1")
	os.Setenv("STROMBOLI_AGENT_CREDENTIALS_FILE", "/tmp/credentials.json")
	os.Setenv("STROMBOLI_AGENT_SESSIONS_DIR", "/tmp/sessions")
	os.Setenv("STROMBOLI_DEFAULT_MEMORY", "1g")
	os.Setenv("STROMBOLI_DEFAULT_CPUS", "2")
	os.Setenv("STROMBOLI_DEFAULT_TIMEOUT", "1h")
	os.Setenv("STROMBOLI_AUTH_ENABLED", "true")
	os.Setenv("STROMBOLI_API_TOKENS", "token1,token2,token3")
	os.Setenv("STROMBOLI_RATE_LIMIT_ENABLED", "true")
	os.Setenv("STROMBOLI_RATE_LIMIT_RPS", "50")
	os.Setenv("STROMBOLI_RATE_LIMIT_BURST", "100")
	os.Setenv("STROMBOLI_JWT_SECRET", validJWTSecret)
	os.Setenv("STROMBOLI_JWT_EXPIRY", "12h")
	os.Setenv("STROMBOLI_JWT_REFRESH_EXPIRY", "72h")
	os.Setenv("STROMBOLI_TOKEN_CACHE_ENABLED", "false")
	os.Setenv("STROMBOLI_TOKEN_CACHE_TTL", "10m")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.Equal(t, "custom-agent:v1", cfg.Agent.Image)
	assert.Equal(t, "/tmp/credentials.json", cfg.Agent.CredentialsFile)
	assert.Equal(t, "/tmp/sessions", cfg.Agent.SessionsDir)
	assert.Equal(t, "1g", cfg.Resources.Memory)
	assert.Equal(t, "2", cfg.Resources.CPUs)
	assert.Equal(t, "1h", cfg.Resources.Timeout)
	assert.True(t, cfg.Auth.Enabled)
	assert.Equal(t, []string{"token1", "token2", "token3"}, cfg.Auth.ValidTokens)
	assert.True(t, cfg.RateLimit.Enabled)
	assert.Equal(t, 50, cfg.RateLimit.Rate)
	assert.Equal(t, 100, cfg.RateLimit.Burst)
	assert.Equal(t, validJWTSecret, cfg.JWT.Secret)
	assert.Equal(t, 12*time.Hour, cfg.JWT.AccessExpiry)
	assert.Equal(t, 72*time.Hour, cfg.JWT.RefreshExpiry)
	assert.False(t, cfg.Agent.TokenCache.Enabled)
	assert.Equal(t, 10*time.Minute, cfg.Agent.TokenCache.TTL)
}

func TestLoad_BackwardCompatibility(t *testing.T) {
	// Test that legacy env vars still work
	cleanEnv(t)

	os.Setenv("STROMBOLI_AUTH_ENABLED", "true")
	os.Setenv("STROMBOLI_JWT_SECRET", validJWTSecret)
	os.Setenv("STROMBOLI_API_TOKENS", "legacy1,legacy2")
	os.Setenv("STROMBOLI_RATE_LIMIT_ENABLED", "true")
	os.Setenv("STROMBOLI_RATE_LIMIT_RPS", "25")
	os.Setenv("STROMBOLI_RATE_LIMIT_BURST", "50")
	os.Setenv("STROMBOLI_DEFAULT_MEMORY", "2g")
	os.Setenv("STROMBOLI_DEFAULT_CPUS", "4")
	os.Setenv("STROMBOLI_DEFAULT_TIMEOUT", "2h")

	cfg, err := Load()
	require.NoError(t, err)

	assert.True(t, cfg.Auth.Enabled)
	assert.Equal(t, []string{"legacy1", "legacy2"}, cfg.Auth.ValidTokens)
	assert.True(t, cfg.RateLimit.Enabled)
	assert.Equal(t, 25, cfg.RateLimit.Rate)
	assert.Equal(t, 50, cfg.RateLimit.Burst)
	assert.Equal(t, "2g", cfg.Resources.Memory)
	assert.Equal(t, "4", cfg.Resources.CPUs)
	assert.Equal(t, "2h", cfg.Resources.Timeout)
}

func TestLoad_ConfigFile(t *testing.T) {
	cleanEnv(t)

	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stromboli.yaml")

	configContent := `
server:
  address: ":7070"

agent:
  image: "file-agent:latest"
  credentials_file: "/config/credentials.json"
  sessions_dir: "/config/sessions"
  token_cache:
    enabled: false
    ttl: "1m"

resources:
  memory: "4g"
  cpus: "8"
  timeout: "4h"

auth:
  enabled: true
  valid_tokens:
    - file-token1
    - file-token2

rate_limit:
  enabled: true
  rate: 100
  burst: 200

jwt:
  secret: "qY3FvR0xUd5HxN6tA8bC9eK2mZ4pL7vJ8sQ1wE3rT5o="
  access_expiry: "6h"
  refresh_expiry: "48h"

jobs:
  cleanup_ttl: "2h"
  cleanup_interval: "10m"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config with explicit path
	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, ":7070", cfg.Server.Address)
	assert.Equal(t, "file-agent:latest", cfg.Agent.Image)
	assert.Equal(t, "/config/credentials.json", cfg.Agent.CredentialsFile)
	assert.Equal(t, "/config/sessions", cfg.Agent.SessionsDir)
	assert.Equal(t, "4g", cfg.Resources.Memory)
	assert.Equal(t, "8", cfg.Resources.CPUs)
	assert.Equal(t, "4h", cfg.Resources.Timeout)
	assert.True(t, cfg.Auth.Enabled)
	assert.Equal(t, []string{"file-token1", "file-token2"}, cfg.Auth.ValidTokens)
	assert.True(t, cfg.RateLimit.Enabled)
	assert.Equal(t, 100, cfg.RateLimit.Rate)
	assert.Equal(t, 200, cfg.RateLimit.Burst)
	assert.Equal(t, validJWTSecret, cfg.JWT.Secret)
	assert.Equal(t, 6*time.Hour, cfg.JWT.AccessExpiry)
	assert.Equal(t, 48*time.Hour, cfg.JWT.RefreshExpiry)
	assert.Equal(t, 2*time.Hour, cfg.Jobs.CleanupTTL)
	assert.Equal(t, 10*time.Minute, cfg.Jobs.CleanupInterval)
	assert.False(t, cfg.Agent.TokenCache.Enabled)
	assert.Equal(t, time.Minute, cfg.Agent.TokenCache.TTL)
}

func TestLoad_EnvOverridesConfigFile(t *testing.T) {
	cleanEnv(t)

	// Create temporary config file with some values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stromboli.yaml")

	configContent := `
server:
  address: ":7070"

auth:
  enabled: false
  valid_tokens:
    - file-token
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Override with environment variables
	os.Setenv("STROMBOLI_SERVER_ADDRESS", ":8888")
	os.Setenv("STROMBOLI_AUTH_ENABLED", "true")
	os.Setenv("STROMBOLI_JWT_SECRET", validJWTSecret)
	os.Setenv("STROMBOLI_API_TOKENS", "env-token")

	cfg, err := LoadFromFile(configPath)
	require.NoError(t, err)

	// Env vars should override file values
	assert.Equal(t, ":8888", cfg.Server.Address)
	assert.True(t, cfg.Auth.Enabled)
	assert.Equal(t, []string{"env-token"}, cfg.Auth.ValidTokens)
}

func TestLoad_InvalidDuration(t *testing.T) {
	cleanEnv(t)

	os.Setenv("STROMBOLI_JWT_EXPIRY", "invalid")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JWT access expiry")
}

func TestLoad_InvalidRateLimit(t *testing.T) {
	cleanEnv(t)

	os.Setenv("STROMBOLI_RATE_LIMIT_ENABLED", "true")
	os.Setenv("STROMBOLI_RATE_LIMIT_RPS", "-5")

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit RPS must be positive")
}

func TestLoad_TokenCacheInvalidTTL(t *testing.T) {
	cleanEnv(t)

	os.Setenv("STROMBOLI_TOKEN_CACHE_ENABLED", "true")
	os.Setenv("STROMBOLI_TOKEN_CACHE_TTL", "invalid")

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token cache TTL")
}

func TestLoad_TokenCacheEnabledWithoutTTL(t *testing.T) {
	cleanEnv(t)

	// Create temporary config file with cache enabled but TTL = 0.
	// Disable auth so the validation we're testing is the token-cache one,
	// not the JWT-secret gate.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stromboli.yaml")

	configContent := `
auth:
  enabled: false

agent:
  token_cache:
    enabled: true
    ttl: "0s"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = LoadFromFile(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token cache TTL must be positive when cache is enabled")
}

func TestLoad_AuthEnabledRequiresJWTSecret(t *testing.T) {
	cleanEnv(t)
	// Auth is on by default; no JWT_SECRET set → must fail.
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "STROMBOLI_JWT_SECRET is empty")
	assert.Contains(t, err.Error(), "openssl rand -base64 32")
}

func TestLoad_AuthDisabledAllowsEmptyJWTSecret(t *testing.T) {
	cleanEnv(t)
	t.Setenv("STROMBOLI_AUTH_ENABLED", "false")
	cfg, err := Load()
	require.NoError(t, err)
	assert.False(t, cfg.Auth.Enabled)
	assert.Empty(t, cfg.JWT.Secret)
}

func TestLoad_AuthEnabledRejectsShortJWTSecret(t *testing.T) {
	cleanEnv(t)
	t.Setenv("STROMBOLI_JWT_SECRET", "short")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestLoad_AuthEnabledRejectsPlaceholderJWTSecret(t *testing.T) {
	cleanEnv(t)
	// Long enough to pass the length check, but a known placeholder value.
	t.Setenv("STROMBOLI_JWT_SECRET", "generate-with-openssl-rand-base64-32")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder")
}

// cleanEnv removes all STROMBOLI_ environment variables for testing
func cleanEnv(t *testing.T) {
	t.Helper()
	for _, env := range os.Environ() {
		// env is "KEY=value", so split on first "="
		idx := 0
		for i, c := range env {
			if c == '=' {
				idx = i
				break
			}
		}
		if idx > 0 {
			key := env[:idx]
			if len(key) >= 10 && key[:10] == "STROMBOLI_" {
				os.Unsetenv(key)
			}
		}
	}
}
