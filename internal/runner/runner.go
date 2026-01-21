package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tomblanc/stromboli/internal/claude"
	"github.com/tomblanc/stromboli/internal/podman"
)

// Runner executes Claude in a container
type Runner interface {
	Run(ctx context.Context, req Request) (*Result, error)
}

// Request contains the parameters for running Claude
type Request struct {
	Prompt    string
	Workspace string
	Model     string
	SessionID string
}

// Result contains the output from running Claude
type Result struct {
	ID        string
	Output    string
	SessionID string
}

// PodmanRunner runs Claude using Podman containers
type PodmanRunner struct {
	image       string
	secretsFile string
}

// NewPodmanRunner creates a new Podman-based runner
func NewPodmanRunner(image, secretsFile string) *PodmanRunner {
	return &PodmanRunner{
		image:       image,
		secretsFile: secretsFile,
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

	// Build Claude command
	claudeBuilder := claude.NewCommandBuilder().
		WithPrompt(req.Prompt).
		WithDangerouslySkipPermissions()

	if req.Model != "" {
		claudeBuilder.WithModel(req.Model)
	}

	if req.SessionID != "" {
		claudeBuilder.WithSessionID(req.SessionID)
	}

	claudeCmd := claudeBuilder.Build()

	// Build Podman command
	podmanBuilder := podman.NewCommand().
		WithEnv("CLAUDE_CODE_OAUTH_TOKEN", token).
		WithImage(r.image)

	if req.Workspace != "" {
		podmanBuilder.
			WithVolume(req.Workspace, "/workspace").
			WithWorkdir("/workspace")
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
		ID:        generateID(),
		Output:    strings.TrimSpace(string(output)),
		SessionID: req.SessionID,
	}, nil
}

// generateID creates a simple unique ID for the run
func generateID() string {
	return fmt.Sprintf("run-%d", randomID())
}

var idCounter int

func randomID() int {
	idCounter++
	return idCounter
}
