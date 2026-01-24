package claude

import (
	"os"
	"path/filepath"
	"testing"

	strerrors "stromboli/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_DefaultCredentialsFile(t *testing.T) {
	client := NewClient("")
	// Default path should be expanded from ~/.claude/.credentials.json
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", ".credentials.json")
	assert.Equal(t, expected, client.credentialsFile)
}

func TestNewClient_CustomCredentialsFile(t *testing.T) {
	client := NewClient("/path/to/credentials.json")
	assert.Equal(t, "/path/to/credentials.json", client.credentialsFile)
}

func TestNewClient_ExpandsHomePath(t *testing.T) {
	client := NewClient("~/.claude/test.json")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", "test.json")
	assert.Equal(t, expected, client.credentialsFile)
}

func TestCredentialsFile_ReturnsPath(t *testing.T) {
	client := NewClient("/custom/path.json")
	assert.Equal(t, "/custom/path.json", client.CredentialsFile())
}

func TestIsConfigured_WhenFileDoesNotExist_ReturnsFalse(t *testing.T) {
	client := NewClient("/nonexistent/path/credentials.json")
	assert.False(t, client.IsConfigured())
}

func TestIsConfigured_WhenFileExists_ReturnsTrue(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	credentialsFile := filepath.Join(tmpDir, ".credentials.json")
	err := os.WriteFile(credentialsFile, []byte("{}"), 0600)
	require.NoError(t, err)

	client := NewClient(credentialsFile)
	assert.True(t, client.IsConfigured())
}

func TestGetToken_ReturnsError_Deprecated(t *testing.T) {
	// GetToken is deprecated - always returns error now
	tmpDir := t.TempDir()
	credentialsFile := filepath.Join(tmpDir, ".credentials.json")
	err := os.WriteFile(credentialsFile, []byte("{}"), 0600)
	require.NoError(t, err)

	client := NewClient(credentialsFile)
	token, err := client.GetToken()
	assert.ErrorIs(t, err, strerrors.ErrTokenNotFound)
	assert.Empty(t, token)
}

func TestGetToken_WhenFileDoesNotExist_ReturnsError(t *testing.T) {
	client := NewClient("/nonexistent/path/credentials.json")
	token, err := client.GetToken()
	assert.ErrorIs(t, err, strerrors.ErrTokenNotFound)
	assert.Empty(t, token)
}

func TestNewClientWithCache_BackwardCompatible(t *testing.T) {
	// NewClientWithCache is deprecated but should still work
	client := NewClientWithCache("/path/to/credentials.json", true, nil)
	assert.Equal(t, "/path/to/credentials.json", client.credentialsFile)
}

func TestInvalidateCache_NoOp(t *testing.T) {
	// InvalidateCache is deprecated but should not panic
	client := NewClient("/path/to/credentials.json")
	client.InvalidateCache() // Should not panic
}

func TestExpandPath_HomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	result := expandPath("~/test/path")
	assert.Equal(t, filepath.Join(home, "test", "path"), result)
}

func TestExpandPath_AbsolutePath(t *testing.T) {
	result := expandPath("/absolute/path")
	assert.Equal(t, "/absolute/path", result)
}

func TestExpandPath_RelativePath(t *testing.T) {
	result := expandPath("relative/path")
	assert.Equal(t, "relative/path", result)
}
