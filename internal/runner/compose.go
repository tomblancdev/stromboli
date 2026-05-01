package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"stromboli/internal/claude"
	"stromboli/internal/compose"
	"stromboli/internal/metrics"
	"stromboli/internal/tracing"
)

// ComposeConfig holds configuration for compose environments
type ComposeConfig struct {
	AllowPrivileged  bool
	AllowHostNetwork bool
	AllowHostVolumes bool
	BuildTimeout     time.Duration
	HealthTimeout    time.Duration
	StackTTL         time.Duration
}

// SetComposeConfig configures compose support on the runner
func (r *PodmanRunner) SetComposeConfig(cfg ComposeConfig) {
	r.composeMgr = compose.NewManager(r.executor, compose.Config{
		AllowPrivileged:  cfg.AllowPrivileged,
		AllowHostNetwork: cfg.AllowHostNetwork,
		AllowHostVolumes: cfg.AllowHostVolumes,
		BuildTimeout:     cfg.BuildTimeout,
		HealthTimeout:    cfg.HealthTimeout,
		StackTTL:         cfg.StackTTL,
	})
}

// isComposeEnvironment returns true if the request uses a compose environment
func isComposeEnvironment(req Request) bool {
	return req.Podman.Environment.Type == "compose"
}

// runWithCompose executes Claude in a compose environment
func (r *PodmanRunner) runWithCompose(ctx context.Context, req Request) (*Result, error) {
	ctx, span := tracing.StartSpanWithKind(ctx, "runner.runWithCompose", tracing.SpanKindInternal)
	defer span.End()

	if r.composeMgr == nil {
		return nil, fmt.Errorf("compose support not configured")
	}

	// Add compose-specific tracing
	tracing.AddSpanAttributes(ctx,
		"runner.compose.path", req.Podman.Environment.Path,
		"runner.compose.service", req.Podman.Environment.Service,
	)

	// Apply defaults
	req.Podman = r.applyDefaults(req.Podman)

	// Create or get session
	sessionID, _, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}

	// Start or get compose stack
	env := compose.Environment{
		Type:         req.Podman.Environment.Type,
		Path:         req.Podman.Environment.Path,
		Service:      req.Podman.Environment.Service,
		BuildTimeout: req.Podman.Environment.BuildTimeout,
	}

	stack := r.composeMgr.GetStack(sessionID)
	if stack == nil || !stack.IsReady() {
		// Start the compose stack (or wait for it to become ready)
		stack, err = r.composeMgr.Up(ctx, env, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to start compose stack: %w", err)
		}
		// Defensive nil check - Up() should never return nil stack without error
		if stack == nil {
			return nil, fmt.Errorf("compose stack is nil after Up() succeeded")
		}
		tracing.AddSpanAttributes(ctx, "runner.compose.stack_started", true)
	}

	// Build Claude command
	claudeBuilder := claude.NewCommandBuilder().WithPrompt(req.Prompt)
	claudeBuilder.WithSessionID(sessionID)
	if req.Claude.Resume {
		claudeBuilder.WithResume()
	}
	claude.ApplyOptions(claudeBuilder, req.Claude)
	claudeCmd := claudeBuilder.Build()

	// Prepend "claude" if mounting CLI
	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}

	// Prepare lifecycle hooks
	needsInit, cleanup, err := r.prepareLifecycleHooks(ctx, req.Podman.Lifecycle, sessionID)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Wrap command with lifecycle hooks
	commandWithHooks := WrapCommandWithHooks(claudeCmd, req.Podman.Lifecycle, needsInit)

	// Wrap with workdir setup if needed
	finalCmd := r.wrapComposeCommandWithSetup(req.Workdir, commandWithHooks)

	// Track active container
	metrics.IncActiveContainers()
	defer metrics.DecActiveContainers()

	// Execute via compose exec
	output, err := r.composeMgr.Exec(ctx, sessionID, stack.Service, finalCmd)
	if err != nil {
		tracing.RecordError(ctx, err)

		// Check if this is a crash
		if IsCrash(err) {
			crashInfo := ExtractCrashInfo(err, string(output))
			tracing.AddSpanAttributes(ctx,
				"runner.crash", true,
				"runner.crash_signal", crashInfo.Signal,
				"runner.crash_exit_code", crashInfo.ExitCode,
			)
			tracing.SetSpanStatus(ctx, tracing.StatusError, "execution crashed")

			return &Result{
				ID:        generateRunID(),
				Output:    strings.TrimSpace(string(output)),
				SessionID: sessionID,
				CrashInfo: crashInfo,
			}, nil
		}

		tracing.SetSpanStatus(ctx, tracing.StatusError, "execution failed")
		return nil, fmt.Errorf("execution failed: %w, output: %s", err, string(output))
	}

	// Mark session as initialized if we ran init hooks successfully
	if needsInit {
		if markErr := r.sessionMgr.MarkInitialized(sessionID); markErr != nil {
			return nil, fmt.Errorf("failed to mark session as initialized: %w", markErr)
		}
	}

	result := &Result{
		ID:        generateRunID(),
		Output:    strings.TrimSpace(string(output)),
		SessionID: sessionID,
	}

	tracing.AddSpanAttributes(ctx,
		"runner.result_id", result.ID,
		"runner.output_length", len(result.Output),
		"runner.session_id", result.SessionID,
	)
	tracing.SetSpanStatus(ctx, tracing.StatusOK, "execution completed")

	return result, nil
}

