package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"stromboli/internal/claude"
	"stromboli/internal/compose"
	strerrors "stromboli/internal/errors"
	"stromboli/internal/job"
	"stromboli/internal/metrics"
	"stromboli/internal/podman"
	"stromboli/internal/secrets"
	"stromboli/internal/session"
	"stromboli/internal/tracing"
	"stromboli/internal/types"
	"stromboli/internal/workspace"
)

// ContainerNamePrefix is used for naming stromboli agent containers
// This allows tracking and cleanup of orphaned containers
const ContainerNamePrefix = "stromboli-agent-"

// Runner executes Claude in a container
type Runner interface {
	Run(ctx context.Context, req Request) (*Result, error)
	RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error)
	RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error))
	DestroySession(sessionID string) error
	ListSessions() ([]string, error)
}

// Request contains the parameters for running Claude
type Request struct {
	Prompt    string
	Workdir string
	Claude    types.ClaudeOptions
	Podman    types.PodmanOptions
}

// Result contains the output from running Claude
type Result struct {
	ID        string
	Output    string
	SessionID string
	CrashInfo *job.CrashInfo
}

// ResourceDefaults contains default resource limits for containers
type ResourceDefaults struct {
	Memory  string
	CPUs    string
	Timeout string
}

// ImageConfig contains image-related configuration
type ImageConfig struct {
	AllowedPatterns []string // Allowed image patterns (empty = allow all)
	MountClaudeCLI  bool     // Mount claude-cli volume into containers
}

// VolumeConfig contains volume-related configuration
type VolumeConfig struct {
	AllowedVolumes    []string // Allowed host paths for volume mounts
	AllowAllVolumes   bool     // DANGEROUS: Allow all volumes when allowlist empty (dev only!)
	WorkdirAutoCreate bool     // Auto-create workdir if it doesn't exist (inside container)
	VolumeAutoCreate  bool     // Auto-create host directories for volume mounts
}

// PodmanRunner runs Claude using Podman containers
type PodmanRunner struct {
	image             string
	secretsMgr        *secrets.Manager
	sessionMgr        *session.Manager
	sessionsHostDir   string // Host path for sessions (for nested container mounts)
	volumeValidator   *workspace.Validator
	imageValidator    *ImageValidator
	mountClaudeCLI    bool
	workdirAutoCreate bool
	volumeAutoCreate  bool // Auto-create host directories for volume mounts
	defaults          ResourceDefaults
	executor          Executor
	composeMgr        *compose.Manager // Compose manager for multi-service environments
}

// NewPodmanRunner creates a new Podman-based runner with no default resource limits
// It ensures the Podman secret exists for secure token handling
func NewPodmanRunner(image, secretsFile, sessionsDir string, allowedVolumes []string) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir, allowedVolumes, ResourceDefaults{})
}

// NewPodmanRunnerWithExecutor creates a new Podman-based runner with a custom executor
// This is primarily useful for testing
func NewPodmanRunnerWithExecutor(image, secretsFile, sessionsDir string, allowedVolumes []string, executor Executor) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaultsAndExecutor(image, secretsFile, sessionsDir, allowedVolumes, ResourceDefaults{}, executor)
}

// NewPodmanRunnerWithDefaults creates a new Podman-based runner with default resource limits
// It ensures the Podman secret exists for secure token handling
func NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir string, allowedVolumes []string, defaults ResourceDefaults) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaultsAndExecutor(image, secretsFile, sessionsDir, allowedVolumes, defaults, NewShellExecutor())
}

// NewPodmanRunnerWithDefaultsAndExecutor creates a new Podman-based runner with default resource limits and custom executor
// This is the most flexible constructor, primarily useful for testing
func NewPodmanRunnerWithDefaultsAndExecutor(image, credentialsFile, sessionsDir string, allowedVolumes []string, defaults ResourceDefaults, executor Executor) (*PodmanRunner, error) {
	volumeConfig := VolumeConfig{
		AllowedVolumes:    allowedVolumes,
		WorkdirAutoCreate: true, // Default to true for backward compatibility
		VolumeAutoCreate:  true, // Default to true for backward compatibility
	}
	return NewPodmanRunnerFull(image, credentialsFile, sessionsDir, sessionsDir, defaults, ImageConfig{}, volumeConfig, executor)
}

