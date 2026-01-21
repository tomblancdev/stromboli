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
	podmanRunner := runner.NewPodmanRunner(defaultImage, defaultSecretsFile)

	// Start the API server
	server := api.NewServer(podmanRunner, claudeClient)
	if err := server.Run(defaultAddr); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
