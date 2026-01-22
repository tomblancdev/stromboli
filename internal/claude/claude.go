package claude

import (
	"os"
	"strings"

	strerrors "github.com/tomblanc/stromboli/internal/errors"
)

const DefaultSecretsFile = ".claude-secrets"

// Client wraps Claude Code token operations
type Client struct {
	secretsFile string
}

// NewClient creates a new Claude client
func NewClient(secretsFile string) *Client {
	if secretsFile == "" {
		secretsFile = DefaultSecretsFile
	}
	return &Client{
		secretsFile: secretsFile,
	}
}

// IsConfigured checks if token file exists
func (c *Client) IsConfigured() bool {
	_, err := os.Stat(c.secretsFile)
	return err == nil
}

// GetToken reads the token from secrets file
func (c *Client) GetToken() (string, error) {
	data, err := os.ReadFile(c.secretsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", strerrors.ErrTokenNotFound
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