// NewPodmanRunnerFull creates a new Podman-based runner with all configuration options
// sessionsDir is the internal path where sessions are created (inside this container)
// sessionsHostDir is the host path for mounting sessions into agent containers
func NewPodmanRunnerFull(image, credentialsFile, sessionsDir, sessionsHostDir string, defaults ResourceDefaults, imageConfig ImageConfig, volumeConfig VolumeConfig, executor Executor) (*PodmanRunner, error) {
	// Create secrets manager and ensure credentials exist + Podman secret is created
	secretsMgr := secrets.NewManagerWithPath(credentialsFile)
	if err := secretsMgr.EnsureExists(context.Background(), ""); err != nil {
		return nil, fmt.Errorf("failed to setup credentials: %w", err)
	}

	// Use compatibility-checking validator when mounting Claude CLI image
	var imageValidator *ImageValidator
	if imageConfig.MountClaudeCLI {
		imageValidator = NewImageValidatorWithCompatCheck(imageConfig.AllowedPatterns, image)
	} else {
		imageValidator = NewImageValidator(imageConfig.AllowedPatterns, image)
	}

	return &PodmanRunner{
		image:             image,
		secretsMgr:        secretsMgr,
		sessionMgr:        session.NewManager(sessionsDir),
		sessionsHostDir:   sessionsHostDir,
		volumeValidator:   workspace.NewValidatorWithAllowAll(volumeConfig.AllowedVolumes, volumeConfig.AllowAllVolumes),
		imageValidator:    imageValidator,
		mountClaudeCLI:    imageConfig.MountClaudeCLI,
		workdirAutoCreate: volumeConfig.WorkdirAutoCreate,
		volumeAutoCreate:  volumeConfig.VolumeAutoCreate,
		defaults:          defaults,
		executor:          executor,
	}, nil
}

// prepareLifecycleHooks validates hooks, acquires locks, and returns state needed for execution.
// Returns (needsInit bool, cleanup func(), error).
// Caller MUST defer cleanup() if it's not nil.
//
// Error Handling:
//   - Returns ErrInitInProgress if another process is currently initializing the session.
//     Callers can detect this with errors.Is(err, strerrors.ErrInitInProgress) and retry.
//   - If a previous initializer crashed, flock(2) automatically releases the lock,
//     allowing the next caller to acquire it and complete initialization.
func (r *PodmanRunner) prepareLifecycleHooks(
	ctx context.Context,
	hooks types.LifecycleHooks,
	sessionID string,
) (needsInit bool, cleanup func(), err error) {
	// Validate lifecycle hooks
	if err := ValidateLifecycleHooks(hooks); err != nil {
		return false, nil, fmt.Errorf("lifecycle hooks validation failed: %w", err)
	}

	// Check if this session needs init hooks with proper locking
	if HasInitHooks(hooks) {
		acquired, lockCleanup, err := r.sessionMgr.TryAcquireInitLock(sessionID)
		if err != nil {
			return false, nil, fmt.Errorf("failed to acquire init lock: %w", err)
		}
		if acquired {
			cleanup = lockCleanup
			// Double-check under lock
			if !r.sessionMgr.IsInitialized(sessionID) {
				needsInit = true
			}
		} else {
			// Lock contention - another process holds the lock
			// Check if previous holder completed or may have crashed
			if !r.sessionMgr.IsInitialized(sessionID) {
				// Previous init holder may have crashed or is still running
				// Return typed error so caller can detect and retry
				return false, nil, strerrors.InitInProgress(sessionID)
			}
			// Session is already initialized by another process, proceed normally
		}
	}

	// Add tracing attributes
	tracing.AddSpanAttributes(ctx,
		"runner.has_init_hooks", HasInitHooks(hooks),
		"runner.has_poststart_hook", len(hooks.PostStart) > 0,
		"runner.needs_init", needsInit,
	)

	return needsInit, cleanup, nil
}

