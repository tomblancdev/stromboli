package session

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	strerrors "github.com/tomblanc/stromboli/internal/errors"
)

// TestGenerateID_Format verifies that generated IDs follow UUID v4 format
func TestGenerateID_Format(t *testing.T) {
	id := GenerateID()

	// UUID v4 format: 8-4-4-4-12 hexadecimal characters
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidPattern, id, "generated ID should match UUID v4 format")

	// Verify UUID version 4 (bits 12-15 of time_hi_and_version should be 0100)
	assert.Equal(t, "4", string(id[14]), "should be UUID version 4")

	// Verify UUID variant (bits 6-7 of clock_seq_hi_and_reserved should be 10)
	variantChar := id[19]
	assert.Contains(t, "89ab", string(variantChar), "should have correct UUID variant")
}

// TestGenerateID_Uniqueness ensures no collisions in 1000 generated IDs
func TestGenerateID_Uniqueness(t *testing.T) {
	iterations := 1000
	ids := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		id := GenerateID()
		assert.False(t, ids[id], "generated ID should be unique: %s", id)
		ids[id] = true
	}

	assert.Len(t, ids, iterations, "all generated IDs should be unique")
}

// TestNewManager verifies manager creation with base directory
func TestNewManager(t *testing.T) {
	baseDir := "/tmp/sessions"
	m := NewManager(baseDir)

	assert.NotNil(t, m, "manager should not be nil")
	assert.Equal(t, baseDir, m.sessionsDir, "sessions directory should match")
}

// TestManager_Create_Success creates a session directory successfully
func TestManager_Create_Success(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	sessionID, sessionPath, err := m.Create("")
	require.NoError(t, err, "should create session successfully")
	assert.NotEmpty(t, sessionID, "session ID should not be empty")
	assert.NotEmpty(t, sessionPath, "session path should not be empty")

	// Verify directory was created
	info, err := os.Stat(sessionPath)
	require.NoError(t, err, "session directory should exist")
	assert.True(t, info.IsDir(), "session path should be a directory")

	// Verify permissions (0700)
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm(), "session directory should have 0700 permissions")

	// Verify path is absolute
	assert.True(t, filepath.IsAbs(sessionPath), "session path should be absolute")

	// Verify path is under base directory
	assert.Contains(t, sessionPath, baseDir, "session path should be under base directory")
}

// TestManager_Create_WithExistingID creates session with provided ID
func TestManager_Create_WithExistingID(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	providedID := "my-custom-session-id"
	sessionID, sessionPath, err := m.Create(providedID)
	require.NoError(t, err, "should create session with provided ID")

	assert.Equal(t, providedID, sessionID, "returned ID should match provided ID")
	assert.Contains(t, sessionPath, providedID, "session path should contain provided ID")

	// Verify directory exists
	_, err = os.Stat(sessionPath)
	assert.NoError(t, err, "session directory should exist")
}

// TestManager_Create_GeneratesID verifies ID auto-generation when empty
func TestManager_Create_GeneratesID(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	sessionID, _, err := m.Create("")
	require.NoError(t, err, "should generate session ID")

	// Verify generated ID is in UUID format
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	assert.Regexp(t, uuidPattern, sessionID, "auto-generated ID should be valid UUID")
}

// TestManager_Destroy_Success removes an existing session
func TestManager_Destroy_Success(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	// Create a session first
	sessionID, sessionPath, err := m.Create("")
	require.NoError(t, err, "should create session")

	// Verify it exists
	_, err = os.Stat(sessionPath)
	require.NoError(t, err, "session should exist before destroy")

	// Destroy the session
	err = m.Destroy(sessionID)
	assert.NoError(t, err, "should destroy session successfully")

	// Verify it no longer exists
	_, err = os.Stat(sessionPath)
	assert.True(t, os.IsNotExist(err), "session should not exist after destroy")
}

