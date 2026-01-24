package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	reader := NewReader("/tmp/sessions")
	assert.NotNil(t, reader)
	assert.Equal(t, "/tmp/sessions", reader.sessionsDir)
}

func TestReader_ListMessages_SessionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)

	_, err := reader.ListMessages("nonexistent-session", 0, 50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session history not found")
}

func TestReader_GetMessage_SessionNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)

	_, err := reader.GetMessage("nonexistent-session", "some-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session history not found")
}

func TestReader_ListMessages_WithValidSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-123"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create a JSONL file with test data
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	testData := `{"type":"user","uuid":"uuid-1","parentUuid":null,"sessionId":"test-session-123","timestamp":"2026-01-24T10:00:00.000Z","cwd":"/workspace","version":"2.1.19","message":{"role":"user","content":"Hello"}}
{"type":"assistant","uuid":"uuid-2","parentUuid":"uuid-1","sessionId":"test-session-123","timestamp":"2026-01-24T10:00:01.000Z","cwd":"/workspace","version":"2.1.19","message":{"role":"assistant","model":"claude-opus-4-5-20251101","id":"msg-123","content":[{"type":"text","text":"Hi there!"}]}}
{"type":"user","uuid":"uuid-3","parentUuid":"uuid-2","sessionId":"test-session-123","timestamp":"2026-01-24T10:00:02.000Z","cwd":"/workspace","version":"2.1.19","message":{"role":"user","content":"How are you?"}}
`
	require.NoError(t, os.WriteFile(jsonlFile, []byte(testData), 0644))

	reader := NewReader(tmpDir)

	// Test listing all messages
	result, err := reader.ListMessages(sessionID, 0, 50)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 3, len(result.Messages))
	assert.Equal(t, 0, result.Offset)
	assert.Equal(t, 50, result.Limit)
	assert.False(t, result.HasMore)

	// Check first message
	assert.Equal(t, "uuid-1", result.Messages[0].UUID)
	assert.Equal(t, MessageTypeUser, result.Messages[0].Type)
	assert.Equal(t, "user", result.Messages[0].Content.Role)

	// Check second message (assistant)
	assert.Equal(t, "uuid-2", result.Messages[1].UUID)
	assert.Equal(t, MessageTypeAssistant, result.Messages[1].Type)
	assert.Equal(t, "claude-opus-4-5-20251101", result.Messages[1].Content.Model)
	assert.Equal(t, "Hi there!", result.Messages[1].Content.Content[0].Text)
}

func TestReader_ListMessages_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-456"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create a JSONL file with multiple messages
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	testData := `{"type":"user","uuid":"uuid-1","sessionId":"test-session-456","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"Msg 1"}}
{"type":"user","uuid":"uuid-2","sessionId":"test-session-456","timestamp":"2026-01-24T10:00:01.000Z","message":{"role":"user","content":"Msg 2"}}
{"type":"user","uuid":"uuid-3","sessionId":"test-session-456","timestamp":"2026-01-24T10:00:02.000Z","message":{"role":"user","content":"Msg 3"}}
{"type":"user","uuid":"uuid-4","sessionId":"test-session-456","timestamp":"2026-01-24T10:00:03.000Z","message":{"role":"user","content":"Msg 4"}}
{"type":"user","uuid":"uuid-5","sessionId":"test-session-456","timestamp":"2026-01-24T10:00:04.000Z","message":{"role":"user","content":"Msg 5"}}
`
	require.NoError(t, os.WriteFile(jsonlFile, []byte(testData), 0644))

	reader := NewReader(tmpDir)

	// Test pagination - first page
	result, err := reader.ListMessages(sessionID, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 2, len(result.Messages))
	assert.Equal(t, "uuid-1", result.Messages[0].UUID)
	assert.Equal(t, "uuid-2", result.Messages[1].UUID)
	assert.True(t, result.HasMore)

	// Test pagination - second page
	result, err = reader.ListMessages(sessionID, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 2, len(result.Messages))
	assert.Equal(t, "uuid-3", result.Messages[0].UUID)
	assert.Equal(t, "uuid-4", result.Messages[1].UUID)
	assert.True(t, result.HasMore)

	// Test pagination - last page
	result, err = reader.ListMessages(sessionID, 4, 2)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 1, len(result.Messages))
	assert.Equal(t, "uuid-5", result.Messages[0].UUID)
	assert.False(t, result.HasMore)
}

