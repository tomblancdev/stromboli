// Package config provides centralized configuration management for Stromboli
// using Viper. It supports configuration from multiple sources with the following priority:
// 1. Environment variables (highest)
// 2. Config file
// 3. Defaults (lowest)
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Agent     AgentConfig
	Resources ResourceConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	JWT       JWTConfig
	Jobs      JobsConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Address string // Server address (default: ":8080")
}

// AgentConfig holds agent container configuration
type AgentConfig struct {
	Image       string           // Container image name
	SecretsFile string           // Path to Claude secrets file
	SessionsDir string           // Directory for session data
	TokenCache  TokenCacheConfig // Token cache configuration
}

// TokenCacheConfig holds token caching configuration
type TokenCacheConfig struct {
	Enabled bool          // Enable token caching
	TTL     time.Duration // Cache time-to-live
}

// ResourceConfig holds default resource limits for containers
type ResourceConfig struct {
	Memory  string // Memory limit (e.g., "512m", "1g")
	CPUs    string // CPU limit (e.g., "1", "2")
	Timeout string // Execution timeout (e.g., "30m", "1h")
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled     bool     // Enable authentication
	ValidTokens []string // List of valid API tokens (legacy)
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled bool          // Enable rate limiting
	Rate    int           // Requests per second
	Burst   int           // Maximum burst size
	Period  time.Duration // Period for rate limit (always 1 second)
}

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	Secret        string        // Secret key for signing tokens
	AccessExpiry  time.Duration // Access token lifetime
	RefreshExpiry time.Duration // Refresh token lifetime
}

// JobsConfig holds job management configuration
type JobsConfig struct {
	CleanupTTL      time.Duration // How long to keep completed jobs
	CleanupInterval time.Duration // How often to run cleanup
}

// Default values
const (
	defaultServerAddress   = ":8080"
	defaultAgentImage      = "stromboli-agent:latest"
	defaultSecretsFile     = ".claude-secrets"
	defaultSessionsDir     = ".stromboli/sessions"
	defaultMemory          = "512m"
	defaultCPUs            = "1"
	defaultTimeout         = "30m"
	defaultRateLimitRate   = 10
	defaultRateLimitBurst  = 20
	defaultJWTAccessExpiry = 24 * time.Hour
	defaultJWTRefresh      = 7 * 24 * time.Hour
	defaultCleanupTTL      = 1 * time.Hour
	defaultCleanupInterval = 5 * time.Minute
	defaultTokenCacheTTL   = 5 * time.Minute
)

// Load reads configuration from environment variables, config files, and defaults.
// Priority: Environment variables > Config file > Defaults
func Load() (*Config, error) {
	v := viper.New()
	setupViper(v)

	// Look for config files in multiple locations
	v.SetConfigName("stromboli")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")              // Current directory
	v.AddConfigPath("$HOME/.stromboli") // User home directory
	v.AddConfigPath("/etc/stromboli") // System-wide config

	// Attempt to read config file (non-fatal if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK - we'll use env vars and defaults
	}

	return parseConfig(v)
}

// LoadFromFile loads configuration from a specific file path
func LoadFromFile(path string) (*Config, error) {
	v := viper.New()
	setupViper(v)

	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	return parseConfig(v)
}

