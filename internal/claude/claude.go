package claude

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/tomblanc/stromboli/internal/container"
)

var ErrNotImplemented = errors.New("not implemented")
var ErrLoginFailed = errors.New("login failed")

// Client wraps Claude Code CLI operations
type Client struct {
	volumeName string
	imageName  string
	manager    *container.Manager
}

// NewClient creates a new Claude client
func NewClient(volumeName string) *Client {
	return &Client{
		volumeName: volumeName,
		imageName:  "stromboli-agent:latest",
		manager:    container.NewManager(),
	}
}

// IsConfigured checks if Claude credentials exist in the volume
func (c *Client) IsConfigured() (bool, error) {
	return c.manager.VolumeExists(c.volumeName)
}

// Login runs claude login to authenticate interactively
func (c *Client) Login() error {
	// Create volume if it doesn't exist
	exists, err := c.manager.VolumeExists(c.volumeName)
	if err != nil {
		return err
	}

	if !exists {
		if err := c.manager.VolumeCreate(c.volumeName); err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}
	}

	// Run claude login interactively
	// Using exec directly for TTY support
	cmd := exec.Command("podman", "run", "--rm", "-it",
		"-v", c.volumeName+":/home/claude/.claude",
		c.imageName,
		"login",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrLoginFailed, err)
	}

	return nil
}

// Logout runs claude logout to remove credentials
func (c *Client) Logout() error {
	return ErrNotImplemented
}
