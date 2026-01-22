//go:build integration

package secrets

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoPodman(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available, skipping test")
	}
}

func TestSecretLifecycle(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()

	// Create a temp file for the secret
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "test-secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("test-token"), 0600))

	// Use a unique secret name for testing
	m := NewManager("stromboli-test-secret")

	// Cleanup any existing secret
	m.Remove(ctx)

	// Test Exists (should not exist initially)
	exists, err := m.Exists(ctx)
	require.NoError(t, err)
	assert.False(t, exists)

	// Test Create
	err = m.Create(ctx, secretFile)
	require.NoError(t, err)

	// Test Exists (should exist now)
	exists, err = m.Exists(ctx)
	require.NoError(t, err)
	assert.True(t, exists)

	// Test EnsureExists (should be a no-op)
	err = m.EnsureExists(ctx, secretFile)
	require.NoError(t, err)

	// Test Update
	require.NoError(t, os.WriteFile(secretFile, []byte("updated-token"), 0600))
	err = m.Update(ctx, secretFile)
	require.NoError(t, err)

	// Test Remove
	err = m.Remove(ctx)
	require.NoError(t, err)

	// Verify it's gone
	exists, err = m.Exists(ctx)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestEnsureExists_CreatesWhenMissing(t *testing.T) {
	skipIfNoPodman(t)
	ctx := context.Background()

	// Create a temp file for the secret
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "test-secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("test-token"), 0600))

	m := NewManager("stromboli-test-ensure")

	// Cleanup any existing secret
	m.Remove(ctx)

	// EnsureExists should create it
	err := m.EnsureExists(ctx, secretFile)
	require.NoError(t, err)

	exists, err := m.Exists(ctx)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup
	m.Remove(ctx)
}

func TestContextCancellation(t *testing.T) {
	skipIfNoPodman(t)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := NewManager("stromboli-test-cancel")

	_, err := m.Exists(ctx)
	assert.Error(t, err, "expected error with cancelled context")
}
