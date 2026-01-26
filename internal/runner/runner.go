package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"stromboli/internal/claude"
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
	Workspace string
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

// PodmanRunner runs Claude using Podman containers
type PodmanRunner struct {
	image              string
	secretsMgr         *secrets.Manager
	sessionMgr         *session.Manager
	workspaceValidator *workspace.Validator
	imageValidator     *ImageValidator
	mountClaudeCLI     bool
	defaults           ResourceDefaults
	executor           Executor
}

// NewPodmanRunner creates a new Podman-based runner with no default resource limits
// It ensures the Podman secret exists for secure token handling
func NewPodmanRunner(image, secretsFile, sessionsDir string, allowedWorkspaces []string) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir, allowedWorkspaces, ResourceDefaults{})
}

// NewPodmanRunnerWithExecutor creates a new Podman-based runner with a custom executor
// This is primarily useful for testing
func NewPodmanRunnerWithExecutor(image, secretsFile, sessionsDir string, allowedWorkspaces []string, executor Executor) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaultsAndExecutor(image, secretsFile, sessionsDir, allowedWorkspaces, ResourceDefaults{}, executor)
}

// NewPodmanRunnerWithDefaults creates a new Podman-based runner with default resource limits
// It ensures the Podman secret exists for secure token handling
func NewPodmanRunnerWithDefaults(image, secretsFile, sessionsDir string, allowedWorkspaces []string, defaults ResourceDefaults) (*PodmanRunner, error) {
	return NewPodmanRunnerWithDefaultsAndExecutor(image, secretsFile, sessionsDir, allowedWorkspaces, defaults, NewShellExecutor())
}

// NewPodmanRunnerWithDefaultsAndExecutor creates a new Podman-based runner with default resource limits and custom executor
// This is the most flexible constructor, primarily useful for testing
func NewPodmanRunnerWithDefaultsAndExecutor(image, credentialsFile, sessionsDir string, allowedWorkspaces []string, defaults ResourceDefaults, executor Executor) (*PodmanRunner, error) {
	return NewPodmanRunnerFull(image, credentialsFile, sessionsDir, allowedWorkspaces, defaults, ImageConfig{}, executor)
}

// NewPodmanRunnerFull creates a new Podman-based runner with all configuration options
func NewPodmanRunnerFull(image, credentialsFile, sessionsDir string, allowedWorkspaces []string, defaults ResourceDefaults, imageConfig ImageConfig, executor Executor) (*PodmanRunner, error) {
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
		image:              image,
		secretsMgr:         secretsMgr,
		sessionMgr:         session.NewManager(sessionsDir),
		workspaceValidator: workspace.NewValidator(allowedWorkspaces),
		imageValidator:     imageValidator,
		mountClaudeCLI:     imageConfig.MountClaudeCLI,
		defaults:           defaults,
		executor:           executor,
	}, nil
}

// Run executes Claude in a Podman container
func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
	ctx, span := tracing.StartSpanWithKind(ctx, "runner.Run", tracing.SpanKindInternal)
	defer span.End()

	// Add request attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.prompt_length", len(req.Prompt),
		"runner.workspace", req.Workspace,
		"runner.model", req.Claude.Model,
		"runner.session_id", req.Claude.SessionID,
	)

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

	// Create or get session directory
	sessionID, absSessionPath, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}

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
	podmanBuilder.WithVolume(absSessionPath, "/home/user")

	// Mount credentials via Podman secret (more secure than volume mount)
	// Secret is mounted at ~/.claude/.credentials.json so Claude finds it automatically
	podmanBuilder.WithSecretTarget(r.secretsMgr.SecretName(), "/home/user/.claude/.credentials.json")

	// Mount workspace for container access
	if req.Workspace != "" {
		validatedPath, err := r.workspaceValidator.Validate(req.Workspace)
		if err != nil {
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
		podmanBuilder.
			WithVolume(validatedPath, "/workspace").
			WithWorkdir("/workspace")
	}

	// Add additional volumes from request
	for _, vol := range req.Podman.Volumes {
		podmanBuilder.WithVolumeRaw(vol)
	}

	// Mount secrets as environment variables
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
	if req.Podman.CPUShares > 0 {
		podmanBuilder.WithCPUShares(req.Podman.CPUShares)
	}

	// When mounting Claude CLI image, prepend "claude" to command
	// (dynamic images don't have an entrypoint that provides it)
	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}
	podmanBuilder.WithCommand(claudeCmd)
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
	defer close(output)

	// Add request attributes to span
	tracing.AddSpanAttributes(ctx,
		"runner.prompt_length", len(req.Prompt),
		"runner.workspace", req.Workspace,
		"runner.model", req.Claude.Model,
		"runner.session_id", req.Claude.SessionID,
		"runner.streaming", true,
	)

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

	// Create or get session directory
	sessionID, absSessionPath, err := r.sessionMgr.Create(req.Claude.SessionID)
	if err != nil {
		return nil, err
	}

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
	podmanBuilder.WithVolume(absSessionPath, "/home/user")

	// Mount credentials via Podman secret (more secure than volume mount)
	podmanBuilder.WithSecretTarget(r.secretsMgr.SecretName(), "/home/user/.claude/.credentials.json")

	// Mount workspace for container access
	if req.Workspace != "" {
		validatedPath, err := r.workspaceValidator.Validate(req.Workspace)
		if err != nil {
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
		podmanBuilder.
			WithVolume(validatedPath, "/workspace").
			WithWorkdir("/workspace")
	}

	// Add additional volumes from request
	for _, vol := range req.Podman.Volumes {
		podmanBuilder.WithVolumeRaw(vol)
	}

	// Mount secrets as environment variables
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
	if req.Podman.CPUShares > 0 {
		podmanBuilder.WithCPUShares(req.Podman.CPUShares)
	}

	// When mounting Claude CLI image, prepend "claude" to command
	// (dynamic images don't have an entrypoint that provides it)
	if r.mountClaudeCLI {
		claudeCmd = append([]string{"claude"}, claudeCmd...)
	}
	podmanBuilder.WithCommand(claudeCmd)
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
	return r.sessionMgr.Destroy(sessionID)
}

// ListSessions returns all existing session IDs
func (r *PodmanRunner) ListSessions() ([]string, error) {
	return r.sessionMgr.List()
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
	if opts.MaxBudgetUSD > 0 {
		b.WithMaxBudgetUSD(opts.MaxBudgetUSD)
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
