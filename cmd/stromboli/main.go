package main

import (
	"log/slog"
	"os"

	"github.com/tomblanc/stromboli/internal/api"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Stromboli 🌋")

	// Start the API server
	server := api.NewServer()
	if err := server.Run(":8080"); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