func TestReader_GetMessage_Found(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-789"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create a JSONL file
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	testData := `{"type":"user","uuid":"target-uuid","sessionId":"test-session-789","timestamp":"2026-01-24T10:00:00.000Z","cwd":"/workspace","message":{"role":"user","content":"Find me!"}}
`
	require.NoError(t, os.WriteFile(jsonlFile, []byte(testData), 0644))

	reader := NewReader(tmpDir)

	msg, err := reader.GetMessage(sessionID, "target-uuid")
	require.NoError(t, err)
	assert.Equal(t, "target-uuid", msg.UUID)
	assert.Equal(t, "/workspace", msg.Cwd)
}

func TestReader_GetMessage_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-abc"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create an empty JSONL file
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(jsonlFile, []byte(""), 0644))

	reader := NewReader(tmpDir)

	_, err := reader.GetMessage(sessionID, "nonexistent-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message not found")
}

func TestReader_ListMessages_WithToolUse(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-tools"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create a JSONL file with tool use
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	testData := `{"type":"assistant","uuid":"tool-uuid","sessionId":"test-session-tools","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"assistant","model":"claude-opus-4-5-20251101","content":[{"type":"tool_use","id":"toolu_123","name":"Bash","input":{"command":"ls -la"}}]}}
{"type":"user","uuid":"result-uuid","parentUuid":"tool-uuid","sessionId":"test-session-tools","timestamp":"2026-01-24T10:00:01.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"file1.txt\nfile2.txt"}]},"toolUseResult":{"stdout":"file1.txt\nfile2.txt","stderr":"","interrupted":false}}
`
	require.NoError(t, os.WriteFile(jsonlFile, []byte(testData), 0644))

	reader := NewReader(tmpDir)

	result, err := reader.ListMessages(sessionID, 0, 50)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)

	// Check tool use message
	toolMsg := result.Messages[0]
	assert.Equal(t, "tool-uuid", toolMsg.UUID)
	assert.Equal(t, "tool_use", toolMsg.Content.Content[0].Type)
	assert.Equal(t, "Bash", toolMsg.Content.Content[0].Name)
	assert.Equal(t, "ls -la", toolMsg.Content.Content[0].Input["command"])

	// Check tool result message
	resultMsg := result.Messages[1]
	assert.Equal(t, "result-uuid", resultMsg.UUID)
	assert.NotNil(t, resultMsg.ToolResult)
	assert.Equal(t, "file1.txt\nfile2.txt", resultMsg.ToolResult.Stdout)
}

func TestReader_ListMessages_LimitEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)
	sessionID := "test-limit"

	// Create session directory structure
	sessionDir := filepath.Join(tmpDir, sessionID, ".claude", "projects", "-workspace")
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create a JSONL file
	jsonlFile := filepath.Join(sessionDir, sessionID+".jsonl")
	require.NoError(t, os.WriteFile(jsonlFile, []byte(`{"type":"user","uuid":"uuid-1","sessionId":"test-limit","timestamp":"2026-01-24T10:00:00.000Z","message":{"role":"user","content":"Test"}}`), 0644))

	// Test that limit is capped at 200
	result, err := reader.ListMessages(sessionID, 0, 500)
	require.NoError(t, err)
	assert.Equal(t, 200, result.Limit)

	// Test that negative offset becomes 0
	result, err = reader.ListMessages(sessionID, -5, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Offset)

	// Test that zero/negative limit becomes 50 (default)
	result, err = reader.ListMessages(sessionID, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, result.Limit)
}
