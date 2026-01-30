// Stromboli - Claude Code Container Orchestration API
//
// Stromboli is a simple API layer for executing Claude Code in isolated Podman containers.
// It provides secure, sandboxed execution with full Claude CLI options exposed.
//
// @title Stromboli API
// @version 1.0
// @description Claude Code container orchestration API - secure, isolated AI execution
// @termsOfService https://github.com/tomblanc/stromboli
//
// @contact.name API Support
// @contact.url https://stromboli/issues
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8080
// @BasePath /
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description OAuth2 Bearer token (future)
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stromboli/internal/api"
	"stromboli/internal/auth"
	"stromboli/internal/claude"
	"stromboli/internal/config"
	"stromboli/internal/job"
	"stromboli/internal/runner"
	"stromboli/internal/secrets"
	"stromboli/internal/tracing"
	"stromboli/internal/version"
)

// defaultHealthTimeout is the timeout for each health check component
const defaultHealthTimeout = 5 * time.Second


func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Stromboli 🌋", "version", version.String())

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Combine image and tag for container operations
	fullImage := cfg.Agent.Image + ":" + cfg.Agent.ImageTag

	slog.Info("Configuration loaded",
		"server_address", cfg.Server.Address,
		"agent_image", fullImage)

	// Ensure Claude CLI image is available (for dynamic image mounting)
	if cfg.Agent.MountClaudeCLI {
		cliImageConfig := runner.CLIImageConfig{
			Image:    cfg.Agent.CLIImage,
			Tag:      cfg.Agent.CLIImageTag,
			AutoPull: cfg.Agent.AutoPullCLI,
		}
		cliImageMgr := runner.NewCLIImageManager(cliImageConfig, runner.NewShellExecutor())

		// Set the global CLI image name for the runner
		runner.ClaudeCLIImageName = cliImageMgr.GetCLIImageName()

		if err := cliImageMgr.EnsureImage(context.Background()); err != nil {
			slog.Error("Failed to ensure CLI image", "error", err)
			os.Exit(1)
		}
	}

	// Initialize tracing
	tracingCfg := tracing.Config{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.Tracing.ServiceName,
		Endpoint:    cfg.Tracing.Endpoint,
		Insecure:    cfg.Tracing.Insecure,
	}

	shutdownTracing, err := tracing.Init(context.Background(), tracingCfg)
	if err != nil {
		slog.Error("Failed to initialize tracing", "error", err)
		os.Exit(1)
	}

	if cfg.Tracing.Enabled {
		slog.Info("Tracing enabled",
			"service_name", cfg.Tracing.ServiceName,
			"endpoint", cfg.Tracing.Endpoint)
	} else {
		slog.Info("Tracing disabled")
	}

	// Create Claude client for credentials validation
	claudeClient := claude.NewClient(cfg.Agent.CredentialsFile)

	if !claudeClient.IsConfigured() {
		slog.Warn("Claude credentials not found - run 'claude' to authenticate",
			"expected_path", claudeClient.CredentialsFile())
	} else {
		slog.Info("Claude credentials found", "path", claudeClient.CredentialsFile())
	}

	resourceDefaults := runner.ResourceDefaults{
		Memory:  cfg.Resources.Memory,
		CPUs:    cfg.Resources.CPUs,
		Timeout: cfg.Resources.Timeout,
	}

	imageConfig := runner.ImageConfig{
		AllowedPatterns: cfg.Agent.AllowedImagePatterns,
		MountClaudeCLI:  cfg.Agent.MountClaudeCLI,
	}

	volumeConfig := runner.VolumeConfig{
		AllowedVolumes:    cfg.Agent.AllowedVolumes,
		WorkdirAutoCreate: cfg.Agent.WorkdirAutoCreate,
	}

	podmanRunner, err := runner.NewPodmanRunnerFull(
		fullImage,
		cfg.Agent.CredentialsFile,
		cfg.Agent.SessionsDir,
		resourceDefaults,
		imageConfig,
		volumeConfig,
		runner.NewShellExecutor(),
	)
	if err != nil {
		slog.Error("Failed to create runner", "error", err)
		os.Exit(1)
	}

	slog.Info("Resource defaults configured",
		"memory", cfg.Resources.Memory,
		"cpus", cfg.Resources.CPUs,
		"timeout", cfg.Resources.Timeout)

	if len(cfg.Agent.AllowedImagePatterns) > 0 {
		slog.Info("Dynamic images enabled",
			"allowed_patterns", cfg.Agent.AllowedImagePatterns,
			"mount_claude_cli", cfg.Agent.MountClaudeCLI)
	}

	if len(cfg.Agent.AllowedVolumes) > 0 {
		slog.Info("Volume allowlist configured",
			"allowed_volumes", cfg.Agent.AllowedVolumes)
	}

	slog.Info("Workdir auto-creation", "enabled", cfg.Agent.WorkdirAutoCreate)

	// Cleanup orphaned containers from previous runs
	slog.Info("Cleaning up orphaned containers...")
	if err := runner.CleanupOrphanedContainers(context.Background()); err != nil {
		slog.Warn("Failed to cleanup orphaned containers", "error", err)
		// Non-fatal, continue starting server
	}

	// Build auth config for middleware
	authConfig := auth.Config{
		Enabled:     cfg.Auth.Enabled,
		ValidTokens: cfg.Auth.ValidTokens,
		JWTConfig: auth.JWTConfig{
			Secret:        cfg.JWT.Secret,
			AccessExpiry:  cfg.JWT.AccessExpiry,
			RefreshExpiry: cfg.JWT.RefreshExpiry,
		},
	}

	if authConfig.Enabled {
		slog.Info("Authentication enabled", "tokens", len(authConfig.ValidTokens))
		if cfg.JWT.Secret != "" {
			slog.Info("JWT authentication enabled",
				"access_expiry", cfg.JWT.AccessExpiry,
				"refresh_expiry", cfg.JWT.RefreshExpiry)
		}
	} else {
		slog.Info("Authentication disabled")
	}

	// Build rate limit config
	rateLimitConfig := api.RateLimitConfig{
		Enabled: cfg.RateLimit.Enabled,
		Rate:    cfg.RateLimit.Rate,
		Period:  cfg.RateLimit.Period,
		Burst:   cfg.RateLimit.Burst,
	}

	if rateLimitConfig.Enabled {
		slog.Info("Rate limiting enabled", "rate", rateLimitConfig.Rate, "burst", rateLimitConfig.Burst)
	} else {
		slog.Info("Rate limiting disabled")
	}

	// Create job manager for async execution
	jobMgr := job.NewManager()

	// Start job cleanup with configured TTL and interval
	jobMgr.StartCleanup(cfg.Jobs.CleanupTTL, cfg.Jobs.CleanupInterval)
	slog.Info("Job cleanup started",
		"ttl", cfg.Jobs.CleanupTTL,
		"interval", cfg.Jobs.CleanupInterval)

	// Create token blacklist for logout support
	blacklist := auth.NewTokenBlacklist()

	// Start blacklist cleanup (cleanup every hour - expired tokens are removed)
	blacklist.StartCleanup(time.Hour)
	slog.Info("Token blacklist started", "cleanup_interval", time.Hour)

	// Create health checker with credentials file and secret check
	healthConfig := api.HealthConfig{
		Timeout:         defaultHealthTimeout,
		CredentialsFile: claudeClient.CredentialsFile(),
		SecretName:      secrets.DefaultSecretName,
	}
	healthChecker := api.NewHealthChecker(runner.NewShellExecutor(), healthConfig)
	slog.Info("Health checker configured",
		"timeout", healthConfig.Timeout,
		"credentials_file", healthConfig.CredentialsFile,
		"secret_name", healthConfig.SecretName)

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create the API server
	server := api.NewServer(podmanRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, blacklist, cfg.Tracing.Enabled, cfg.Agent.SessionsDir)

	srv := &http.Server{
		Addr:    cfg.Server.Address,
		Handler: server.Handler(),
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Server starting", "addr", cfg.Server.Address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutdown signal received, gracefully shutting down...")

	// Stop job cleanup
	jobMgr.StopCleanup()
	slog.Info("Job cleanup stopped")

	// Stop blacklist cleanup
	blacklist.StopCleanup()
	slog.Info("Token blacklist cleanup stopped")

	// Shutdown tracing
	if err := shutdownTracing(context.Background()); err != nil {
		slog.Error("Failed to shutdown tracing", "error", err)
	}
	slog.Info("Tracing shutdown completed")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped")
}
