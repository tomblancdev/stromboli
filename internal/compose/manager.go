package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Executor executes shell commands (compatible with runner.Executor)
type Executor interface {
	// Run executes a command and returns the combined output
	Run(ctx context.Context, args []string) ([]byte, error)

	// RunStream executes a command and returns pipes for streaming output
	RunStream(ctx context.Context, args []string) (stdout io.ReadCloser, stderr io.ReadCloser, start func() error, wait func() error, err error)
}

// Manager orchestrates compose stack lifecycle
type Manager struct {
	executor      Executor
	validator     *FileValidator
	healthChecker *HealthChecker
	config        Config
	logger        *slog.Logger

	// Track active stacks for cleanup
	mu     sync.RWMutex
	stacks map[string]*Stack // sessionID -> Stack
}

// NewManager creates a new compose Manager
func NewManager(executor Executor, cfg Config) *Manager {
	return &Manager{
		executor:      executor,
		validator:     NewFileValidator(cfg),
		healthChecker: NewHealthChecker(executor),
		config:        cfg,
		logger:        slog.Default().With("component", "compose"),
		stacks:        make(map[string]*Stack),
	}
}

// Up starts a compose stack for the given session.
// It validates the compose file, runs podman compose up, and waits for services to be healthy.
func (m *Manager) Up(ctx context.Context, env Environment, sessionID string) (*Stack, error) {
	m.logger.Info("starting compose stack",
		"session_id", sessionID,
		"compose_path", env.Path,
		"service", env.Service,
	)

	// Validate the compose file
	if err := m.validator.ValidateWithService(env.Path, env.Service); err != nil {
		return nil, fmt.Errorf("compose file validation failed: %w", err)
	}

	projectName := ProjectName(sessionID)

	// Determine build timeout
	buildTimeout := m.config.BuildTimeout
	if env.BuildTimeout != "" {
		parsed, err := time.ParseDuration(env.BuildTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid build timeout %q: %w", env.BuildTimeout, err)
		}
		buildTimeout = parsed
	}

	// Create context with build timeout
	buildCtx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()

	// Build and start the stack
	cmd := NewCommandBuilder().
		WithFile(env.Path).
		WithProject(projectName).
		Up().
		Detached().
		WithBuild().
		Build()

	if _, err := m.executor.Run(buildCtx, cmd); err != nil {
		m.logger.Error("compose up failed",
			"session_id", sessionID,
			"error", err,
		)

		// Attempt cleanup of any partial resources created by compose up
		// Use timeout that scales with build timeout for adequate cleanup time
		cleanupTimeout := buildTimeout / 2
		if cleanupTimeout < 1*time.Minute {
			cleanupTimeout = 1 * time.Minute
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		if cleanupErr := m.cleanupProject(cleanupCtx, projectName); cleanupErr != nil {
			m.logger.Warn("cleanup failed after failed compose up",
				"session_id", sessionID,
				"cleanup_error", cleanupErr,
			)
		}
		cleanupCancel()
		return nil, fmt.Errorf("failed to start compose stack: %w", err)
	}

	// Create stack object but DON'T track yet - only track after health check passes
	// This prevents concurrent operations from seeing unhealthy stacks
	stack := &Stack{
		ProjectName: projectName,
		ComposePath: env.Path,
		Service:     env.Service,
		SessionID:   sessionID,
		StartedAt:   time.Now(),
		State:       StackStateStarting,
	}

	// Wait for services to be healthy BEFORE adding to tracking
	if err := m.healthChecker.WaitForHealthy(ctx, projectName, m.config.HealthTimeout); err != nil {
		m.logger.Error("health check failed",
			"session_id", sessionID,
			"error", err,
		)

		// Clean up without tracking - use cleanupProject since stack isn't tracked
		cleanupTimeout := m.config.HealthTimeout
		if cleanupTimeout < 1*time.Minute {
			cleanupTimeout = 1 * time.Minute
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
		if cleanupErr := m.cleanupProject(cleanupCtx, projectName); cleanupErr != nil {
			m.logger.Warn("cleanup failed after health check failure",
				"session_id", sessionID,
				"cleanup_error", cleanupErr,
			)
		}
		cleanupCancel()
		return nil, err
	}

	// Health check passed - NOW add to tracking as running
	// This ensures concurrent callers only see fully-healthy stacks
	stack.State = StackStateRunning
	m.mu.Lock()
	m.stacks[sessionID] = stack
	m.mu.Unlock()

	m.logger.Info("compose stack started successfully",
		"session_id", sessionID,
		"project_name", projectName,
	)

	return stack, nil
}

// Down stops and removes a compose stack.
// This operation is idempotent - it succeeds even if the stack doesn't exist.
func (m *Manager) Down(ctx context.Context, sessionID string) error {
	projectName := ProjectName(sessionID)

	m.logger.Info("stopping compose stack", "session_id", sessionID, "project_name", projectName)

	// Mark as stopping, then remove from tracking
	m.mu.Lock()
	if s, ok := m.stacks[sessionID]; ok {
		s.State = StackStateStopping
	}
	delete(m.stacks, sessionID)
	m.mu.Unlock()

	// Stop and remove the stack
	cmd := NewCommandBuilder().
		WithProject(projectName).
		Down().
		RemoveVolumes().
		Build()

	_, err := m.executor.Run(ctx, cmd)
	if err != nil {
		// Check if it's an ExitError - non-zero exit often means "not found" for down
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// For podman compose down, non-zero exit on non-existent project is OK
			// Log it but treat as success since Down should be idempotent
			m.logger.Debug("compose down returned non-zero exit",
				"session_id", sessionID,
				"exit_code", exitErr.ExitCode(),
			)
			return nil
		}
		// Also handle string-based errors from mock executors in tests
		errStr := err.Error()
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "no such") ||
			strings.Contains(errStr, "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to stop compose stack: %w", err)
	}

	m.logger.Info("compose stack stopped", "session_id", sessionID)
	return nil
}

