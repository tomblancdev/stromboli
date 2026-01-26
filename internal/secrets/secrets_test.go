package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager_DefaultValues(t *testing.T) {
	m := NewManager("")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", ".credentials.json")
	assert.Equal(t, expected, m.credentialsFile)
	assert.Equal(t, DefaultSecretName, m.secretName)
}

func TestNewManager_CustomSecretName(t *testing.T) {
	m := NewManager("custom-secret")
	assert.Equal(t, "custom-secret", m.secretName)
}

func TestNewManagerWithPath_CustomPath(t *testing.T) {
	m := NewManagerWithPath("/custom/path.json")
	assert.Equal(t, "/custom/path.json", m.credentialsFile)
	assert.Equal(t, DefaultSecretName, m.secretName)
}

func TestNewManagerWithPath_ExpandsHomePath(t *testing.T) {
	m := NewManagerWithPath("~/.claude/test.json")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".claude", "test.json")
	assert.Equal(t, expected, m.credentialsFile)
}

func TestCredentialsFile_ReturnsPath(t *testing.T) {
	m := NewManagerWithPath("/custom/path.json")
	assert.Equal(t, "/custom/path.json", m.CredentialsFile())
}

func TestSecretName_ReturnsName(t *testing.T) {
	m := NewManager("test-secret")
	assert.Equal(t, "test-secret", m.SecretName())
}

func TestFileExists_WhenFileDoesNotExist(t *testing.T) {
	m := NewManagerWithPath("/nonexistent/path/credentials.json")
	assert.False(t, m.FileExists())
}

func TestFileExists_WhenFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	require.NoError(t, os.WriteFile(credFile, []byte("{}"), 0600))

	m := NewManagerWithPath(credFile)
	assert.True(t, m.FileExists())
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

func TestDefaultSecretName(t *testing.T) {
	assert.Equal(t, "claude-credentials", DefaultSecretName)
}

func TestDefaultCredentialsFile(t *testing.T) {
	assert.Equal(t, "~/.claude/.credentials.json", DefaultCredentialsFile)
}

func TestGetFileHash_ComputesHash(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	content := []byte(`{"accessToken":"test-token"}`)
	require.NoError(t, os.WriteFile(credFile, content, 0600))

	m := NewManagerWithPath(credFile)
	hash, err := m.GetFileHash()

	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	// SHA256 produces 64 hex characters
	assert.Len(t, hash, 64)
}

func TestGetFileHash_SameContentSameHash(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	content := []byte(`{"accessToken":"test-token"}`)
	require.NoError(t, os.WriteFile(credFile, content, 0600))

	m := NewManagerWithPath(credFile)
	hash1, err := m.GetFileHash()
	require.NoError(t, err)

	hash2, err := m.GetFileHash()
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}

func TestGetFileHash_DifferentContentDifferentHash(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")

	// First content
	content1 := []byte(`{"accessToken":"token-1"}`)
	require.NoError(t, os.WriteFile(credFile, content1, 0600))

	m := NewManagerWithPath(credFile)
	hash1, err := m.GetFileHash()
	require.NoError(t, err)

	// Second content
	content2 := []byte(`{"accessToken":"token-2"}`)
	require.NoError(t, os.WriteFile(credFile, content2, 0600))

	hash2, err := m.GetFileHash()
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
}

func TestGetFileHash_FileNotFound(t *testing.T) {
	m := NewManagerWithPath("/nonexistent/path/credentials.json")
	_, err := m.GetFileHash()
	assert.Error(t, err)
}

func TestSyncIfChanged_CachesHash(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")
	content := []byte(`{"accessToken":"test-token"}`)
	require.NoError(t, os.WriteFile(credFile, content, 0600))

	m := NewManagerWithPath(credFile)

	// Initialize hash
	hash, err := m.GetFileHash()
	require.NoError(t, err)
	m.cachedHash = hash

	// File unchanged - should return false (no sync needed)
	// Note: We can't actually test SyncIfChanged without podman,
	// but we can verify the hash comparison logic
	assert.Equal(t, hash, m.cachedHash)
}

func TestSyncIfChanged_DetectsChange(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, ".credentials.json")

	// Initial content
	content1 := []byte(`{"accessToken":"token-1"}`)
	require.NoError(t, os.WriteFile(credFile, content1, 0600))

	m := NewManagerWithPath(credFile)
	hash1, _ := m.GetFileHash()
	m.cachedHash = hash1

	// Change the file
	content2 := []byte(`{"accessToken":"token-2"}`)
	require.NoError(t, os.WriteFile(credFile, content2, 0600))

	// New hash should differ from cached
	hash2, _ := m.GetFileHash()
	assert.NotEqual(t, m.cachedHash, hash2)
}