// Run executes Claude in a Podman container
func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
	ctx, span := tracing.StartSpanWithKind(ctx, "runner.Run", tracing.SpanKindInternal)
	defer span.End()

	// Add request attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.prompt_length", len(req.Prompt),
		"runner.workdir", req.Workdir,
		"runner.model", req.Claude.Model,
		"runner.session_id", req.Claude.SessionID,
	)

	// Check if this is a compose environment request
	if isComposeEnvironment(req) {
		tracing.AddSpanAttributes(ctx, "runner.environment_type", "compose")
		return r.runWithCompose(ctx, req)
	}

	// Sync credentials if the file has changed (handles local token refresh)
	synced, err := r.secretsMgr.SyncIfChanged(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync credentials: %w", err)
	}
	if synced {
		tracing.AddSpanAttributes(ctx, "runner.credentials_synced", true)
	}

	// Apply defaults to request if not specified
	req.Podman = r.applyDefaults(req.Podman)

	// Apply timeout if specified
	if req.Podman.Timeout != "" {
		timeout, err := time.ParseDuration(req.Podman.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration: %w", err)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Validate and resolve container image
	if err := r.imageValidator.Validate(req.Podman.Image); err != nil {
		return nil, fmt.Errorf("image validation failed: %w", err)
	}
	containerImage := r.imageValidator.Resolve(req.Podman.Image)

	// Create or get session directory (creates on internal path)
	sessionID, _, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}
	// Get the host path for mounting into agent container
	hostSessionPath := r.getHostSessionPath(sessionID)

	// Build Claude command with all options
	claudeBuilder := claude.NewCommandBuilder().WithPrompt(req.Prompt)

	// Session handling:
	// - Always set --session-id so Claude uses our UUID
	// - Add --resume only when explicitly resuming
	claudeBuilder.WithSessionID(sessionID)
	if req.Claude.Resume {
		claudeBuilder.WithResume()
	}

	// Apply all other Claude options
	r.applyClaudeOptions(claudeBuilder, req.Claude)

	claudeCmd := claudeBuilder.Build()

	// Build Podman command
	// Use --userns=keep-id with --user to run as host user, preserving file ownership
	// Set HOME to mounted session directory so Claude writes there (not container's /home/claude)
	currentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	containerName := ContainerNamePrefix + sessionID
	podmanBuilder := podman.NewCommand().
		WithName(containerName).
		WithKeepID().
		WithUser(currentUser).
		WithEnv("HOME", "/home/user").
		WithImage(containerImage)

	// Mount Claude CLI from image if configured (for dynamic images)
	// The CLI image is mounted at /opt/claude, we add it to PATH
	if r.mountClaudeCLI {
		podmanBuilder.WithMountImage(ClaudeCLIImageName, ClaudeCLIMountPath)
		podmanBuilder.WithEnv("PATH", ClaudeCLIMountPath+"/usr/local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	// SESSION ISOLATION: Mount session-specific directory as user's home
	// Each session gets its own persistent storage at /home/user
	// HOME env points here so Claude writes to host-owned directory
	// Sessions are isolated from each other
	// Data persists until explicitly destroyed
	// NOTE: We use hostSessionPath because Podman runs on host, not inside this container
	podmanBuilder.WithVolume(hostSessionPath, "/home/user")

	// Mount credentials via Podman secret (more secure than volume mount)
	// Secret is mounted at ~/.claude/.credentials.json so Claude finds it automatically
	podmanBuilder.WithSecretTarget(r.secretsMgr.SecretName(), "/home/user/.claude/.credentials.json")

	// Apply request configuration (workdir, volumes, secrets, resource limits)
	if err := r.applyRequestConfig(req, podmanBuilder); err != nil {
		return nil, err
	}

	// When mounting Claude CLI image, prepend "claude" to command
	// (dynamic images don't have an entrypoint that provides it)
	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}

	// Prepare lifecycle hooks (validate, acquire locks, determine if init needed)
	needsInit, cleanup, err := r.prepareLifecycleHooks(ctx, req.Podman.Lifecycle, sessionID)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Wrap command with lifecycle hooks if any are configured
	// Init hooks (onCreateCommand, postCreate) only run on first session use
	// PostStart runs every time
	commandWithHooks := WrapCommandWithHooks(claudeCmd, req.Podman.Lifecycle, needsInit)

	// Wrap command to create workdir if it doesn't exist
	finalCmd := wrapCommandWithWorkdirSetup(req.Workdir, commandWithHooks, r.workdirAutoCreate)
	podmanBuilder.WithCommand(finalCmd)
	fullCmd := podmanBuilder.Build()

	// Track active container
	metrics.IncActiveContainers()
	defer metrics.DecActiveContainers()

	// Execute command using executor
	output, err := r.executor.Run(ctx, fullCmd)
	if err != nil {
		tracing.RecordError(ctx, err)

		// Check if this is a crash (signal termination)
		if IsCrash(err) {
			crashInfo := ExtractCrashInfo(err, string(output))
			tracing.AddSpanAttributes(ctx,
				"runner.crash", true,
				"runner.crash_signal", crashInfo.Signal,
				"runner.crash_exit_code", crashInfo.ExitCode,
				"runner.task_completed", crashInfo.TaskCompleted,
			)
			tracing.SetSpanStatus(ctx, tracing.StatusError, "execution crashed")

			// Return result with crash info instead of error
			// This allows the caller to handle crashes differently
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
	// Note: Lock cleanup is handled by defer, so we don't need to release manually
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

	// Add result attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.result_id", result.ID,
		"runner.output_length", len(result.Output),
		"runner.session_id", result.SessionID,
	)
	tracing.SetSpanStatus(ctx, tracing.StatusOK, "execution completed")

	return result, nil
}

// RunStream executes Claude in a Podman container and streams output in real-time
func (r *PodmanRunner) RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error) {
	ctx, span := tracing.StartSpanWithKind(ctx, "runner.RunStream", tracing.SpanKindInternal)
	defer span.End()

	// Add request attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.prompt_length", len(req.Prompt),
		"runner.workdir", req.Workdir,
		"runner.model", req.Claude.Model,
		"runner.session_id", req.Claude.SessionID,
		"runner.streaming", true,
	)

	// Check if this is a compose environment request
	if isComposeEnvironment(req) {
		tracing.AddSpanAttributes(ctx, "runner.environment_type", "compose")
		return r.runStreamWithCompose(ctx, req, output)
	}

	// Non-compose path: close output channel when done
	defer close(output)

	// Sync credentials if the file has changed (handles local token refresh)
	synced, err := r.secretsMgr.SyncIfChanged(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync credentials: %w", err)
	}
	if synced {
		tracing.AddSpanAttributes(ctx, "runner.credentials_synced", true)
	}

	// Apply defaults to request if not specified
	req.Podman = r.applyDefaults(req.Podman)

	// Apply timeout if specified
	if req.Podman.Timeout != "" {
		timeout, err := time.ParseDuration(req.Podman.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration: %w", err)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Validate and resolve container image
	if err := r.imageValidator.Validate(req.Podman.Image); err != nil {
		return nil, fmt.Errorf("image validation failed: %w", err)
	}
	containerImage := r.imageValidator.Resolve(req.Podman.Image)

	// Create or get session directory (creates on internal path)
	sessionID, _, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}
	// Get the host path for mounting into agent container
	hostSessionPath := r.getHostSessionPath(sessionID)

	// Build Claude command with all options
	claudeBuilder := claude.NewCommandBuilder().WithPrompt(req.Prompt)

	// Session handling
	claudeBuilder.WithSessionID(sessionID)
	if req.Claude.Resume {
		claudeBuilder.WithResume()
	}

	// Apply all other Claude options
	r.applyClaudeOptions(claudeBuilder, req.Claude)

	claudeCmd := claudeBuilder.Build()

	// Build Podman command
	// Use --userns=keep-id with --user to run as host user, preserving file ownership
	currentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	containerName := ContainerNamePrefix + sessionID
	podmanBuilder := podman.NewCommand().
		WithName(containerName).
		WithKeepID().
		WithUser(currentUser).
		WithEnv("HOME", "/home/user").
		WithImage(containerImage)

	// Mount Claude CLI from image if configured (for dynamic images)
	// The CLI image is mounted at /opt/claude, we add it to PATH
	if r.mountClaudeCLI {
		podmanBuilder.WithMountImage(ClaudeCLIImageName, ClaudeCLIMountPath)
		podmanBuilder.WithEnv("PATH", ClaudeCLIMountPath+"/usr/local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	}

	// Mount session-specific directory as user's home
	// NOTE: We use hostSessionPath because Podman runs on host, not inside this container
	podmanBuilder.WithVolume(hostSessionPath, "/home/user")

	// Mount credentials via Podman secret (more secure than volume mount)
	podmanBuilder.WithSecretTarget(r.secretsMgr.SecretName(), "/home/user/.claude/.credentials.json")

	// Apply request configuration (workdir, volumes, secrets, resource limits)
	if err := r.applyRequestConfig(req, podmanBuilder); err != nil {
		return nil, err
	}

	// When mounting Claude CLI image, prepend "claude" to command
	// (dynamic images don't have an entrypoint that provides it)
	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}

	// Prepare lifecycle hooks (validate, acquire locks, determine if init needed)
	needsInit, cleanup, err := r.prepareLifecycleHooks(ctx, req.Podman.Lifecycle, sessionID)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Wrap command with lifecycle hooks if any are configured
	// Init hooks (onCreateCommand, postCreate) only run on first session use
	// PostStart runs every time
	commandWithHooks := WrapCommandWithHooks(claudeCmd, req.Podman.Lifecycle, needsInit)

	// Wrap command to create workdir if it doesn't exist
	finalCmd := wrapCommandWithWorkdirSetup(req.Workdir, commandWithHooks, r.workdirAutoCreate)
	podmanBuilder.WithCommand(finalCmd)
	fullCmd := podmanBuilder.Build()

	// Track active container
	metrics.IncActiveContainers()
	defer metrics.DecActiveContainers()

	// Execute command with streaming using executor
	stdoutPipe, stderrPipe, start, wait, err := r.executor.RunStream(ctx, fullCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to setup streaming: %w", err)
	}

	// Start command
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

			// Send to output channel (non-blocking if context cancelled)
			select {
			case output <- line:
			case <-ctx.Done():
				return
			}
		}
	}

	// Start streaming both pipes
	go func() {
		streamPipe(stdoutPipe)
		done <- struct{}{}
	}()

	go func() {
		streamPipe(stderrPipe)
		done <- struct{}{}
	}()

	// Wait for both pipes to finish
	<-done
	<-done

	// Wait for command to finish
	cmdErr := wait()

	if cmdErr != nil {
		tracing.RecordError(ctx, cmdErr)

		// Check if this is a crash (signal termination)
		if IsCrash(cmdErr) {
			crashInfo := ExtractCrashInfo(cmdErr, allOutput.String())
			tracing.AddSpanAttributes(ctx,
				"runner.crash", true,
				"runner.crash_signal", crashInfo.Signal,
				"runner.crash_exit_code", crashInfo.ExitCode,
				"runner.task_completed", crashInfo.TaskCompleted,
			)
			tracing.SetSpanStatus(ctx, tracing.StatusError, "execution crashed")

			// Return result with crash info instead of error
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

	// Mark session as initialized if we ran init hooks successfully
	// Note: Lock cleanup is handled by defer, so we don't need to release manually
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

	// Add result attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.result_id", result.ID,
		"runner.output_length", len(result.Output),
		"runner.session_id", result.SessionID,
	)
	tracing.SetSpanStatus(ctx, tracing.StatusOK, "streaming completed")

	return result, nil
}

// DestroySession removes a session and all its data
func (r *PodmanRunner) DestroySession(sessionID string) error {
	// Clean up compose stack if active
	if r.composeMgr != nil {
		_ = r.composeMgr.Down(context.Background(), sessionID)
	}
	return r.sessionMgr.Destroy(sessionID)
}

// ListSessions returns all existing session IDs
func (r *PodmanRunner) ListSessions() ([]string, error) {
	return r.sessionMgr.List()
}

// getHostSessionPath returns the host path for a session directory
// This is used when mounting sessions into agent containers
func (r *PodmanRunner) getHostSessionPath(sessionID string) string {
	return filepath.Join(r.sessionsHostDir, sessionID)
}

// applyRequestConfig validates and applies workdir, volumes, secrets, and resource limits
// to the podman builder. This consolidates the common configuration logic used by both
// Run() and RunStream().
func (r *PodmanRunner) applyRequestConfig(req Request, podmanBuilder *podman.CommandBuilder) error {
	// Validate workdir for shell safety
	if err := validateWorkdir(req.Workdir); err != nil {
		return fmt.Errorf("workdir validation failed: %w", err)
	}

	// Validate and add volumes (host:container[:options] format)
	if err := r.validateVolumes(req.Podman.Volumes); err != nil {
		return fmt.Errorf("volume validation failed: %w", err)
	}

	// Auto-create host directories for volume mounts if enabled
	if r.volumeAutoCreate {
		if err := r.ensureVolumeHostDirs(req.Podman.Volumes); err != nil {
			return fmt.Errorf("failed to create volume host directories: %w", err)
		}
	}

	for _, vol := range req.Podman.Volumes {
		podmanBuilder.WithVolumeRaw(vol)
	}

	// Set working directory inside container
	if req.Workdir != "" {
		podmanBuilder.WithWorkdir(req.Workdir)
	}

	// Validate and mount secrets as environment variables
	if err := ValidateSecretsEnv(req.Podman.SecretsEnv); err != nil {
		return fmt.Errorf("secrets validation failed: %w", err)
	}
	for envVar, secretName := range req.Podman.SecretsEnv {
		podmanBuilder.WithSecretEnv(secretName, envVar)
	}

	// Apply resource limits
	if req.Podman.Memory != "" {
		podmanBuilder.WithMemory(req.Podman.Memory)
	}
	if req.Podman.CPUs != "" {
		podmanBuilder.WithCPUs(req.Podman.CPUs)
	}
	if req.Podman.CPUShares != nil {
		podmanBuilder.WithCPUShares(*req.Podman.CPUShares)
	}

	return nil
}

// RunAsync executes Claude in a goroutine and calls onComplete when done
func (r *PodmanRunner) RunAsync(ctx context.Context, req Request, jobID string, onComplete func(*Result, error)) {
	go func() {
		ctx, span := tracing.StartSpanWithKind(ctx, "runner.RunAsync", tracing.SpanKindInternal)
		defer span.End()

		tracing.AddSpanAttributes(ctx,
			"runner.job_id", jobID,
			"runner.async", true,
		)

		result, err := r.Run(ctx, req)
		if err != nil {
			tracing.RecordError(ctx, err)
			tracing.SetSpanStatus(ctx, tracing.StatusError, "async execution failed")
		} else {
			tracing.SetSpanStatus(ctx, tracing.StatusOK, "async execution completed")
		}
		onComplete(result, err)
	}()
}

// applyClaudeOptions applies all Claude options to the builder
func (r *PodmanRunner) applyClaudeOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	// Note: SessionID is handled separately in Run()
	r.applySessionOptions(b, opts)
	r.applyModelOptions(b, opts)
	r.applyPromptOptions(b, opts)
	r.applyToolsOptions(b, opts)
	r.applyPermissionOptions(b, opts)
	r.applyIOOptions(b, opts)
	r.applyResourceOptions(b, opts)
	r.applyMiscOptions(b, opts)
}