// runStreamWithCompose executes Claude in a compose environment with streaming output
func (r *PodmanRunner) runStreamWithCompose(ctx context.Context, req Request, output chan<- string) (*Result, error) {
	ctx, span := tracing.StartSpanWithKind(ctx, "runner.runStreamWithCompose", tracing.SpanKindInternal)
	defer span.End()
	defer close(output)

	if r.composeMgr == nil {
		return nil, fmt.Errorf("compose support not configured")
	}

	// Add compose-specific tracing
	tracing.AddSpanAttributes(ctx,
		"runner.compose.path", req.Podman.Environment.Path,
		"runner.compose.service", req.Podman.Environment.Service,
		"runner.streaming", true,
	)

	// Apply defaults
	req.Podman = r.applyDefaults(req.Podman)

	// Create or get session
	sessionID, _, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}

	// Start or get compose stack
	env := compose.Environment{
		Type:         req.Podman.Environment.Type,
		Path:         req.Podman.Environment.Path,
		Service:      req.Podman.Environment.Service,
		BuildTimeout: req.Podman.Environment.BuildTimeout,
	}

	stack := r.composeMgr.GetStack(sessionID)
	if stack == nil || !stack.IsReady() {
		stack, err = r.composeMgr.Up(ctx, env, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to start compose stack: %w", err)
		}
		// Defensive nil check - Up() should never return nil stack without error
		if stack == nil {
			return nil, fmt.Errorf("compose stack is nil after Up() succeeded")
		}
		tracing.AddSpanAttributes(ctx, "runner.compose.stack_started", true)
	}

	// Build Claude command
	claudeBuilder := claude.NewCommandBuilder().WithPrompt(req.Prompt)
	claudeBuilder.WithSessionID(sessionID)
	if req.Claude.Resume {
		claudeBuilder.WithResume()
	}
	claude.ApplyOptions(claudeBuilder, req.Claude)
	claudeCmd := claudeBuilder.Build()

	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}

	// Prepare lifecycle hooks
	needsInit, cleanup, err := r.prepareLifecycleHooks(ctx, req.Podman.Lifecycle, sessionID)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Wrap command with lifecycle hooks
	commandWithHooks := WrapCommandWithHooks(claudeCmd, req.Podman.Lifecycle, needsInit)

	// Wrap with workdir setup if needed
	finalCmd := r.wrapComposeCommandWithSetup(req.Workdir, commandWithHooks)

	// Track active container
	metrics.IncActiveContainers()
	defer metrics.DecActiveContainers()

	// Execute via compose exec with streaming
	stdoutPipe, stderrPipe, start, wait, err := r.composeMgr.ExecStream(ctx, sessionID, stack.Service, finalCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to setup streaming: %w", err)
	}

	if err := start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Collect all output for final result
	var allOutput strings.Builder
	done := make(chan struct{})

	// Stream stdout and stderr concurrently
	streamPipe := func(pipe io.ReadCloser) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			allOutput.WriteString(line)
			allOutput.WriteString("\n")

			select {
			case output <- line:
			case <-ctx.Done():
				return
			}
		}
	}

	go func() {
		streamPipe(stdoutPipe)
		done <- struct{}{}
	}()

	go func() {
		streamPipe(stderrPipe)
		done <- struct{}{}
	}()

	<-done
	<-done

	cmdErr := wait()

	if cmdErr != nil {
		tracing.RecordError(ctx, cmdErr)

		if IsCrash(cmdErr) {
			crashInfo := ExtractCrashInfo(cmdErr, allOutput.String())
			tracing.AddSpanAttributes(ctx,
				"runner.crash", true,
				"runner.crash_signal", crashInfo.Signal,
				"runner.crash_exit_code", crashInfo.ExitCode,
			)
			tracing.SetSpanStatus(ctx, tracing.StatusError, "execution crashed")

			return &Result{
				ID:        generateRunID(),
				Output:    strings.TrimSpace(allOutput.String()),
				SessionID: sessionID,
				CrashInfo: crashInfo,
			}, nil
		}

		tracing.SetSpanStatus(ctx, tracing.StatusError, "execution failed")
		return nil, fmt.Errorf("execution failed: %w", cmdErr)
	}

	if needsInit {
		if markErr := r.sessionMgr.MarkInitialized(sessionID); markErr != nil {
			return nil, fmt.Errorf("failed to mark session as initialized: %w", markErr)
		}
	}

	result := &Result{
		ID:        generateRunID(),
		Output:    strings.TrimSpace(allOutput.String()),
		SessionID: sessionID,
	}

	tracing.AddSpanAttributes(ctx,
		"runner.result_id", result.ID,
		"runner.output_length", len(result.Output),
		"runner.session_id", result.SessionID,
	)
	tracing.SetSpanStatus(ctx, tracing.StatusOK, "streaming completed")

	return result, nil
}

