package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"stromboli/internal/history"
)

// SessionHistoryHandler handles session message history requests
type SessionHistoryHandler struct {
	reader *history.Reader
}

// NewSessionHistoryHandler creates a new session history handler
func NewSessionHistoryHandler(sessionsDir string) *SessionHistoryHandler {
	return &SessionHistoryHandler{
		reader: history.NewReader(sessionsDir),
	}
}

// Reader returns the underlying history reader so other handlers (like /run)
// can aggregate session data without re-creating the reader.
func (h *SessionHistoryHandler) Reader() *history.Reader {
	return h.reader
}

// SessionMessagesResponse is the response for listing session messages
// @Description Paginated list of session messages
type SessionMessagesResponse struct {
	Messages []history.Message `json:"messages"`
	Total    int               `json:"total"`
	Offset   int               `json:"offset"`
	Limit    int               `json:"limit"`
	HasMore  bool              `json:"has_more"`
}

// SessionMessageResponse is the response for getting a single message
// @Description A single message from session history
type SessionMessageResponse struct {
	Message *history.Message `json:"message,omitempty"`
	Error   string           `json:"error,omitempty"`
}

// ListMessages returns paginated messages for a session
// @Summary List session messages
// @Description Returns paginated conversation history for a session including all messages, tool calls, and results
// @Tags sessions
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param offset query int false "Offset for pagination (default: 0)"
// @Param limit query int false "Number of messages to return (default: 50, max: 200)"
// @Success 200 {object} SessionMessagesResponse
// @Failure 400 {object} SessionMessagesResponse "Invalid parameters"
// @Failure 404 {object} SessionMessagesResponse "Session not found"
// @Failure 500 {object} SessionMessagesResponse "Internal error"
// @Router /sessions/{id}/messages [get]
func (h *SessionHistoryHandler) ListMessages(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ID required"})
		return
	}

	// Parse pagination parameters
	offset := 0
	limit := 50

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	result, err := h.reader.ListMessages(sessionID, offset, limit)
	if err != nil {
		// Check if it's a "not found" error
		if err.Error() == "session history not found: "+sessionID {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionMessagesResponse{
		Messages: result.Messages,
		Total:    result.Total,
		Offset:   result.Offset,
		Limit:    result.Limit,
		HasMore:  result.HasMore,
	})
}

// GetMessage returns a specific message by UUID
// @Summary Get session message
// @Description Returns a specific message from session history by UUID, including full content, tool calls, and results
// @Tags sessions
// @Produce json
// @Param id path string true "Session ID (UUID)"
// @Param message_id path string true "Message UUID"
// @Success 200 {object} SessionMessageResponse
// @Failure 400 {object} SessionMessageResponse "Invalid parameters"
// @Failure 404 {object} SessionMessageResponse "Message not found"
// @Failure 500 {object} SessionMessageResponse "Internal error"
// @Router /sessions/{id}/messages/{message_id} [get]
func (h *SessionHistoryHandler) GetMessage(c *gin.Context) {
	sessionID := c.Param("id")
	messageID := c.Param("message_id")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ID required"})
		return
	}
	if messageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message ID required"})
		return
	}

	msg, err := h.reader.GetMessage(sessionID, messageID)
	if err != nil {
		if err.Error() == "message not found: "+messageID {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "session history not found: "+sessionID {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SessionMessageResponse{
		Message: msg,
	})
}