// applySessionOptions handles Continue, ForkSession, NoPersistence options
func (r *PodmanRunner) applySessionOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.Continue {
		b.WithContinue()
	}
	if opts.ForkSession {
		b.WithForkSession()
	}
	if opts.NoPersistence {
		b.WithNoSessionPersistence()
	}
}

// applyModelOptions handles Model, FallbackModel options
func (r *PodmanRunner) applyModelOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.Model != "" {
		b.WithModel(opts.Model)
	}
	if opts.FallbackModel != "" {
		b.WithFallbackModel(opts.FallbackModel)
	}
}

// applyPromptOptions handles SystemPrompt, AppendSystemPrompt options
func (r *PodmanRunner) applyPromptOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.SystemPrompt != "" {
		b.WithSystemPrompt(opts.SystemPrompt)
	}
	if opts.AppendSystemPrompt != "" {
		b.WithAppendSystemPrompt(opts.AppendSystemPrompt)
	}
}

// applyToolsOptions handles Tools, AllowedTools, DisallowedTools options
func (r *PodmanRunner) applyToolsOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if len(opts.Tools) > 0 {
		b.WithTools(opts.Tools...)
	}
	if len(opts.AllowedTools) > 0 {
		b.WithAllowedTools(opts.AllowedTools...)
	}
	if len(opts.DisallowedTools) > 0 {
		b.WithDisallowedTools(opts.DisallowedTools...)
	}
}