// TestManager_Destroy_NotFound returns error for non-existent session
func TestManager_Destroy_NotFound(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	nonExistentID := "does-not-exist"
	err := m.Destroy(nonExistentID)

	assert.Error(t, err, "should return error for non-existent session")
	assert.ErrorIs(t, err, strerrors.ErrSessionNotFound, "should be session not found error")
	assert.Contains(t, err.Error(), nonExistentID, "error should contain session ID")
}

// TestManager_Destroy_EmptyID returns error for empty session ID
func TestManager_Destroy_EmptyID(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	err := m.Destroy("")
	assert.Error(t, err, "should return error for empty session ID")
	assert.ErrorIs(t, err, strerrors.ErrSessionIDRequired, "should be session ID required error")
}

// TestManager_List_Empty returns empty list when no sessions exist
func TestManager_List_Empty(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	sessions, err := m.List()
	assert.NoError(t, err, "should list sessions successfully")
	assert.Empty(t, sessions, "should return empty list when no sessions exist")
}

// TestManager_List_WithSessions returns all session IDs
func TestManager_List_WithSessions(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	// Create multiple sessions
	expectedIDs := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		sessionID, _, err := m.Create("")
		require.NoError(t, err, "should create session")
		expectedIDs = append(expectedIDs, sessionID)
	}

	// List sessions
	sessions, err := m.List()
	assert.NoError(t, err, "should list sessions successfully")
	assert.Len(t, sessions, 3, "should return correct number of sessions")

	// Verify all created sessions are in the list
	for _, expectedID := range expectedIDs {
		assert.Contains(t, sessions, expectedID, "list should contain session ID: %s", expectedID)
	}
}

// TestManager_List_NonExistentDirectory handles missing base directory
func TestManager_List_NonExistentDirectory(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "does-not-exist")
	m := NewManager(baseDir)

	sessions, err := m.List()
	assert.NoError(t, err, "should not error for non-existent directory")
	assert.Empty(t, sessions, "should return empty list for non-existent directory")
}

// TestManager_List_IgnoresFiles only returns directories, not files
func TestManager_List_IgnoresFiles(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	// Create a session directory
	sessionID, _, err := m.Create("")
	require.NoError(t, err, "should create session")

	// Create a file in base directory (should be ignored)
	filePath := filepath.Join(baseDir, "not-a-session.txt")
	err = os.WriteFile(filePath, []byte("test"), 0644)
	require.NoError(t, err, "should create test file")

	// List sessions
	sessions, err := m.List()
	assert.NoError(t, err, "should list sessions successfully")
	assert.Len(t, sessions, 1, "should only return directories")
	assert.Equal(t, sessionID, sessions[0], "should return the session directory only")
}

// TestManager_PathTraversal_DotDot blocks ../ path traversal attacks
func TestManager_PathTraversal_DotDot(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	tests := []struct {
		name      string
		sessionID string
	}{
		{"basic traversal", "../etc"},
		{"double traversal", "../../etc"},
		{"traversal in middle", "foo/../../../etc"},
		{"traversal at end", "foo/.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := m.Create(tt.sessionID)
			assert.Error(t, err, "should block path traversal: %s", tt.sessionID)
			assert.ErrorIs(t, err, strerrors.ErrInvalidSessionID, "should be invalid session ID error")
		})
	}
}

// TestManager_PathTraversal_Slash blocks absolute path attacks
func TestManager_PathTraversal_Slash(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	tests := []struct {
		name      string
		sessionID string
	}{
		{"absolute path", "/etc/passwd"},
		{"absolute path no leading slash", "etc/passwd"},
		{"slash in middle", "foo/bar/baz"},
		{"trailing slash", "foo/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := m.Create(tt.sessionID)
			assert.Error(t, err, "should block slash in session ID: %s", tt.sessionID)
			assert.ErrorIs(t, err, strerrors.ErrInvalidSessionID, "should be invalid session ID error")
		})
	}
}

