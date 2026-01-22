package claude

import (
	"os"
	"path/filepath"
	"testing"

	strerrors "github.com/tomblanc/stromboli/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DefaultSecretsFile(t *testing.T) {
	client := NewClient("")
	assert.Equal(t, DefaultSecretsFile, client.secretsFile)
}

func TestNewClient_CustomSecretsFile(t *testing.T) {
	client := NewClient("/path/to/secrets")
	assert.Equal(t, "/path/to/secrets", client.secretsFile)
}

func TestIsConfigured_WhenFileDoesNotExist_ReturnsFalse(t *testing.T) {
	client := NewClient("/nonexistent/path/secrets")
	assert.False(t, client.IsConfigured())
}

func TestIsConfigured_WhenFileExists_ReturnsTrue(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("test-token"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	assert.True(t, client.IsConfigured())
}

func TestGetToken_WhenFileDoesNotExist_ReturnsError(t *testing.T) {
	client := NewClient("/nonexistent/path/secrets")
	token, err := client.GetToken()
	assert.ErrorIs(t, err, strerrors.ErrTokenNotFound)
	assert.Empty(t, token)
}

func TestGetToken_WhenFileExists_ReturnsToken(t *testing.T) {
	// Create temp file with token
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("my-secret-token\n"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	token, err := client.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "my-secret-token", token)
}

func TestGetToken_TrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, ".claude-secrets")
	err := os.WriteFile(secretsFile, []byte("  token-with-spaces  \n\n"), 0600)
	require.NoError(t, err)

	client := NewClient(secretsFile)
	token, err := client.GetToken()
	assert.NoError(t, err)
	assert.Equal(t, "token-with-spaces", token)
}
