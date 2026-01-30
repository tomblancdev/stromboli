package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// CLIImageConfig holds configuration for the Claude CLI image
type CLIImageConfig struct {
	Image    string // Full image name (e.g., ghcr.io/tomblancdev/stromboli-agent)
	Tag      string // Image tag (e.g., latest, claude-1.0.3)
	AutoPull bool   // Auto-pull if missing
}

// FullName returns the full image name with tag
func (c CLIImageConfig) FullName() string {
	if c.Tag == "" {
		return c.Image + ":latest"
	}
	return c.Image + ":" + c.Tag
}

// CLIImageManager manages the Claude CLI image
type CLIImageManager struct {
	config   CLIImageConfig
	executor Executor
}

// NewCLIImageManager creates a new CLI image manager
func NewCLIImageManager(config CLIImageConfig, executor Executor) *CLIImageManager {
	return &CLIImageManager{
		config:   config,
		executor: executor,
	}
}

// EnsureImage checks if the CLI image exists and pulls it if missing and auto-pull is enabled
func (m *CLIImageManager) EnsureImage(ctx context.Context) error {
	fullName := m.config.FullName()

	// Check if image exists locally
	exists, err := m.imageExists(ctx, fullName)
	if err != nil {
		return fmt.Errorf("failed to check CLI image: %w", err)
	}

	if exists {
		slog.Info("Claude CLI image found", "image", fullName)
		return nil
	}

	// Image doesn't exist
	if !m.config.AutoPull {
		slog.Warn("Claude CLI image not found and auto-pull disabled",
			"image", fullName,
			"hint", "Pull manually: podman pull "+fullName)
		return nil // Non-fatal, user might have it under a different name
	}

	// Pull the image
	slog.Info("Pulling Claude CLI image...", "image", fullName)
	if err := m.pullImage(ctx, fullName); err != nil {
		return fmt.Errorf("failed to pull CLI image: %w", err)
	}

	slog.Info("Claude CLI image pulled successfully", "image", fullName)
	return nil
}

// imageExists checks if an image exists locally
func (m *CLIImageManager) imageExists(ctx context.Context, image string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "image", "exists", image)
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means image doesn't exist (not an error)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// pullImage pulls an image from the registry
func (m *CLIImageManager) pullImage(ctx context.Context, image string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "pull", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// GetCLIImageName returns the full CLI image name to use for mounting
func (m *CLIImageManager) GetCLIImageName() string {
	return m.config.FullName()
}