// applyPermissionOptions handles PermissionMode, DangerouslySkipPermissions, AllowDangerouslySkipPermissions options
func (r *PodmanRunner) applyPermissionOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.PermissionMode != "" {
		b.WithPermissionMode(opts.PermissionMode)
	}
	if opts.DangerouslySkipPermissions {
		b.WithDangerouslySkipPermissions()
	}
	if opts.AllowDangerouslySkipPermissions {
		b.WithAllowDangerouslySkipPermissions()
	}
}

// applyIOOptions handles InputFormat, OutputFormat, IncludePartialMessages, ReplayUserMessages, JSONSchema options
func (r *PodmanRunner) applyIOOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.InputFormat != "" {
		b.WithInputFormat(opts.InputFormat)
	}
	if opts.OutputFormat != "" {
		b.WithOutputFormat(opts.OutputFormat)
	}
	if opts.IncludePartialMessages {
		b.WithIncludePartialMessages()
	}
	if opts.ReplayUserMessages {
		b.WithReplayUserMessages()
	}
	if opts.JSONSchema != "" {
		b.WithJSONSchema(opts.JSONSchema)
	}
}

// applyResourceOptions handles MaxBudgetUSD, MCPConfigs, StrictMCPConfig, Agent, Agents, AddDirs, PluginDirs, Files, Settings, SettingSources, Betas options
func (r *PodmanRunner) applyResourceOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.MaxBudgetUSD != nil {
		b.WithMaxBudgetUSD(*opts.MaxBudgetUSD)
	}
	if opts.MaxTurns != nil {
		b.WithMaxTurns(*opts.MaxTurns)
	}
	if len(opts.MCPConfigs) > 0 {
		b.WithMCPConfig(opts.MCPConfigs...)
	}
	if opts.StrictMCPConfig {
		b.WithStrictMCPConfig()
	}
	if opts.Agent != "" {
		b.WithAgent(opts.Agent)
	}
	if opts.Agents != nil {
		b.WithAgents(opts.Agents)
	}
	if len(opts.AddDirs) > 0 {
		b.WithAddDir(opts.AddDirs...)
	}
	if len(opts.PluginDirs) > 0 {
		b.WithPluginDir(opts.PluginDirs...)
	}
	if len(opts.Files) > 0 {
		b.WithFiles(opts.Files...)
	}
	if opts.Settings != "" {
		b.WithSettings(opts.Settings)
	}
	if len(opts.SettingSources) > 0 {
		b.WithSettingSources(opts.SettingSources...)
	}
	if len(opts.Betas) > 0 {
		b.WithBetas(opts.Betas...)
	}
}

