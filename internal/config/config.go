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
	Tracing   TracingConfig
	Compose   ComposeConfig
	Metrics   MetricsConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Address string // Server address (default: ":8080")
}

// AgentConfig holds agent container configuration
type AgentConfig struct {
	Image                string           // Container image name (default)
	ImageTag             string           // Container image tag
	AllowedImagePatterns []string         // Allowed image patterns (e.g., "python:*", "golang:*")
	AllowedVolumes       []string         // Allowed host paths for volume mounts
	AllowAllVolumes      bool             // DANGEROUS: Allow all volume mounts when allowlist is empty (dev only!)
	WorkdirAutoCreate    bool             // Auto-create workdir if it doesn't exist (inside container)
	VolumeAutoCreate     bool             // Auto-create host directories for volume mounts
	MountClaudeCLI       bool             // Mount claude-cli volume into containers
	CLIImage             string           // Claude CLI image for mounting (e.g., ghcr.io/tomblancdev/stromboli-agent)
	CLIImageTag          string           // Claude CLI image tag
	AutoPullCLI          bool             // Auto-pull CLI image on startup if missing
	CredentialsFile      string           // Path to Claude credentials file (~/.claude/.credentials.json)
	SessionsDir          string           // Directory for session data (internal container path)
	SessionsHostDir      string           // Host path for sessions (for nested container mounts, defaults to SessionsDir)
	TokenCache           TokenCacheConfig // Token cache configuration
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

// TracingConfig holds OpenTelemetry tracing configuration
type TracingConfig struct {
	Enabled     bool   // Enable tracing
	ServiceName string // Service name in traces
	Endpoint    string // OTLP collector endpoint (e.g., "localhost:4317")
	Insecure    bool   // Use insecure connection (no TLS)
}

// MetricsConfig holds Prometheus metrics endpoint configuration.
//
// /metrics exposes operational data (job counts, container lifecycle, request
// rates) and is served on a separate listener that defaults to localhost only.
// Production scrapers should run alongside Stromboli (sidecar / DaemonSet) or
// reach it through a private network — never put this on a public interface.
type MetricsConfig struct {
	Enabled bool   // Whether to expose /metrics (default: true)
	Address string // Listener address (default: "127.0.0.1:9090")
}

// ComposeConfig holds compose environment configuration
type ComposeConfig struct {
	AllowPrivileged  bool          // Allow services with privileged: true (default: false)
	AllowHostNetwork bool          // Allow services with network_mode: host (default: false)
	AllowHostVolumes bool          // Allow host volume mounts (default: false)
	BuildTimeout     time.Duration // Maximum time for compose build/up (default: 10m)
	HealthTimeout    time.Duration // Maximum time to wait for healthy services (default: 2m)
	StackTTL         time.Duration // Maximum age for orphaned stacks before cleanup (default: 1h)
}

// Default values
const (
	defaultServerAddress      = ":8080"
	defaultAgentImage         = "stromboli-agent"
	defaultAgentImageTag      = "latest"
	defaultCLIImage           = "ghcr.io/tomblancdev/stromboli-agent"
	defaultCLIImageTag        = "latest"
	defaultCredentialsFile    = "~/.claude/.credentials.json"
	defaultSessionsDir        = ".stromboli/sessions"
	defaultMemory            = "512m"
	defaultCPUs              = "1"
	defaultTimeout           = "30m"
	defaultRateLimitRate     = 10
	defaultRateLimitBurst    = 20
	defaultJWTAccessExpiry   = 24 * time.Hour
	defaultJWTRefresh        = 7 * 24 * time.Hour
	defaultCleanupTTL        = 1 * time.Hour
	defaultCleanupInterval   = 5 * time.Minute
	defaultTokenCacheTTL     = 5 * time.Minute
	defaultTracingEndpoint   = "localhost:4317"
	defaultTracingService    = "stromboli"
	defaultMetricsAddress    = "127.0.0.1:9090"

	// MinJWTSecretLength is the minimum allowed JWT secret length in bytes.
	// 32 bytes matches the output of `openssl rand -base64 24` (32 chars) and
	// gives at least 192 bits of entropy when generated from random bytes.
	MinJWTSecretLength = 32

	// Compose defaults
	defaultComposeBuildTimeout  = 10 * time.Minute
	defaultComposeHealthTimeout = 2 * time.Minute
	defaultComposeStackTTL      = 1 * time.Hour
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
	v.SetDefault("agent.image_tag", defaultAgentImageTag)
	v.SetDefault("agent.allowed_image_patterns", []string{})
	v.SetDefault("agent.allowed_volumes", []string{})
	v.SetDefault("agent.allow_all_volumes", false) // SECURE DEFAULT: deny all when no allowlist
	v.SetDefault("agent.workdir_auto_create", true)
	v.SetDefault("agent.volume_auto_create", true)
	v.SetDefault("agent.mount_claude_cli", false)
	v.SetDefault("agent.cli_image", defaultCLIImage)
	v.SetDefault("agent.cli_image_tag", defaultCLIImageTag)
	v.SetDefault("agent.auto_pull_cli", true)
	v.SetDefault("agent.credentials_file", defaultCredentialsFile)
	v.SetDefault("agent.sessions_dir", defaultSessionsDir)
	v.SetDefault("agent.sessions_host_dir", "") // Empty means same as sessions_dir
	v.SetDefault("agent.token_cache.enabled", true)
	v.SetDefault("agent.token_cache.ttl", defaultTokenCacheTTL.String())
	v.SetDefault("resources.memory", defaultMemory)
	v.SetDefault("resources.cpus", defaultCPUs)
	v.SetDefault("resources.timeout", defaultTimeout)
	// Auth is on by default — secure-by-default.
	// Operators must provide STROMBOLI_JWT_SECRET; Validate() refuses to start otherwise.
	// Local dev that wants to skip auth must set STROMBOLI_AUTH_ENABLED=false explicitly.
	v.SetDefault("auth.enabled", true)
	v.SetDefault("auth.valid_tokens", []string{})
	v.SetDefault("rate_limit.enabled", false)
	v.SetDefault("rate_limit.rate", defaultRateLimitRate)
	v.SetDefault("rate_limit.burst", defaultRateLimitBurst)
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.access_expiry", defaultJWTAccessExpiry.String())
	v.SetDefault("jwt.refresh_expiry", defaultJWTRefresh.String())
	v.SetDefault("jobs.cleanup_ttl", defaultCleanupTTL.String())
	v.SetDefault("jobs.cleanup_interval", defaultCleanupInterval.String())
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.service_name", defaultTracingService)
	v.SetDefault("tracing.endpoint", defaultTracingEndpoint)
	// Tracing transport is TLS by default — plaintext gRPC leaks request
	// metadata and session IDs. Operators with a localhost OTLP collector
	// or trusted private network can opt back in via STROMBOLI_TRACING_INSECURE=true.
	v.SetDefault("tracing.insecure", false)

	// Compose defaults (secure by default)
	v.SetDefault("compose.allow_privileged", false)
	v.SetDefault("compose.allow_host_network", false)
	v.SetDefault("compose.allow_host_volumes", false)
	v.SetDefault("compose.build_timeout", defaultComposeBuildTimeout.String())
	v.SetDefault("compose.health_timeout", defaultComposeHealthTimeout.String())
	v.SetDefault("compose.stack_ttl", defaultComposeStackTTL.String())

	// Metrics endpoint defaults — bind to localhost so /metrics doesn't leak
	// without an explicit operator opt-in. Set metrics.address to ":9090" to
	// expose on all interfaces.
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.address", defaultMetricsAddress)

	// Environment variable configuration
	v.SetEnvPrefix("STROMBOLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Map legacy environment variables for backward compatibility
	// BindEnv errors are ignored as these are static configuration that cannot fail at runtime
	_ = v.BindEnv("server.address", "STROMBOLI_SERVER_ADDRESS")
	_ = v.BindEnv("agent.image", "STROMBOLI_AGENT_IMAGE")
	_ = v.BindEnv("agent.image_tag", "STROMBOLI_AGENT_IMAGE_TAG")
	_ = v.BindEnv("agent.allowed_image_patterns", "STROMBOLI_AGENT_ALLOWED_IMAGE_PATTERNS")
	_ = v.BindEnv("agent.allowed_volumes", "STROMBOLI_AGENT_ALLOWED_VOLUMES")
	_ = v.BindEnv("agent.allow_all_volumes", "STROMBOLI_AGENT_ALLOW_ALL_VOLUMES")
	_ = v.BindEnv("agent.workdir_auto_create", "STROMBOLI_AGENT_WORKDIR_AUTO_CREATE")
	_ = v.BindEnv("agent.volume_auto_create", "STROMBOLI_AGENT_VOLUME_AUTO_CREATE")
	_ = v.BindEnv("agent.mount_claude_cli", "STROMBOLI_AGENT_MOUNT_CLAUDE_CLI")
	_ = v.BindEnv("agent.cli_image", "STROMBOLI_AGENT_CLI_IMAGE")
	_ = v.BindEnv("agent.cli_image_tag", "STROMBOLI_AGENT_CLI_IMAGE_TAG")
	_ = v.BindEnv("agent.auto_pull_cli", "STROMBOLI_AGENT_AUTO_PULL_CLI")
	_ = v.BindEnv("agent.credentials_file", "STROMBOLI_AGENT_CREDENTIALS_FILE")
	_ = v.BindEnv("agent.sessions_dir", "STROMBOLI_AGENT_SESSIONS_DIR")
	_ = v.BindEnv("agent.sessions_host_dir", "STROMBOLI_AGENT_SESSIONS_HOST_DIR")
	_ = v.BindEnv("agent.token_cache.enabled", "STROMBOLI_TOKEN_CACHE_ENABLED")
	_ = v.BindEnv("agent.token_cache.ttl", "STROMBOLI_TOKEN_CACHE_TTL")
	_ = v.BindEnv("resources.memory", "STROMBOLI_DEFAULT_MEMORY")
	_ = v.BindEnv("resources.cpus", "STROMBOLI_DEFAULT_CPUS")
	_ = v.BindEnv("resources.timeout", "STROMBOLI_DEFAULT_TIMEOUT")
	_ = v.BindEnv("auth.enabled", "STROMBOLI_AUTH_ENABLED")
	_ = v.BindEnv("auth.valid_tokens", "STROMBOLI_API_TOKENS")
	_ = v.BindEnv("rate_limit.enabled", "STROMBOLI_RATE_LIMIT_ENABLED")
	_ = v.BindEnv("rate_limit.rate", "STROMBOLI_RATE_LIMIT_RPS")
	_ = v.BindEnv("rate_limit.burst", "STROMBOLI_RATE_LIMIT_BURST")
	_ = v.BindEnv("jwt.secret", "STROMBOLI_JWT_SECRET")
	_ = v.BindEnv("jwt.access_expiry", "STROMBOLI_JWT_EXPIRY")
	_ = v.BindEnv("jwt.refresh_expiry", "STROMBOLI_JWT_REFRESH_EXPIRY")
	_ = v.BindEnv("jobs.cleanup_ttl", "STROMBOLI_JOBS_CLEANUP_TTL")
	_ = v.BindEnv("jobs.cleanup_interval", "STROMBOLI_JOBS_CLEANUP_INTERVAL")
	_ = v.BindEnv("tracing.enabled", "STROMBOLI_TRACING_ENABLED")
	_ = v.BindEnv("tracing.service_name", "STROMBOLI_TRACING_SERVICE_NAME")
	_ = v.BindEnv("tracing.endpoint", "STROMBOLI_TRACING_ENDPOINT")
	_ = v.BindEnv("tracing.insecure", "STROMBOLI_TRACING_INSECURE")

	// Compose environment variables
	_ = v.BindEnv("compose.allow_privileged", "STROMBOLI_COMPOSE_ALLOW_PRIVILEGED")
	_ = v.BindEnv("compose.allow_host_network", "STROMBOLI_COMPOSE_ALLOW_HOST_NETWORK")
	_ = v.BindEnv("compose.allow_host_volumes", "STROMBOLI_COMPOSE_ALLOW_HOST_VOLUMES")
	_ = v.BindEnv("compose.build_timeout", "STROMBOLI_COMPOSE_BUILD_TIMEOUT")
	_ = v.BindEnv("compose.health_timeout", "STROMBOLI_COMPOSE_HEALTH_TIMEOUT")
	_ = v.BindEnv("compose.stack_ttl", "STROMBOLI_COMPOSE_STACK_TTL")

	// Metrics environment variables
	_ = v.BindEnv("metrics.enabled", "STROMBOLI_METRICS_ENABLED")
	_ = v.BindEnv("metrics.address", "STROMBOLI_METRICS_ADDRESS")
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
			Image:                v.GetString("agent.image"),
			ImageTag:             v.GetString("agent.image_tag"),
			AllowedImagePatterns: getStringSliceOrSplit(v, "agent.allowed_image_patterns"),
			AllowedVolumes:       getStringSliceOrSplit(v, "agent.allowed_volumes"),
			AllowAllVolumes:      v.GetBool("agent.allow_all_volumes"),
			WorkdirAutoCreate:    v.GetBool("agent.workdir_auto_create"),
			VolumeAutoCreate:     v.GetBool("agent.volume_auto_create"),
			MountClaudeCLI:       v.GetBool("agent.mount_claude_cli"),
			CLIImage:             v.GetString("agent.cli_image"),
			CLIImageTag:          v.GetString("agent.cli_image_tag"),
			AutoPullCLI:          v.GetBool("agent.auto_pull_cli"),
			CredentialsFile:      v.GetString("agent.credentials_file"),
			SessionsDir:          v.GetString("agent.sessions_dir"),
			SessionsHostDir:      getSessionsHostDir(v),
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
			ValidTokens: getValidTokens(v),
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

	// Parse tracing config
	cfg.Tracing = TracingConfig{
		Enabled:     v.GetBool("tracing.enabled"),
		ServiceName: v.GetString("tracing.service_name"),
		Endpoint:    v.GetString("tracing.endpoint"),
		Insecure:    v.GetBool("tracing.insecure"),
	}

	// Parse compose config
	composeBuildTimeout, err := parseDuration(v.GetString("compose.build_timeout"))
	if err != nil {
		return nil, fmt.Errorf("invalid compose build timeout: %w", err)
	}
	composeHealthTimeout, err := parseDuration(v.GetString("compose.health_timeout"))
	if err != nil {
		return nil, fmt.Errorf("invalid compose health timeout: %w", err)
	}
	composeStackTTL, err := parseDuration(v.GetString("compose.stack_ttl"))
	if err != nil {
		return nil, fmt.Errorf("invalid compose stack TTL: %w", err)
	}

	cfg.Compose = ComposeConfig{
		AllowPrivileged:  v.GetBool("compose.allow_privileged"),
		AllowHostNetwork: v.GetBool("compose.allow_host_network"),
		AllowHostVolumes: v.GetBool("compose.allow_host_volumes"),
		BuildTimeout:     composeBuildTimeout,
		HealthTimeout:    composeHealthTimeout,
		StackTTL:         composeStackTTL,
	}

	cfg.Metrics = MetricsConfig{
		Enabled: v.GetBool("metrics.enabled"),
		Address: v.GetString("metrics.address"),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// getValidTokens retrieves valid tokens from viper, handling both
// comma-separated strings (from env vars) and arrays (from YAML config)
func getValidTokens(v *viper.Viper) []string {
	// Try to get as string slice (from YAML arrays)
	tokens := v.GetStringSlice("auth.valid_tokens")

	// If we got a single element that contains commas, it's from an env var
	// and needs to be split. YAML arrays would have multiple elements.
	if len(tokens) == 1 && strings.Contains(tokens[0], ",") {
		tokens = strings.Split(tokens[0], ",")
	}

	// If empty, return empty slice
	if len(tokens) == 0 || (len(tokens) == 1 && tokens[0] == "") {
		return []string{}
	}

	// Trim whitespace from each token
	for i := range tokens {
		tokens[i] = strings.TrimSpace(tokens[i])
	}
	return tokens
}

// getStringSliceOrSplit gets a string slice from viper, handling comma-separated env vars
func getStringSliceOrSplit(v *viper.Viper, key string) []string {
	values := v.GetStringSlice(key)

	// If we got a single element that contains commas, it's from an env var
	if len(values) == 1 && strings.Contains(values[0], ",") {
		values = strings.Split(values[0], ",")
	}

	// If empty, return empty slice
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		return []string{}
	}

	// Trim whitespace
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}

// parseDuration parses a duration string, handling both Go durations and integers
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// getSessionsHostDir returns the sessions host dir, falling back to sessions_dir if not set
func getSessionsHostDir(v *viper.Viper) string {
	hostDir := v.GetString("agent.sessions_host_dir")
	if hostDir != "" {
		return hostDir
	}
	return v.GetString("agent.sessions_dir")
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

	// When auth is enabled, the JWT secret must be present, long enough, and not a
	// known placeholder. This is the secure-by-default gate: a misconfigured prod
	// deploy fails to start instead of silently exposing an unauthenticated API.
	if c.Auth.Enabled {
		if err := validateJWTSecret(c.JWT.Secret); err != nil {
			return err
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

	// Validate tracing config
	if c.Tracing.Enabled && c.Tracing.Endpoint == "" {
		return fmt.Errorf("tracing endpoint must be set when tracing is enabled")
	}

	// Validate compose config
	if c.Compose.BuildTimeout <= 0 {
		return fmt.Errorf("compose build timeout must be positive")
	}
	if c.Compose.HealthTimeout <= 0 {
		return fmt.Errorf("compose health timeout must be positive")
	}
	if c.Compose.StackTTL <= 0 {
		return fmt.Errorf("compose stack TTL must be positive")
	}

	return nil
}

// jwtPlaceholderSecrets are well-known placeholder strings shipped in templates
// and documentation. Treating them as invalid prevents an operator from
// accidentally promoting a copy-pasted example to production.
var jwtPlaceholderSecrets = map[string]struct{}{
	"change-me":                              {},
	"changeme":                               {},
	"replace-me":                             {},
	"your-secret-here":                       {},
	"your-jwt-secret":                        {},
	"generate-with-openssl-rand-base64-32":   {},
	"insecure-secret-do-not-use-in-prod":     {},
	"test-secret":                            {},
}

// validateJWTSecret enforces the JWT secret requirements when auth is enabled.
// Returns an actionable error directing the operator to generate a real secret.
func validateJWTSecret(secret string) error {
	const howToFix = "generate one with: openssl rand -base64 32"

	if secret == "" {
		return fmt.Errorf("auth is enabled but STROMBOLI_JWT_SECRET is empty — %s", howToFix)
	}
	if len(secret) < MinJWTSecretLength {
		return fmt.Errorf("auth is enabled but STROMBOLI_JWT_SECRET is too short (%d chars, need at least %d) — %s",
			len(secret), MinJWTSecretLength, howToFix)
	}
	if _, isPlaceholder := jwtPlaceholderSecrets[strings.ToLower(strings.TrimSpace(secret))]; isPlaceholder {
		return fmt.Errorf("auth is enabled but STROMBOLI_JWT_SECRET is a known placeholder value — %s", howToFix)
	}
	return nil
}
