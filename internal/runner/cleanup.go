package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// fetchContainerLogs retrieves the last maxBytes of logs from a container.
// Used as a fallback when CombinedOutput() returns empty after a crash/timeout
// (SIGKILL on the podman process can break the stdout relay before output is captured).
func fetchContainerLogs(containerName string, maxBytes int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "logs", "--tail", "200", containerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("Failed to fetch container logs as crash output fallback", "name", containerName, "error", err)
		return ""
	}

	logs := strings.TrimSpace(string(out))
	if len(logs) > maxBytes {
		return "..." + logs[len(logs)-maxBytes:]
	}
	return logs
}

// removeContainer force-removes a container by name.
// Called via defer after execution to ensure cleanup even when --rm fails
// (e.g. after SIGKILL from timeout or OOM). Errors are logged but not returned
// because this runs in a defer and the primary result is already captured.
func removeContainer(containerName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", containerName)
	if err := cmd.Run(); err != nil {
		// Ignore "no such container" — means --rm already cleaned it up
		if !strings.Contains(err.Error(), "no such") {
			slog.Warn("Failed to remove container after execution", "name", containerName, "error", err)
		}
	}
}

// CleanupOrphanedContainers removes any stromboli-agent containers that are still running
// This should be called on server startup to clean up containers from previous runs
func CleanupOrphanedContainers(ctx context.Context) error {
	// List all containers with our prefix (running or stopped)
	cmd := exec.CommandContext(ctx, "podman", "ps", "-a", "--filter", "name="+ContainerNamePrefix, "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		// If no containers found, that's fine
		if strings.Contains(err.Error(), "no such") {
			return nil
		}
		return fmt.Errorf("runner: list orphaned containers via podman ps: %w", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(containers) == 0 || (len(containers) == 1 && containers[0] == "") {
		slog.Info("No orphaned containers found")
		return nil
	}

	slog.Info("Found orphaned containers", "count", len(containers))

	for _, container := range containers {
		if container == "" {
			continue
		}

		slog.Info("Cleaning up orphaned container", "name", container)

		// Stop the container (with timeout)
		stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		stopCmd := exec.CommandContext(stopCtx, "podman", "stop", container)
		if err := stopCmd.Run(); err != nil {
			slog.Warn("Failed to stop container, trying force remove", "name", container, "error", err)
		}
		cancel()

		// Remove the container
		rmCmd := exec.CommandContext(ctx, "podman", "rm", "-f", container)
		if err := rmCmd.Run(); err != nil {
			slog.Error("Failed to remove container", "name", container, "error", err)
		} else {
			slog.Info("Removed orphaned container", "name", container)
		}
	}

	return nil
}

// CleanupStaleContainers removes containers that have been running longer than maxAge
// This can be called periodically to prevent runaway containers
func CleanupStaleContainers(ctx context.Context, maxAge time.Duration) error {
	// List running containers with our prefix and their creation time
	cmd := exec.CommandContext(ctx, "podman", "ps", "--filter", "name="+ContainerNamePrefix, "--format", "{{.Names}}\t{{.RunningFor}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("runner: list stale containers via podman ps: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil
	}

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		containerName := parts[0]
		runningFor := parts[1]

		// Parse running time (e.g., "3 days ago", "2 hours ago")
		if isOlderThan(runningFor, maxAge) {
			slog.Warn("Container running too long, stopping", "name", containerName, "running_for", runningFor)

			stopCmd := exec.CommandContext(ctx, "podman", "stop", containerName)
			if err := stopCmd.Run(); err != nil {
				slog.Error("Failed to stop stale container", "name", containerName, "error", err)
			}
		}
	}

	return nil
}

// isOlderThan checks if the running time string indicates older than maxAge
func isOlderThan(runningFor string, maxAge time.Duration) bool {
	runningFor = strings.ToLower(runningFor)

	// Parse common formats: "3 days", "2 hours", "5 minutes", "30 seconds"
	var value int
	var unit string

	// Try to parse
	parts := strings.Fields(runningFor)
	if len(parts) < 2 {
		return false
	}

	_, err := fmt.Sscanf(parts[0], "%d", &value)
	if err != nil {
		return false
	}
	unit = parts[1]

	var duration time.Duration
	switch {
	case strings.HasPrefix(unit, "second"):
		duration = time.Duration(value) * time.Second
	case strings.HasPrefix(unit, "minute"):
		duration = time.Duration(value) * time.Minute
	case strings.HasPrefix(unit, "hour"):
		duration = time.Duration(value) * time.Hour
	case strings.HasPrefix(unit, "day"):
		duration = time.Duration(value) * 24 * time.Hour
	case strings.HasPrefix(unit, "week"):
		duration = time.Duration(value) * 7 * 24 * time.Hour
	case strings.HasPrefix(unit, "month"):
		duration = time.Duration(value) * 30 * 24 * time.Hour
	default:
		return false
	}

	return duration > maxAge
}