// applyMiscOptions handles Verbose, Debug, DisableSlashCommands options
func (r *PodmanRunner) applyMiscOptions(b *claude.CommandBuilder, opts types.ClaudeOptions) {
	if opts.Verbose {
		b.WithVerbose()
	}
	if opts.Debug != "" {
		b.WithDebug(opts.Debug)
	}
	if opts.DisableSlashCommands {
		b.WithDisableSlashCommands()
	}
}

// generateRunID creates a unique run ID
func generateRunID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes) // Error ignored: crypto/rand.Read always succeeds on supported platforms
	return "run-" + hex.EncodeToString(bytes)
}

// applyDefaults applies default resource limits when not explicitly specified
func (r *PodmanRunner) applyDefaults(opts types.PodmanOptions) types.PodmanOptions {
	if opts.Memory == "" && r.defaults.Memory != "" {
		opts.Memory = r.defaults.Memory
	}
	if opts.CPUs == "" && r.defaults.CPUs != "" {
		opts.CPUs = r.defaults.CPUs
	}
	if opts.Timeout == "" && r.defaults.Timeout != "" {
		opts.Timeout = r.defaults.Timeout
	}
	return opts
}

// validateVolumes validates volume mount strings against multiple security checks:
// 1. Volume format (host:container[:options])
// 2. Host path against allowlist
// 3. Container path against blocklist
// 4. Mount options against allowlist
func (r *PodmanRunner) validateVolumes(volumes []string) error {
	for _, vol := range volumes {
		hostPath, containerPath, options, err := parseVolumeComponents(vol)
		if err != nil {
			return fmt.Errorf("invalid volume format %q: %w", vol, err)
		}

		// Validate host path against allowlist
		if _, err := r.volumeValidator.Validate(hostPath); err != nil {
			return fmt.Errorf("volume host path not allowed: %w", err)
		}

		// Validate container path against blocklist
		if err := validateContainerPath(containerPath); err != nil {
			return fmt.Errorf("volume container path invalid: %w", err)
		}

		// Validate mount options
		if err := validateMountOptions(options); err != nil {
			return fmt.Errorf("volume mount options invalid: %w", err)
		}
	}
	return nil
}

