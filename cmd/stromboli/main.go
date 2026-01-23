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
)

// defaultHealthTimeout is the timeout for each health check component
const defaultHealthTimeout = 5 * time.Second

// allowedWorkspaces restricts which host paths can be mounted as workspaces.
// Empty slice allows all paths (backward compatible).
// In production, configure this to restrict access to specific directories.
var allowedWorkspaces = []string{}

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Stromboli 🌋")

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

	// Create dependencies
	claudeClient := claude.NewClientWithCache(
		cfg.Agent.SecretsFile,
		cfg.Agent.TokenCache.Enabled,
		cfg.Agent.TokenCache.TTL,
	)

	if cfg.Agent.TokenCache.Enabled {
		slog.Info("Token caching enabled", "ttl", cfg.Agent.TokenCache.TTL)
	} else {
		slog.Info("Token caching disabled")
	}

	resourceDefaults := runner.ResourceDefaults{
		Memory:  cfg.Resources.Memory,
		CPUs:    cfg.Resources.CPUs,
		Timeout: cfg.Resources.Timeout,
	}

	podmanRunner, err := runner.NewPodmanRunnerWithDefaults(
		fullImage,
		cfg.Agent.SecretsFile,
		cfg.Agent.SessionsDir,
		allowedWorkspaces,
		resourceDefaults,
	)
	if err != nil {
		slog.Error("Failed to create runner", "error", err)
		os.Exit(1)
	}

	slog.Info("Resource defaults configured",
		"memory", cfg.Resources.Memory,
		"cpus", cfg.Resources.CPUs,
		"timeout", cfg.Resources.Timeout)

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

	// Create health checker with default config
	healthConfig := api.HealthConfig{
		Timeout:    defaultHealthTimeout,
		SecretName: "claude-token",
	}
	healthChecker := api.NewHealthChecker(runner.NewShellExecutor(), healthConfig)
	slog.Info("Health checker configured",
		"timeout", healthConfig.Timeout,
		"secret_name", healthConfig.SecretName)

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create the API server
	server := api.NewServer(podmanRunner, claudeClient, authConfig, rateLimitConfig, jobMgr, healthChecker, blacklist)

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

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped")
}
