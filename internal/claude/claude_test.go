package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tomblanc/stromboli/internal/container"
)

func TestNewClient(t *testing.T) {
	client := NewClient("claude-auth")
	assert.NotNil(t, client)
	assert.Equal(t, "claude-auth", client.volumeName)
}

func TestIsConfigured_WhenVolumeDoesNotExist_ReturnsFalse(t *testing.T) {
	client := NewClient("nonexistent-volume-12345")
	configured, err := client.IsConfigured()
	assert.NoError(t, err)
	assert.False(t, configured)
}

func TestIsConfigured_WhenVolumeExists_ReturnsTrue(t *testing.T) {
	// Setup: create test volume
	manager := container.NewManager()
	volumeName := "test-claude-auth"
	err := manager.VolumeCreate(volumeName)
	require.NoError(t, err)
	defer manager.VolumeRemove(volumeName)

	// Test
	client := NewClient(volumeName)
	configured, err := client.IsConfigured()
	assert.NoError(t, err)
	assert.True(t, configured)
}

func TestLogin_CreatesVolume(t *testing.T) {
	// Skip in CI - needs TTY
	t.Skip("Login requires interactive TTY")

	volumeName := "test-login-volume"
	client := NewClient(volumeName)
	defer client.manager.VolumeRemove(volumeName)

	// Volume should not exist initially
	exists, _ := client.IsConfigured()
	assert.False(t, exists)

	// After Login attempt, volume should be created
	// (even if login fails due to no TTY)
	_ = client.Login()

	exists, _ = client.IsConfigured()
	assert.True(t, exists)
}

func TestLogout_NotImplemented(t *testing.T) {
	client := NewClient("claude-auth")
	err := client.Logout()
	assert.ErrorIs(t, err, ErrNotImplemented)
}