// TestManager_PathTraversal_DotSlash blocks ./ path attacks
func TestManager_PathTraversal_DotSlash(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	sessionID := "./session"
	_, _, err := m.Create(sessionID)
	assert.Error(t, err, "should block ./ in session ID")
	assert.ErrorIs(t, err, strerrors.ErrInvalidSessionID, "should be invalid session ID error")
}

// TestManager_PathTraversal_Destroy verifies Destroy also blocks traversal
func TestManager_PathTraversal_Destroy(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	tests := []struct {
		name      string
		sessionID string
	}{
		{"slash", "foo/bar"},
		{"double dot", "../etc"},
		{"absolute", "/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Destroy(tt.sessionID)
			assert.Error(t, err, "should block path traversal in Destroy: %s", tt.sessionID)
			assert.ErrorIs(t, err, strerrors.ErrInvalidSessionID, "should be invalid session ID error")
		})
	}
}

// TestManager_ConcurrentCreate tests thread-safe concurrent session creation
func TestManager_ConcurrentCreate(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Channel to collect session IDs
	sessionIDs := make(chan string, concurrency)
	errors := make(chan error, concurrency)

	// Create sessions concurrently
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			sessionID, _, err := m.Create("")
			if err != nil {
				errors <- err
				return
			}
			sessionIDs <- sessionID
		}()
	}

	// Wait for all goroutines
	wg.Wait()
	close(sessionIDs)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent create failed: %v", err)
	}

	// Verify all sessions were created with unique IDs
	ids := make(map[string]bool)
	for id := range sessionIDs {
		assert.False(t, ids[id], "concurrent creates should produce unique IDs")
		ids[id] = true

		// Verify directory exists
		sessionPath := filepath.Join(baseDir, id)
		_, err := os.Stat(sessionPath)
		assert.NoError(t, err, "session directory should exist: %s", id)
	}

	assert.Len(t, ids, concurrency, "should create correct number of sessions")
}

// TestManager_ConcurrentDestroy tests thread-safe concurrent session deletion
func TestManager_ConcurrentDestroy(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	// Create sessions first
	concurrency := 10
	sessionIDs := make([]string, 0, concurrency)
	for i := 0; i < concurrency; i++ {
		sessionID, _, err := m.Create("")
		require.NoError(t, err, "should create session")
		sessionIDs = append(sessionIDs, sessionID)
	}

	// Destroy sessions concurrently
	var wg sync.WaitGroup
	wg.Add(concurrency)
	errors := make(chan error, concurrency)

	for _, id := range sessionIDs {
		sessionID := id // capture for goroutine
		go func() {
			defer wg.Done()
			if err := m.Destroy(sessionID); err != nil {
				errors <- err
			}
		}()
	}

	// Wait for all goroutines
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent destroy failed: %v", err)
	}

	// Verify all sessions were destroyed
	sessions, err := m.List()
	assert.NoError(t, err, "should list sessions")
	assert.Empty(t, sessions, "all sessions should be destroyed")
}

// TestManager_Create_PreservesExistingDirectory ensures Create is idempotent
func TestManager_Create_PreservesExistingDirectory(t *testing.T) {
	baseDir := t.TempDir()
	m := NewManager(baseDir)

	sessionID := "test-session"

	// Create session first time
	id1, path1, err := m.Create(sessionID)
	require.NoError(t, err, "first create should succeed")

	// Write a file in the session directory
	testFile := filepath.Join(path1, "test.txt")
	testContent := []byte("test data")
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err, "should write test file")

	// Create session second time (should be idempotent)
	id2, path2, err := m.Create(sessionID)
	require.NoError(t, err, "second create should succeed")

	assert.Equal(t, id1, id2, "session IDs should match")
	assert.Equal(t, path1, path2, "session paths should match")

	// Verify the test file still exists
	data, err := os.ReadFile(testFile)
	require.NoError(t, err, "test file should still exist")
	assert.Equal(t, testContent, data, "test file content should be preserved")
}
