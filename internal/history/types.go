package history

import "time"

// MessageType represents the type of message in the conversation
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "queue-operation"
)

// Message represents a conversation message from the session history
// @Description A message in the conversation history
type Message struct {
	// Unique identifier for this message
	UUID string `json:"uuid" example:"40adde19-546a-43e8-ad25-31ef4faa4112"`
	// Parent message UUID for threading
	ParentUUID *string `json:"parent_uuid,omitempty" example:"92242819-b7d1-48d4-b023-6134c3e9f63a"`
	// Message type: user, assistant, queue-operation
	Type MessageType `json:"type" example:"assistant"`
	// Session ID this message belongs to
	SessionID string `json:"session_id" example:"c7518652-f0ea-436e-9143-327085022abd"`
	// Timestamp when the message was created
	Timestamp time.Time `json:"timestamp" example:"2026-01-24T10:06:42.906Z"`
	// Working directory at time of message
	Cwd string `json:"cwd,omitempty" example:"/workspace"`
	// Git branch at time of message
	GitBranch string `json:"git_branch,omitempty" example:"main"`
	// Claude Code version
	Version string `json:"version,omitempty" example:"2.1.19"`
	// Permission mode active for this message
	PermissionMode string `json:"permission_mode,omitempty" example:"bypassPermissions"`
	// The actual message content
	Content MessageContent `json:"content"`
	// Tool use result (for tool_result messages)
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

// MessageContent contains the message payload
// @Description Message content with role and content blocks
type MessageContent struct {
	// Role: user or assistant
	Role string `json:"role" example:"assistant"`
	// Model used (for assistant messages)
	Model string `json:"model,omitempty" example:"claude-opus-4-5-20251101"`
	// Message ID from API
	MessageID string `json:"message_id,omitempty" example:"msg_017ETE4Wk32ZXAQJp3GXP1Bo"`
	// Content blocks (text, tool_use, tool_result)
	Content []ContentBlock `json:"content"`
	// Stop reason (for assistant messages)
	StopReason string `json:"stop_reason,omitempty" example:"end_turn"`
	// Token usage
	Usage *Usage `json:"usage,omitempty"`
}

// ContentBlock represents a single content block in a message
// @Description A content block (text, tool_use, or tool_result)
type ContentBlock struct {
	// Block type: text, tool_use, tool_result
	Type string `json:"type" example:"tool_use"`
	// Text content (for text blocks)
	Text string `json:"text,omitempty" example:"I'll help you with that."`
	// Tool use ID (for tool_use and tool_result)
	ID string `json:"id,omitempty" example:"toolu_01G5uAJ4YZ26yyJbXNnG2byM"`
	// Tool name (for tool_use)
	Name string `json:"name,omitempty" example:"Bash"`
	// Tool input (for tool_use)
	Input map[string]any `json:"input,omitempty"`
	// Tool use ID reference (for tool_result)
	ToolUseID string `json:"tool_use_id,omitempty" example:"toolu_01G5uAJ4YZ26yyJbXNnG2byM"`
	// Tool result content (for tool_result)
	Content string `json:"content,omitempty" example:"file1.txt\nfile2.txt"`
	// Whether tool execution errored
	IsError bool `json:"is_error,omitempty" example:"false"`
}

// ToolResult contains detailed tool execution results
// @Description Detailed result of a tool execution
type ToolResult struct {
	// Standard output
	Stdout string `json:"stdout,omitempty" example:"file1.txt\nfile2.txt"`
	// Standard error
	Stderr string `json:"stderr,omitempty" example:""`
	// Whether execution was interrupted
	Interrupted bool `json:"interrupted,omitempty" example:"false"`
	// Whether result is an image
	IsImage bool `json:"is_image,omitempty" example:"false"`
}

// Usage contains token usage information
// @Description Token usage statistics
type Usage struct {
	InputTokens              int `json:"input_tokens,omitempty" example:"150"`
	OutputTokens             int `json:"output_tokens,omitempty" example:"42"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty" example:"336"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty" example:"18121"`
}

// MessageList represents a paginated list of messages
// @Description Paginated list of conversation messages
type MessageList struct {
	// List of messages
	Messages []Message `json:"messages"`
	// Total number of messages in session
	Total int `json:"total"`
	// Current offset
	Offset int `json:"offset"`
	// Number of messages returned
	Limit int `json:"limit"`
	// Whether there are more messages
	HasMore bool `json:"has_more"`
}
