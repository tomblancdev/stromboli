package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_EmptyPath(t *testing.T) {
	v := NewValidator([]string{"/allowed"})
	result, err := v.Validate("")
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestValidate_NoAllowlist(t *testing.T) {
	v := NewValidator([]string{})

	// Any path should be allowed when no allowlist is configured
	result, err := v.Validate("/some/random/path")
	assert.NoError(t, err)
	assert.Equal(t, "/some/random/path", result)

	result, err = v.Validate("/etc/passwd")
	assert.NoError(t, err)
	assert.Equal(t, "/etc/passwd", result)
}

func TestValidate_PathInAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewValidator([]string{tmpDir})

	result, err := v.Validate(tmpDir)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir, result)
}

func TestValidate_PathNotInAllowlist(t *testing.T) {
	v := NewValidator([]string{"/allowed/path"})

	_, err := v.Validate("/etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in allowed directories")
}

func TestValidate_SubdirectoryOfAllowedPath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "project", "src")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	v := NewValidator([]string{tmpDir})

	result, err := v.Validate(subDir)
	assert.NoError(t, err)
	assert.Equal(t, subDir, result)
}

func TestValidate_PathTraversalAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewValidator([]string{tmpDir})

	tests := []struct {
		name string
		path string
	}{
		{
			name: "basic traversal",
			path: tmpDir + "/../etc/passwd",
		},
		{
			name: "double traversal",
			path: tmpDir + "/../../etc/passwd",
		},
		{
			name: "traversal with subdirectory",
			path: tmpDir + "/subdir/../../../etc/passwd",
		},
		{
			name: "traversal to root",
			path: tmpDir + "/../../../../../../../etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v.Validate(tt.path)
			assert.Error(t, err, "path traversal should be blocked: %s", tt.path)
			assert.Contains(t, err.Error(), "not in allowed directories")
		})
	}
}

func TestValidate_RelativePathsResolved(t *testing.T) {
	// Get current working directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Create a temp directory and add it to allowlist
	tmpDir := t.TempDir()
	v := NewValidator([]string{tmpDir})

	// Test that relative path starting from cwd is resolved correctly
	// and rejected if not under allowed path
	_, err = v.Validate("./somepath")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in allowed directories")

	// Now test with cwd in allowlist
	v2 := NewValidator([]string{cwd})
	result, err := v2.Validate(".")
	assert.NoError(t, err)
	assert.Equal(t, cwd, result)
}

func TestValidate_MultipleAllowedPaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()
	v := NewValidator([]string{tmpDir1, tmpDir2})

	// First allowed path should work
	result, err := v.Validate(tmpDir1)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir1, result)

	// Second allowed path should work
	result, err = v.Validate(tmpDir2)
	assert.NoError(t, err)
	assert.Equal(t, tmpDir2, result)

	// Subdirectory of second allowed path should work
	subDir := filepath.Join(tmpDir2, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	result, err = v.Validate(subDir)
	assert.NoError(t, err)
	assert.Equal(t, subDir, result)

	// Path not in either should fail
	_, err = v.Validate("/etc/passwd")
	assert.Error(t, err)
}

func TestValidate_PartialPathMatch(t *testing.T) {
	// Ensure /allowed doesn't match /allowed-other
	v := NewValidator([]string{"/tmp/allowed"})

	// This should fail because /tmp/allowed-other is not under /tmp/allowed
	_, err := v.Validate("/tmp/allowed-other")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in allowed directories")
}

func TestValidate_CleanedPath(t *testing.T) {
	tmpDir := t.TempDir()
	v := NewValidator([]string{tmpDir})

	// Path with extra slashes should be cleaned
	messyPath := tmpDir + "//subdir///file"
	result, err := v.Validate(messyPath)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "subdir", "file"), result)
}

func TestIsConfigured(t *testing.T) {
	v1 := NewValidator([]string{})
	assert.False(t, v1.IsConfigured())

	v2 := NewValidator(nil)
	assert.False(t, v2.IsConfigured())

	v3 := NewValidator([]string{"/allowed"})
	assert.True(t, v3.IsConfigured())
}

func TestNewValidator(t *testing.T) {
	v := NewValidator([]string{"/path1", "/path2"})
	assert.NotNil(t, v)
	assert.Equal(t, []string{"/path1", "/path2"}, v.allowedPaths)
}
