package claude

import (
	"os"
	"path/filepath"
	"strings"

	strerrors "stromboli/internal/errors"
)

const (
	// DefaultCredentialsFile is the default path to Claude credentials
	DefaultCredentialsFile = "~/.claude/.credentials.json"
)

// Client wraps Claude credentials operations
type Client struct {
	credentialsFile string
}

// NewClient creates a new Claude client
func NewClient(credentialsFile string) *Client {
	if credentialsFile == "" {
		credentialsFile = DefaultCredentialsFile
	}
	return &Client{
		credentialsFile: expandPath(credentialsFile),
	}
}

// NewClientWithCache creates a new Claude client (cache params ignored for compatibility)
// Deprecated: Use NewClient instead. Cache is no longer used.
func NewClientWithCache(credentialsFile string, _ bool, _ any) *Client {
	return NewClient(credentialsFile)
}

// CredentialsFile returns the resolved path to the credentials file
func (c *Client) CredentialsFile() string {
	return c.credentialsFile
}

// IsConfigured checks if credentials file exists
func (c *Client) IsConfigured() bool {
	_, err := os.Stat(c.credentialsFile)
	return err == nil
}

// GetToken is deprecated - credentials file is mounted directly into containers
// Returns error directing users to the new approach
func (c *Client) GetToken() (string, error) {
	if !c.IsConfigured() {
		return "", strerrors.ErrTokenNotFound
	}
	return "", strerrors.ErrTokenNotFound
}

// InvalidateCache is a no-op for compatibility
// Deprecated: Cache is no longer used
func (c *Client) InvalidateCache() {}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
