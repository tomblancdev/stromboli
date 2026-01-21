package claude

import (
	"errors"
	"os/exec"
	"strings"
)

var ErrNotImplemented = errors.New("not implemented")
var ErrConnectionFailed = errors.New("failed to connect to podman")

// Client wraps Claude Code CLI operations
type Client struct {
	volumeName string
}

// NewClient creates a new Claude client
func NewClient(volumeName string) *Client {
	return &Client{
		volumeName: volumeName,
	}
}

// IsConfigured checks if Claude credentials exist in the volume
func (c *Client) IsConfigured() (bool, error) {
	cmd := exec.Command("podman", "volume", "inspect", c.volumeName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's "volume not found" vs actual error
		if strings.Contains(string(output), "no such volume") {
			return false, nil
		}
		return false, ErrConnectionFailed
	}

	return true, nil
}

// Login runs claude login to authenticate
func (c *Client) Login() error {
	return ErrNotImplemented
}

// Logout runs claude logout to remove credentials
func (c *Client) Logout() error {
	return ErrNotImplemented
}