// ensureVolumeHostDirs creates host directories for volume mounts if they don't exist
// This mimics Docker/Podman's default behavior of auto-creating mount points
func (r *PodmanRunner) ensureVolumeHostDirs(volumes []string) error {
	for _, vol := range volumes {
		hostPath, _, _, err := parseVolumeComponents(vol)
		if err != nil {
			// Skip invalid volumes - they'll fail validation anyway
			continue
		}

		// Check if the path exists
		if _, err := os.Stat(hostPath); os.IsNotExist(err) {
			// Create the directory with user-writable permissions
			// Use 0755 to match typical directory permissions
			if err := os.MkdirAll(hostPath, 0755); err != nil {
				return fmt.Errorf("failed to create host directory %q: %w", hostPath, err)
			}
			slog.Debug("Auto-created volume host directory", "path", hostPath)
		}
	}
	return nil
}

// wrapCommandWithWorkdirSetup wraps the command to create the workdir if it doesn't exist
func wrapCommandWithWorkdirSetup(workdir string, cmd []string, autoCreate bool) []string {
	if workdir == "" || !autoCreate {
		return cmd
	}

	// Escape each argument for shell using the shared helper
	var escapedArgs []string
	for _, arg := range cmd {
		escapedArgs = append(escapedArgs, EscapeShellArg(arg))
	}

	// Build the wrapped command:
	// mkdir -p '/workdir' && exec original_command
	shellCmd := fmt.Sprintf("mkdir -p %s && exec %s", EscapeShellArg(workdir), strings.Join(escapedArgs, " "))

	return []string{"sh", "-c", shellCmd}
}

// Security validation constants
const (
	// MaxWorkdirLength is the maximum length of workdir path
	// Based on PATH_MAX (4096) on Linux systems
	MaxWorkdirLength = 4096

	// MaxVolumePathLength is the maximum length of volume paths
	// Based on PATH_MAX (4096) on Linux systems
	MaxVolumePathLength = 4096
)

// blockedContainerPaths are sensitive container paths that cannot be mounted to
// Using a map for O(1) lookup performance
var blockedContainerPaths = map[string]bool{
	// User credential directories
	"/home/user/.claude":          true, // Claude credentials and config
	"/home/user/.ssh":             true, // SSH keys
	"/home/user/.gnupg":           true, // GPG keys
	"/home/user/.aws":             true, // AWS credentials
	"/home/user/.docker":          true, // Docker credentials
	"/home/user/.kube":            true, // Kubernetes credentials
	"/home/user/.config":          true, // Application configs (often contain secrets)
	"/home/user/.local":           true, // Local data (often contains secrets)
	"/home/user/.netrc":           true, // Network credentials
	"/home/user/.git-credentials": true, // Git credentials
	"/home/user/.password-store":  true, // Pass password manager
	// Shell config (injection risk)
	"/home/user/.bashrc":       true,
	"/home/user/.bash_profile": true,
	"/home/user/.profile":      true,
	"/home/user/.zshrc":        true,
	// System paths
	"/etc":           true, // System config
	"/root":          true, // Root home
	"/bin":           true, // System binaries
	"/sbin":          true, // System binaries
	"/usr/bin":       true, // User binaries
	"/usr/sbin":      true, // User binaries
	"/usr/local/bin": true, // Local binaries
	"/lib":           true, // System libraries
	"/lib64":         true, // System libraries
	"/var/run":       true, // Runtime data
	"/run":           true, // Runtime data
	"/proc":          true, // Process info
	"/sys":           true, // System info
	"/dev":           true, // Devices
}