// Exec runs a command in the specified service container.
// Returns the combined output from the command.
// The service must match the stack's configured service for security.
func (m *Manager) Exec(ctx context.Context, sessionID, service string, cmd []string) ([]byte, error) {
	// Validate service matches stack's configured service
	// This prevents accidental or malicious exec into unvalidated services
	m.mu.RLock()
	stack, ok := m.stacks[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("stack not found for session %s", sessionID)
	}

	if service != stack.Service {
		return nil, fmt.Errorf("service %q not allowed; only %q is configured for this stack", service, stack.Service)
	}

	projectName := ProjectName(sessionID)

	execCmd := NewCommandBuilder().
		WithProject(projectName).
		Exec(service, cmd...).
		Build()

	output, err := m.executor.Run(ctx, execCmd)
	if err != nil {
		return output, fmt.Errorf("compose exec failed: %w", err)
	}

	return output, nil
}

// ExecStream runs a command with streaming output.
// Returns stdout and stderr readers, start function, and wait function.
// The service must match the stack's configured service for security.
func (m *Manager) ExecStream(ctx context.Context, sessionID, service string, cmd []string) (
	stdout io.ReadCloser,
	stderr io.ReadCloser,
	start func() error,
	wait func() error,
	err error,
) {
	// Validate service matches stack's configured service
	m.mu.RLock()
	stack, ok := m.stacks[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, nil, nil, nil, fmt.Errorf("stack not found for session %s", sessionID)
	}

	if service != stack.Service {
		return nil, nil, nil, nil, fmt.Errorf("service %q not allowed; only %q is configured for this stack", service, stack.Service)
	}

	projectName := ProjectName(sessionID)

	execCmd := NewCommandBuilder().
		WithProject(projectName).
		Exec(service, cmd...).
		Build()

	return m.executor.RunStream(ctx, execCmd)
}

// GetStack returns the stack for a session, or nil if not found
func (m *Manager) GetStack(sessionID string) *Stack {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stacks[sessionID]
}

// IsActive returns true if a compose stack is active for the session
func (m *Manager) IsActive(sessionID string) bool {
	return m.GetStack(sessionID) != nil
}

