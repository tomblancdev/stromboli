package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/podman"
)

// Runner executes Claude in a container
type Runner interface {
	Run(ctx context.Context, req Request) (*Result, error)
	DestroySession(sessionID string) error
	ListSessions() ([]string, error)
}

// Request contains the parameters for running Claude
type Request struct {
	Prompt    string
	Workspace string
	Claude    ClaudeOptions
	Podman    PodmanOptions
}

// ClaudeOptions mirrors api.ClaudeOptions for runner layer
type ClaudeOptions struct {
	// Session management
	SessionID     string
	Resume        bool // Resume existing session (requires SessionID)
	Continue      bool // Continue most recent conversation
	ForkSession   bool
	NoPersistence bool

	// Model
	Model         string
	FallbackModel string

	// System prompt
	SystemPrompt       string
	AppendSystemPrompt string

	// Tools
	Tools           []string
	AllowedTools    []string
	DisallowedTools []string

	// Permissions
	PermissionMode                  string
	DangerouslySkipPermissions      bool
	AllowDangerouslySkipPermissions bool

	// I/O format
	InputFormat            string
	OutputFormat           string
	IncludePartialMessages bool
	ReplayUserMessages     bool

	// Structured output
	JSONSchema string

	// Budget
	MaxBudgetUSD float64

	// MCP
	MCPConfigs      []string
	StrictMCPConfig bool

	// Agents
	Agent  string
	Agents map[string]any

	// Resources
	AddDirs    []string
	PluginDirs []string
	Files      []string

	// Settings
	Settings       string
	SettingSources []string

	// Beta
	Betas []string

	// Misc
	Verbose              bool
	Debug                string
	DisableSlashCommands bool
}

// PodmanOptions mirrors api.PodmanOptions for runner layer
type PodmanOptions struct {
	Volumes []string
}

// Result contains the output from running Claude
type Result struct {
	ID        string
	Output    string
	SessionID string
}

// ErrSessionNotFound returns a session not found error
func ErrSessionNotFound(sessionID string) error {
	return fmt.Errorf("session not found: %s", sessionID)
}

// PodmanRunner runs Claude using Podman containers
type PodmanRunner struct {
	image       string
	secretsFile string
	sessionsDir string // Directory to store session data
	mu          sync.Mutex
}

// NewPodmanRunner creates a new Podman-based runner
func NewPodmanRunner(image, secretsFile, sessionsDir string) *PodmanRunner {
	return &PodmanRunner{
		image:       image,
		secretsFile: secretsFile,
		sessionsDir: sessionsDir,
	}
}

// Run executes Claude in a Podman container
func (r *PodmanRunner) Run(ctx context.Context, req Request) (*Result, error) {
	// Get token from secrets
	client := claude.NewClient(r.secretsFile)
	token, err := client.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Generate or use provided session ID
	sessionID := req.Claude.SessionID
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Create session directory if it doesn't exist
	sessionPath := filepath.Join(r.sessionsDir, sessionID)
	if err := os.MkdirAll(sessionPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Get absolute path for Podman volume mount
	absSessionPath, err := filepath.Abs(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute session path: %w", err)
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
		WithEnv("CLAUDE_CODE_OAUTH_TOKEN", token).
		WithEnv("HOME", "/home/claude").
		WithImage(r.image)

	// SESSION ISOLATION: Mount session-specific directory
	// Each session gets its own persistent storage
	// Sessions are isolated from each other
	// Data persists until explicitly destroyed
	// :U flag ensures container user can write to the volume
	podmanBuilder.WithVolumeChown(absSessionPath, "/home/claude/.claude")

	// Mount workspace
	if req.Workspace != "" {
		podmanBuilder.
			WithVolume(req.Workspace, "/workspace").
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

// DestroySession removes a session and all its data
func (r *PodmanRunner) DestroySession(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Validate session ID to prevent path traversal
	if strings.Contains(sessionID, "/") || strings.Contains(sessionID, "..") {
		return fmt.Errorf("invalid session ID")
	}

	sessionPath := filepath.Join(r.sessionsDir, sessionID)

	// Check if session exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Remove session directory
	if err := os.RemoveAll(sessionPath); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}

	return nil
}

// ListSessions returns all existing session IDs
func (r *PodmanRunner) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(r.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if entry.IsDir() {
			sessions = append(sessions, entry.Name())
		}
	}

	return sessions, nil
}

// applyClaudeOptions applies all Claude options to the builder
func (r *PodmanRunner) applyClaudeOptions(b *claude.CommandBuilder, opts ClaudeOptions) {
	// Note: SessionID is handled separately in Run()

	if opts.Continue {
		b.WithContinue()
	}
	if opts.ForkSession {
		b.WithForkSession()
	}
	if opts.NoPersistence {
		b.WithNoSessionPersistence()
	}

	// Model
	if opts.Model != "" {
		b.WithModel(opts.Model)
	}
	if opts.FallbackModel != "" {
		b.WithFallbackModel(opts.FallbackModel)
	}

	// System prompt
	if opts.SystemPrompt != "" {
		b.WithSystemPrompt(opts.SystemPrompt)
	}
	if opts.AppendSystemPrompt != "" {
		b.WithAppendSystemPrompt(opts.AppendSystemPrompt)
	}

	// Tools
	if len(opts.Tools) > 0 {
		b.WithTools(opts.Tools...)
	}
	if len(opts.AllowedTools) > 0 {
		b.WithAllowedTools(opts.AllowedTools...)
	}
	if len(opts.DisallowedTools) > 0 {
		b.WithDisallowedTools(opts.DisallowedTools...)
	}

	// Permissions
	if opts.PermissionMode != "" {
		b.WithPermissionMode(opts.PermissionMode)
	}
	if opts.DangerouslySkipPermissions {
		b.WithDangerouslySkipPermissions()
	}
	if opts.AllowDangerouslySkipPermissions {
		b.WithAllowDangerouslySkipPermissions()
	}

	// I/O format
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

	// Structured output
	if opts.JSONSchema != "" {
		b.WithJSONSchema(opts.JSONSchema)
	}

	// Budget
	if opts.MaxBudgetUSD > 0 {
		b.WithMaxBudgetUSD(opts.MaxBudgetUSD)
	}

	// MCP
	if len(opts.MCPConfigs) > 0 {
		b.WithMCPConfig(opts.MCPConfigs...)
	}
	if opts.StrictMCPConfig {
		b.WithStrictMCPConfig()
	}

	// Agents
	if opts.Agent != "" {
		b.WithAgent(opts.Agent)
	}
	if opts.Agents != nil {
		b.WithAgents(opts.Agents)
	}

	// Resources
	if len(opts.AddDirs) > 0 {
		b.WithAddDir(opts.AddDirs...)
	}
	if len(opts.PluginDirs) > 0 {
		b.WithPluginDir(opts.PluginDirs...)
	}
	if len(opts.Files) > 0 {
		b.WithFiles(opts.Files...)
	}

	// Settings
	if opts.Settings != "" {
		b.WithSettings(opts.Settings)
	}
	if len(opts.SettingSources) > 0 {
		b.WithSettingSources(opts.SettingSources...)
	}

	// Beta
	if len(opts.Betas) > 0 {
		b.WithBetas(opts.Betas...)
	}

	// Misc
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

// generateSessionID creates a unique session ID in UUID format
// Claude CLI expects UUIDs for --resume flag
func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	// Set UUID version 4 bits
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	// Set UUID variant bits
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// generateRunID creates a unique run ID
func generateRunID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "run-" + hex.EncodeToString(bytes)
}