// allowedMountOptions are the only mount options allowed in volume specifications
var allowedMountOptions = map[string]bool{
	"ro":       true, // Read-only
	"rw":       true, // Read-write (default)
	"z":        true, // SELinux shared label
	"Z":        true, // SELinux private label
	"noexec":   true, // Prevent execution (security-enhancing)
	"nosuid":   true, // Prevent setuid (security-enhancing)
	"nodev":    true, // Prevent device files (security-enhancing)
	"rslave":   true, // Recursive slave propagation
	"rprivate": true, // Recursive private propagation
	"rshared":  true, // Recursive shared propagation
	"slave":    true, // Slave propagation
	"private":  true, // Private propagation
	"shared":   true, // Shared propagation
	"nocopy":   true, // Don't copy data from container
	"copy":     true, // Copy data from container
	"U":        true, // Chown to container user
}

// validWorkdirPattern only allows safe characters in workdir paths
var validWorkdirPattern = regexp.MustCompile(`^[a-zA-Z0-9/_.\-]+$`)

// validateWorkdir validates the workdir path for safety
func validateWorkdir(workdir string) error {
	if workdir == "" {
		return nil // Empty workdir is valid
	}

	// Check length
	if len(workdir) > MaxWorkdirLength {
		return fmt.Errorf("workdir path too long (max %d characters)", MaxWorkdirLength)
	}

	// Must be absolute path
	if !strings.HasPrefix(workdir, "/") {
		return fmt.Errorf("workdir must be an absolute path starting with /")
	}

	// Check for safe characters only
	if !validWorkdirPattern.MatchString(workdir) {
		return fmt.Errorf("workdir contains invalid characters (only alphanumeric, /, _, ., - allowed)")
	}

	// Check for path traversal attempts
	if strings.Contains(workdir, "..") {
		return fmt.Errorf("workdir cannot contain path traversal (..) sequences")
	}

	return nil
}

// validateContainerPath checks if a container path is allowed
func validateContainerPath(containerPath string) error {
	if containerPath == "" {
		return fmt.Errorf("container path cannot be empty")
	}

	// Check length
	if len(containerPath) > MaxVolumePathLength {
		return fmt.Errorf("container path too long (max %d characters)", MaxVolumePathLength)
	}

	// Must be absolute
	if !strings.HasPrefix(containerPath, "/") {
		return fmt.Errorf("container path must be absolute")
	}

	// Clean the path for consistent comparison
	cleanPath := strings.TrimSuffix(containerPath, "/")

	// Check against blocked paths using O(1) map lookup
	// We check the path and all its parent directories
	pathToCheck := cleanPath
	for pathToCheck != "" && pathToCheck != "/" {
		if blockedContainerPaths[pathToCheck] {
			return fmt.Errorf("container path %q is blocked for security reasons", containerPath)
		}
		// Move to parent directory
		pathToCheck = filepath.Dir(pathToCheck)
	}

	return nil
}

// validateMountOptions validates volume mount options
func validateMountOptions(options string) error {
	if options == "" {
		return nil // No options is valid
	}

	// Split options by comma
	for _, opt := range strings.Split(options, ",") {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}

		if !allowedMountOptions[opt] {
			return fmt.Errorf("mount option %q is not allowed", opt)
		}
	}

	return nil
}

// parseVolumeComponents parses a volume string into host path, container path, and options
func parseVolumeComponents(volume string) (hostPath, containerPath, options string, err error) {
	parts := strings.Split(volume, ":")

	// Handle Windows paths (e.g., C:\path:container)
	if len(parts) >= 2 && len(parts[0]) == 1 && strings.ToUpper(parts[0]) >= "A" && strings.ToUpper(parts[0]) <= "Z" {
		// Windows path like C:\foo:/container
		if len(parts) < 3 {
			return "", "", "", fmt.Errorf("invalid volume format, expected host:container")
		}
		hostPath = parts[0] + ":" + parts[1]
		containerPath = parts[2]
		if len(parts) >= 4 {
			options = parts[3]
		}
		return hostPath, containerPath, options, nil
	}

	// Unix paths
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid volume format, expected host:container[:options]")
	}

	hostPath = parts[0]
	containerPath = parts[1]
	if len(parts) >= 3 {
		options = parts[2]
	}

	return hostPath, containerPath, options, nil
}