// IsReady returns true if a compose stack exists and is in a usable state
func (m *Manager) IsReady(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if stack, ok := m.stacks[sessionID]; ok {
		return stack.State == StackStateRunning
	}
	return false
}

// ListStacks returns all tracked stacks
func (m *Manager) ListStacks() []*Stack {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stacks := make([]*Stack, 0, len(m.stacks))
	for _, stack := range m.stacks {
		stacks = append(stacks, stack)
	}
	return stacks
}

// CleanupOrphanedStacks removes stacks older than maxAge.
// This should be called periodically to clean up stacks that weren't properly shut down.
// Uses its own context to ensure cleanup completes even if caller cancels.
func (m *Manager) CleanupOrphanedStacks(ctx context.Context, maxAge time.Duration) error {
	m.mu.Lock()
	var toCleanup []*Stack
	now := time.Now()

	for _, stack := range m.stacks {
		// Only cleanup stacks that are running and older than maxAge
		// Skip stacks that are already starting or stopping
		if stack.State == StackStateRunning && now.Sub(stack.StartedAt) > maxAge {
			// Mark as stopping while we hold the lock to prevent other operations
			stack.State = StackStateStopping
			toCleanup = append(toCleanup, stack)
		}
	}
	m.mu.Unlock()

	if len(toCleanup) == 0 {
		return nil
	}

	// Use a cleanup-specific context independent of caller's context
	// This ensures cleanup happens even if caller cancels early
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Clean up old stacks (they're already marked as stopping)
	var errs []error
	for _, stack := range toCleanup {
		if err := m.Down(cleanupCtx, stack.SessionID); err != nil {
			errs = append(errs, fmt.Errorf("cleanup %s: %w", stack.SessionID, err))

			// Restore state to Running so it can be retried later
			m.mu.Lock()
			if s, ok := m.stacks[stack.SessionID]; ok && s.State == StackStateStopping {
				s.State = StackStateRunning
				m.logger.Warn("cleanup failed, restored stack state",
					"session_id", stack.SessionID,
					"error", err,
				)
			}
			m.mu.Unlock()
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// composeProject represents the JSON output from podman compose ls
type composeProject struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

// DiscoverOrphanedStacks looks for running stacks with the stromboli- prefix
// that aren't tracked in memory (e.g., after a restart).
func (m *Manager) DiscoverOrphanedStacks(ctx context.Context) ([]string, error) {
	// Run podman compose ls to find all projects
	cmd := []string{"podman", "compose", "ls", "--format", "json"}

	output, err := m.executor.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list compose projects: %w", err)
	}

	// Handle empty output
	if len(output) == 0 || strings.TrimSpace(string(output)) == "" {
		return nil, nil
	}

	// Parse JSON array of projects
	var projects []composeProject
	if err := json.Unmarshal(output, &projects); err != nil {
		return nil, fmt.Errorf("failed to parse compose ls output: %w", err)
	}

	var orphaned []string
	for _, proj := range projects {
		if strings.HasPrefix(proj.Name, ProjectPrefix) {
			sessionID := strings.TrimPrefix(proj.Name, ProjectPrefix)

			// Check if we're tracking this session
			if !m.IsActive(sessionID) {
				orphaned = append(orphaned, sessionID)
			}
		}
	}

	return orphaned, nil
}

// cleanupProject runs podman compose down for a project without affecting stack tracking.
// This is used for cleanup on build failures where the stack was never tracked.
func (m *Manager) cleanupProject(ctx context.Context, projectName string) error {
	cmd := NewCommandBuilder().
		WithProject(projectName).
		Down().
		RemoveVolumes().
		Build()

	_, err := m.executor.Run(ctx, cmd)
	if err != nil {
		// Check if it's an ExitError - non-zero exit often means "not found" for down
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Project may not exist, that's OK for cleanup
			return nil
		}
		// Also handle string-based errors
		errStr := err.Error()
		if strings.Contains(errStr, "not found") ||
			strings.Contains(errStr, "no such") ||
			strings.Contains(errStr, "does not exist") {
			return nil
		}
		return err
	}
	return nil
}
