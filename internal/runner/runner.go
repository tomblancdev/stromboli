package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/podman"
	"github.com/tomblanc/stromboli/internal/secrets"
	"github.com/tomblanc/stromboli/internal/session"
	"github.com/tomblanc/stromboli/internal/types"
	"github.com/tomblanc/stromboli/internal/workspace"
)

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
}

// PodmanRunner runs Claude using Podman containers
type PodmanRunner struct {
	image              string
	secretsMgr         *secrets.Manager
	sessionMgr         *session.Manager
	workspaceValidator *workspace.Validator
}

// NewPodmanRunner creates a new Podman-based runner
// It ensures the Podman secret exists for secure token handling
func NewPodmanRunner(image, secretsFile, sessionsDir string, allowedWorkspaces []string) (*PodmanRunner, error) {
	// Create secrets manager and ensure secret exists
	// Use background context for startup initialization
	secretsMgr := secrets.NewManager("")
	if err := secretsMgr.EnsureExists(context.Background(), secretsFile); err != nil {
		return nil, fmt.Errorf("failed to setup secrets: %w", err)
	}

	return &PodmanRunner{
		image:              image,
		secretsMgr:         secretsMgr,
		sessionMgr:         session.NewManager(sessionsDir),
		workspaceValidator: workspace.NewValidator(allowedWorkspaces),
	}, nil
}

// Run executes Claude in a Podman container
func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
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
	podmanBuilder := podman.NewCommand().
		WithSecret(r.secretsMgr.SecretName()).
		WithEnv("HOME", "/home/claude").
		WithImage(r.image)

	// SESSION ISOLATION: Mount session-specific directory
	// Each session gets its own persistent storage
	// Sessions are isolated from each other
	// Data persists until explicitly destroyed
	// :U flag ensures container user can write to the volume
	podmanBuilder.WithVolumeChown(absSessionPath, "/home/claude/.claude")

	// Mount workspace with :U flag for container user write access
	if req.Workspace != "" {
		validatedPath, err := r.workspaceValidator.Validate(req.Workspace)
		if err != nil {
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
		podmanBuilder.
			WithVolumeChown(validatedPath, "/workspace").
			WithWorkdir("/workspace")
	}

	// Add additional volumes from request
	for _, vol := range req.Podman.Volumes {
		podmanBuilder.WithVolumeRaw(vol)
	}

	podmanBuilder.WithCommand(claudeCmd)
	fullCmd := podmanBuilder.Build()

	// Execute command
	cmd := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w, output: %s", err, string(output))
	}

	return &Result{
		ID:        generateRunID(),
		Output:    strings.TrimSpace(string(output)),
		SessionID: sessionID,
	}, nil
}

// RunStream executes Claude in a Podman container and streams output in real-time
func (r *PodmanRunner) RunStream(ctx context.Context, req Request, output chan<- string) (*Result, error) {
	defer close(output)

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
	podmanBuilder := podman.NewCommand().
		WithSecret(r.secretsMgr.SecretName()).
		WithEnv("HOME", "/home/claude").
		WithImage(r.image)

	// Mount session-specific directory
	podmanBuilder.WithVolumeChown(absSessionPath, "/home/claude/.claude")

	// Mount workspace with :U flag for container user write access
	if req.Workspace != "" {
		validatedPath, err := r.workspaceValidator.Validate(req.Workspace)
		if err != nil {
			return nil, fmt.Errorf("workspace validation failed: %w", err)
		}
		podmanBuilder.
			WithVolumeChown(validatedPath, "/workspace").
			WithWorkdir("/workspace")
	}

	// Add additional volumes from request
	for _, vol := range req.Podman.Volumes {
		podmanBuilder.WithVolumeRaw(vol)
	}

	podmanBuilder.WithCommand(claudeCmd)
	fullCmd := podmanBuilder.Build()

	// Execute command with streaming
	cmd := exec.CommandContext(ctx, fullCmd[0], fullCmd[1:]...)

	// Get stdout and stderr pipes
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
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
	cmdErr := cmd.Wait()

	if cmdErr != nil {
		return nil, fmt.Errorf("execution failed: %w", cmdErr)
	}

	return &Result{
		ID:        generateRunID(),
		Output:    strings.TrimSpace(allOutput.String()),
		SessionID: sessionID,
	}, nil
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
		result, err := r.Run(ctx, req)
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
	rand.Read(bytes)
	return "run-" + hex.EncodeToString(bytes)
}
