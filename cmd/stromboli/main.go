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
	"log/slog"
	"os"
	"strings"

	"github.com/tomblanc/stromboli/internal/api"
	"github.com/tomblanc/stromboli/internal/auth"
	"github.com/tomblanc/stromboli/internal/claude"
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
	podmanRunner := runner.NewPodmanRunner(defaultImage, defaultSecretsFile, defaultSessionsDir, allowedWorkspaces)
	authConfig := getAuthConfig()

	if authConfig.Enabled {
		slog.Info("Authentication enabled", "tokens", len(authConfig.ValidTokens))
	} else {
		slog.Info("Authentication disabled")
	}

	// Start the API server
	server := api.NewServer(podmanRunner, claudeClient, authConfig)
	if err := server.Run(defaultAddr); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
