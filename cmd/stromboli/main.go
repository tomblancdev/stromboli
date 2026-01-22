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

	"github.com/tomblanc/stromboli/internal/api"
	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/runner"
)

const (
	defaultImage       = "stromboli-agent:latest"
	defaultSecretsFile = ".claude-secrets"
	defaultSessionsDir = ".stromboli/sessions"
	defaultAddr        = ":8080"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Stromboli 🌋")

	// Create dependencies
	claudeClient := claude.NewClient(defaultSecretsFile)
	podmanRunner := runner.NewPodmanRunner(defaultImage, defaultSecretsFile, defaultSessionsDir)

	// Start the API server
	server := api.NewServer(podmanRunner, claudeClient)
	if err := server.Run(defaultAddr); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
