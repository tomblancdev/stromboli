package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	client := NewClient("claude-auth")
	assert.NotNil(t, client)
	assert.Equal(t, "claude-auth", client.volumeName)
}

func TestIsConfigured_WhenVolumeDoesNotExist_ReturnsFalse(t *testing.T) {
	client := NewClient("nonexistent-volume")
	configured, err := client.IsConfigured()
	assert.NoError(t, err)
	assert.False(t, configured)
}

func TestIsConfigured_WhenVolumeExistsWithCredentials_ReturnsTrue(t *testing.T) {
	client := NewClient("claude-auth-test")
	configured, err := client.IsConfigured()
	assert.NoError(t, err)
	assert.True(t, configured)
}

func TestIsConfigured_WhenVolumeExistsButConnectionFailed_ReturnsError(t *testing.T) {
	client := NewClient("claude-auth-invalid")
	configured, err := client.IsConfigured()
	assert.Error(t, err)
	assert.False(t, configured)
}

func TestLogin_Success(t *testing.T) {
	client := NewClient("claude-auth")
	err := client.Login()
	assert.NoError(t, err)
}

func TestLogout_Success(t *testing.T) {
	client := NewClient("claude-auth")
	err := client.Logout()
	assert.NoError(t, err)
}