// wrapComposeCommandWithSetup wraps command for compose execution
func (r *PodmanRunner) wrapComposeCommandWithSetup(workdir string, cmd []string) []string {
	// For compose, we may need to set up workdir and HOME
	// The command is executed via "podman compose exec", so we wrap in sh -c
	var parts []string

	if workdir != "" && r.workdirAutoCreate {
		// Create workdir if it doesn't exist
		parts = append(parts, fmt.Sprintf("mkdir -p %s", EscapeShellArg(workdir)))
		// Change to workdir
		parts = append(parts, fmt.Sprintf("cd %s", EscapeShellArg(workdir)))
	} else if workdir != "" {
		parts = append(parts, fmt.Sprintf("cd %s", EscapeShellArg(workdir)))
	}

	// Build the main command
	var escapedArgs []string
	for _, arg := range cmd {
		escapedArgs = append(escapedArgs, EscapeShellArg(arg))
	}
	parts = append(parts, strings.Join(escapedArgs, " "))

	if len(parts) == 1 {
		// No setup needed, return command as-is
		return cmd
	}

	// Wrap in sh -c
	shellCmd := strings.Join(parts, " && ")
	return []string{"sh", "-c", shellCmd}
}

// destroyComposeSession cleans up compose stack for a session
func (r *PodmanRunner) destroyComposeSession(sessionID string) error {
	if r.composeMgr != nil {
		return r.composeMgr.Down(context.Background(), sessionID)
	}
	return nil
}

// CleanupComposeStacks removes orphaned compose stacks older than maxAge
func (r *PodmanRunner) CleanupComposeStacks(ctx context.Context, maxAge time.Duration) error {
	if r.composeMgr == nil {
		return nil
	}
	return r.composeMgr.CleanupOrphanedStacks(ctx, maxAge)
}

// ComposeManager returns the compose manager if configured
func (r *PodmanRunner) ComposeManager() *compose.Manager {
	return r.composeMgr
}

// IsComposeActive returns true if a compose stack is active for the session
func (r *PodmanRunner) IsComposeActive(sessionID string) bool {
	if r.composeMgr == nil {
		return false
	}
	return r.composeMgr.IsActive(sessionID)
}