// setupViper configures viper with defaults and environment variable mapping
func setupViper(v *viper.Viper) {
	// Set defaults
	v.SetDefault("server.address", defaultServerAddress)
	v.SetDefault("agent.image", defaultAgentImage)
	v.SetDefault("agent.secrets_file", defaultSecretsFile)
	v.SetDefault("agent.sessions_dir", defaultSessionsDir)
	v.SetDefault("agent.token_cache.enabled", true)
	v.SetDefault("agent.token_cache.ttl", defaultTokenCacheTTL.String())
	v.SetDefault("resources.memory", defaultMemory)
	v.SetDefault("resources.cpus", defaultCPUs)
	v.SetDefault("resources.timeout", defaultTimeout)
	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.valid_tokens", []string{})
	v.SetDefault("rate_limit.enabled", false)
	v.SetDefault("rate_limit.rate", defaultRateLimitRate)
	v.SetDefault("rate_limit.burst", defaultRateLimitBurst)
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.access_expiry", defaultJWTAccessExpiry.String())
	v.SetDefault("jwt.refresh_expiry", defaultJWTRefresh.String())
	v.SetDefault("jobs.cleanup_ttl", defaultCleanupTTL.String())
	v.SetDefault("jobs.cleanup_interval", defaultCleanupInterval.String())

	// Environment variable configuration
	v.SetEnvPrefix("STROMBOLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map legacy environment variables for backward compatibility
	v.BindEnv("server.address", "STROMBOLI_SERVER_ADDRESS")
	v.BindEnv("agent.image", "STROMBOLI_AGENT_IMAGE")
	v.BindEnv("agent.secrets_file", "STROMBOLI_AGENT_SECRETS_FILE")
	v.BindEnv("agent.sessions_dir", "STROMBOLI_AGENT_SESSIONS_DIR")
	v.BindEnv("agent.token_cache.enabled", "STROMBOLI_TOKEN_CACHE_ENABLED")
	v.BindEnv("agent.token_cache.ttl", "STROMBOLI_TOKEN_CACHE_TTL")
	v.BindEnv("resources.memory", "STROMBOLI_DEFAULT_MEMORY")
	v.BindEnv("resources.cpus", "STROMBOLI_DEFAULT_CPUS")
	v.BindEnv("resources.timeout", "STROMBOLI_DEFAULT_TIMEOUT")
	v.BindEnv("auth.enabled", "STROMBOLI_AUTH_ENABLED")
	v.BindEnv("auth.valid_tokens", "STROMBOLI_API_TOKENS")
	v.BindEnv("rate_limit.enabled", "STROMBOLI_RATE_LIMIT_ENABLED")
	v.BindEnv("rate_limit.rate", "STROMBOLI_RATE_LIMIT_RPS")
	v.BindEnv("rate_limit.burst", "STROMBOLI_RATE_LIMIT_BURST")
	v.BindEnv("jwt.secret", "STROMBOLI_JWT_SECRET")
	v.BindEnv("jwt.access_expiry", "STROMBOLI_JWT_EXPIRY")
	v.BindEnv("jwt.refresh_expiry", "STROMBOLI_JWT_REFRESH_EXPIRY")
	v.BindEnv("jobs.cleanup_ttl", "STROMBOLI_JOBS_CLEANUP_TTL")
	v.BindEnv("jobs.cleanup_interval", "STROMBOLI_JOBS_CLEANUP_INTERVAL")
}

// parseConfig extracts and validates configuration from viper
func parseConfig(v *viper.Viper) (*Config, error) {
	// Parse token cache TTL
	cacheTTL, err := parseDuration(v.GetString("agent.token_cache.ttl"))
	if err != nil {
		return nil, fmt.Errorf("invalid token cache TTL: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Address: v.GetString("server.address"),
		},
		Agent: AgentConfig{
			Image:       v.GetString("agent.image"),
			SecretsFile: v.GetString("agent.secrets_file"),
			SessionsDir: v.GetString("agent.sessions_dir"),
			TokenCache: TokenCacheConfig{
				Enabled: v.GetBool("agent.token_cache.enabled"),
				TTL:     cacheTTL,
			},
		},
		Resources: ResourceConfig{
			Memory:  v.GetString("resources.memory"),
			CPUs:    v.GetString("resources.cpus"),
			Timeout: v.GetString("resources.timeout"),
		},
		Auth: AuthConfig{
			Enabled:     v.GetBool("auth.enabled"),
			ValidTokens: parseTokens(v.GetString("auth.valid_tokens")),
		},
		RateLimit: RateLimitConfig{
			Enabled: v.GetBool("rate_limit.enabled"),
			Rate:    v.GetInt("rate_limit.rate"),
			Burst:   v.GetInt("rate_limit.burst"),
			Period:  time.Second,
		},
		JWT: JWTConfig{
			Secret: v.GetString("jwt.secret"),
		},
	}

	// Parse JWT durations
	accessExpiry, err := parseDuration(v.GetString("jwt.access_expiry"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT access expiry: %w", err)
	}
	cfg.JWT.AccessExpiry = accessExpiry

	refreshExpiry, err := parseDuration(v.GetString("jwt.refresh_expiry"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT refresh expiry: %w", err)
	}
	cfg.JWT.RefreshExpiry = refreshExpiry

	// Parse job cleanup durations
	cleanupTTL, err := parseDuration(v.GetString("jobs.cleanup_ttl"))
	if err != nil {
		return nil, fmt.Errorf("invalid jobs cleanup TTL: %w", err)
	}
	cfg.Jobs.CleanupTTL = cleanupTTL

	cleanupInterval, err := parseDuration(v.GetString("jobs.cleanup_interval"))
	if err != nil {
		return nil, fmt.Errorf("invalid jobs cleanup interval: %w", err)
	}
	cfg.Jobs.CleanupInterval = cleanupInterval

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// parseTokens parses a comma-separated token string into a slice
func parseTokens(tokensStr string) []string {
	if tokensStr == "" {
		return []string{}
	}
	tokens := strings.Split(tokensStr, ",")
	// Trim whitespace from each token
	for i := range tokens {
		tokens[i] = strings.TrimSpace(tokens[i])
	}
	return tokens
}

// parseDuration parses a duration string, handling both Go durations and integers
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// Validate checks that the configuration is valid
func (c *Config) Validate() error {
	// Validate rate limit values
	if c.RateLimit.Enabled {
		if c.RateLimit.Rate <= 0 {
			return fmt.Errorf("rate limit RPS must be positive, got %d", c.RateLimit.Rate)
		}
		if c.RateLimit.Burst <= 0 {
			return fmt.Errorf("rate limit burst must be positive, got %d", c.RateLimit.Burst)
		}
	}

	// Validate JWT config
	if c.JWT.Secret != "" {
		if c.JWT.AccessExpiry <= 0 {
			return fmt.Errorf("JWT access expiry must be positive")
		}
		if c.JWT.RefreshExpiry <= 0 {
			return fmt.Errorf("JWT refresh expiry must be positive")
		}
	}

	// Validate job cleanup config
	if c.Jobs.CleanupTTL <= 0 {
		return fmt.Errorf("job cleanup TTL must be positive")
	}
	if c.Jobs.CleanupInterval <= 0 {
		return fmt.Errorf("job cleanup interval must be positive")
	}

	// Validate token cache config
	if c.Agent.TokenCache.Enabled && c.Agent.TokenCache.TTL <= 0 {
		return fmt.Errorf("token cache TTL must be positive when cache is enabled")
	}

	return nil
}
