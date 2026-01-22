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
// @contact.url https://github.com/tomblanc/stromboli/issues
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tomblanc/stromboli/internal/api"
	"github.com/tomblanc/stromboli/internal/auth"
	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/job"
	"github.com/tomblanc/stromboli/internal/runner"
)

const (
	defaultImage       = "stromboli-agent:latest"
	defaultSecretsFile = ".claude-secrets"
	defaultSessionsDir = ".stromboli/sessions"
	defaultAddr        = ":8080"
)

// getAuthConfig loads authentication configuration from environment variables.
// Auth is disabled by default for backward compatibility.
func getAuthConfig() auth.Config {
	enabled := os.Getenv("STROMBOLI_AUTH_ENABLED") == "true"
	tokensEnv := os.Getenv("STROMBOLI_API_TOKENS")

	var tokens []string
	if tokensEnv != "" {
		tokens = strings.Split(tokensEnv, ",")
	}

	return auth.Config{
		Enabled:     enabled,
		ValidTokens: tokens,
	}
}

// getRateLimitConfig loads rate limiting configuration from environment variables.
// Rate limiting is disabled by default.
func getRateLimitConfig() api.RateLimitConfig {
	enabled := os.Getenv("STROMBOLI_RATE_LIMIT_ENABLED") == "true"

	rate := 10 // default: 10 requests per second
	if rateStr := os.Getenv("STROMBOLI_RATE_LIMIT_RPS"); rateStr != "" {
		if r, err := strconv.Atoi(rateStr); err == nil && r > 0 {
			rate = r
		}
	}

	burst := 20 // default: burst of 20
	if burstStr := os.Getenv("STROMBOLI_RATE_LIMIT_BURST"); burstStr != "" {
		if b, err := strconv.Atoi(burstStr); err == nil && b > 0 {
			burst = b
		}
	}

	return api.RateLimitConfig{
		Enabled: enabled,
		Rate:    rate,
		Period:  time.Second,
		Burst:   burst,
	}
}

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

	// Create dependencies
	claudeClient := claude.NewClient(defaultSecretsFile)
	podmanRunner, err := runner.NewPodmanRunner(defaultImage, defaultSecretsFile, defaultSessionsDir, allowedWorkspaces)
	if err != nil {
		slog.Error("Failed to create runner", "error", err)
		os.Exit(1)
	}
	authConfig := getAuthConfig()

	if authConfig.Enabled {
		slog.Info("Authentication enabled", "tokens", len(authConfig.ValidTokens))
	} else {
		slog.Info("Authentication disabled")
	}

	rateLimitConfig := getRateLimitConfig()

	if rateLimitConfig.Enabled {
		slog.Info("Rate limiting enabled", "rate", rateLimitConfig.Rate, "burst", rateLimitConfig.Burst)
	} else {
		slog.Info("Rate limiting disabled")
	}

	// Create job manager for async execution
	jobMgr := job.NewManager()

	// Start job cleanup with 1 hour TTL, 5 minute interval
	jobMgr.StartCleanup(1*time.Hour, 5*time.Minute)
	slog.Info("Job cleanup started", "ttl", "1h", "interval", "5m")

	// Setup signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create the API server
	server := api.NewServer(podmanRunner, claudeClient, authConfig, rateLimitConfig, jobMgr)

	srv := &http.Server{
		Addr:    defaultAddr,
		Handler: server.Handler(),
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Server starting", "addr", defaultAddr)
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

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server stopped")
}
