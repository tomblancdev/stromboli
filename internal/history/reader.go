package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Reader reads session history from JSONL files
type Reader struct {
	sessionsDir string
}

// NewReader creates a new history reader
func NewReader(sessionsDir string) *Reader {
	return &Reader{
		sessionsDir: sessionsDir,
	}
}

// rawMessage is the internal representation from Claude's JSONL format
type rawMessage struct {
	Type         string          `json:"type"`
	UUID         string          `json:"uuid"`
	ParentUUID   *string         `json:"parentUuid"`
	SessionID    string          `json:"sessionId"`
	Timestamp    string          `json:"timestamp"`
	Cwd          string          `json:"cwd"`
	GitBranch    string          `json:"gitBranch"`
	Version      string          `json:"version"`
	PermMode     string          `json:"permissionMode"`
	Message      json.RawMessage `json:"message"`
	ToolResult   *rawToolResult  `json:"toolUseResult"`
	Operation    string          `json:"operation"`
	IsSidechain  bool            `json:"isSidechain"`
}

type rawToolResult struct {
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Interrupted bool   `json:"interrupted"`
	IsImage     bool   `json:"isImage"`
}

type rawMessageContent struct {
	Role       string            `json:"role"`
	Model      string            `json:"model"`
	ID         string            `json:"id"`
	Content    json.RawMessage   `json:"content"`
	StopReason string            `json:"stop_reason"`
	Usage      *rawUsage         `json:"usage"`
}

type rawUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// findSessionFile locates the JSONL file for a session.
// Claude Code names the project folder based on the container's workdir
// (e.g. /workspace → -workspace, /project → -project), so we glob
// for any folder name under .claude/projects/.
func (r *Reader) findSessionFile(sessionID string) (string, error) {
	pattern := filepath.Join(r.sessionsDir, sessionID, ".claude", "projects", "*", sessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to search for session file: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("session history not found: %s", sessionID)
	}
	return matches[0], nil
}

// ListMessages returns a paginated list of messages for a session
func (r *Reader) ListMessages(sessionID string, offset, limit int) (*MessageList, error) {
	filePath, err := r.findSessionFile(sessionID)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	// Read all messages (we need total count anyway)
	var allMessages []Message
	scanner := bufio.NewScanner(file)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		msg, err := r.parseMessage(line)
		if err != nil {
			// Skip unparseable messages but log line number
			continue
		}

		// Skip system/queue messages for the API
		if msg.Type == MessageTypeSystem {
			continue
		}

		allMessages = append(allMessages, *msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file: %w", err)
	}

	total := len(allMessages)

	// Apply pagination
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 50 // Default limit
	}
	if limit > 200 {
		limit = 200 // Max limit
	}

	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	pagedMessages := allMessages[start:end]

	return &MessageList{
		Messages: pagedMessages,
		Total:    total,
		Offset:   offset,
		Limit:    limit,
		HasMore:  end < total,
	}, nil
}

// GetMessage returns a specific message by UUID
func (r *Reader) GetMessage(sessionID, messageUUID string) (*Message, error) {
	filePath, err := r.findSessionFile(sessionID)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		msg, err := r.parseMessage(line)
		if err != nil {
			continue
		}

		if msg.UUID == messageUUID {
			return msg, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file: %w", err)
	}

	return nil, fmt.Errorf("message not found: %s", messageUUID)
}

// AggregateUsage walks the session JSONL once and sums every assistant
// message's `usage` block. Returns the totals plus the most recently seen
// model identifier (used for cost estimation).
//
// Returns (nil, nil) if the session file exists but no assistant message has
// usage info — that's a normal in-flight state, not an error.
func (r *Reader) AggregateUsage(sessionID string) (*UsageSummary, error) {
	filePath, err := r.findSessionFile(sessionID)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var summary UsageSummary
	hasUsage := false

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		msg, err := r.parseMessage(line)
		if err != nil {
			continue
		}
		if msg.Type != MessageTypeAssistant || msg.Content.Usage == nil {
			continue
		}
		u := msg.Content.Usage
		summary.InputTokens += u.InputTokens
		summary.OutputTokens += u.OutputTokens
		summary.CacheCreationInputTokens += u.CacheCreationInputTokens
		summary.CacheReadInputTokens += u.CacheReadInputTokens
		if msg.Content.Model != "" {
			summary.Model = msg.Content.Model
		}
		hasUsage = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading session file: %w", err)
	}
	if !hasUsage {
		return nil, nil
	}

	summary.TotalTokens = summary.InputTokens + summary.OutputTokens +
		summary.CacheCreationInputTokens + summary.CacheReadInputTokens
	return &summary, nil
}

// parseMessage converts a raw JSONL line to a Message
func (r *Reader) parseMessage(line []byte) (*Message, error) {
	var raw rawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	// Skip queue operations
	if raw.Type == "queue-operation" {
		return &Message{Type: MessageTypeSystem}, nil
	}

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339, raw.Timestamp)
	if err != nil {
		ts = time.Now()
	}

	msg := &Message{
		UUID:           raw.UUID,
		ParentUUID:     raw.ParentUUID,
		Type:           MessageType(raw.Type),
		SessionID:      raw.SessionID,
		Timestamp:      ts,
		Cwd:            raw.Cwd,
		GitBranch:      raw.GitBranch,
		Version:        raw.Version,
		PermissionMode: raw.PermMode,
	}

	// Parse message content
	if len(raw.Message) > 0 {
		var content rawMessageContent
		if err := json.Unmarshal(raw.Message, &content); err == nil {
			msg.Content = MessageContent{
				Role:       content.Role,
				Model:      content.Model,
				MessageID:  content.ID,
				StopReason: content.StopReason,
			}

			// Parse content blocks
			if len(content.Content) > 0 {
				// Content can be string or array of blocks
				var strContent string
				if err := json.Unmarshal(content.Content, &strContent); err == nil {
					// Simple string content
					msg.Content.Content = []ContentBlock{{Type: "text", Text: strContent}}
				} else {
					// Array of content blocks
					var blocks []ContentBlock
					if err := json.Unmarshal(content.Content, &blocks); err == nil {
						msg.Content.Content = blocks
					}
				}
			}

			// Parse usage
			if content.Usage != nil {
				msg.Content.Usage = &Usage{
					InputTokens:              content.Usage.InputTokens,
					OutputTokens:             content.Usage.OutputTokens,
					CacheCreationInputTokens: content.Usage.CacheCreationInputTokens,
					CacheReadInputTokens:     content.Usage.CacheReadInputTokens,
				}
			}
		}
	}

	// Parse tool result
	if raw.ToolResult != nil {
		msg.ToolResult = &ToolResult{
			Stdout:      raw.ToolResult.Stdout,
			Stderr:      raw.ToolResult.Stderr,
			Interrupted: raw.ToolResult.Interrupted,
			IsImage:     raw.ToolResult.IsImage,
		}
	}

	return msg, nil
}
