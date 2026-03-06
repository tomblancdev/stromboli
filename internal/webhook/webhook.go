package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TokensUsed contains Claude token usage statistics
type TokensUsed struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// JobResult represents the result of a job to be sent to webhook
type JobResult struct {
	JobID           string      `json:"job_id"`
	Status          string      `json:"status"`
	Output          string      `json:"output,omitempty"`
	Error           string      `json:"error,omitempty"`
	SessionID       string      `json:"session_id,omitempty"`
	FilesChanged    []string    `json:"files_changed"`
	TokensUsed      *TokensUsed `json:"tokens_used"`
	DurationSeconds float64     `json:"duration_seconds"`
}

// Notifier sends webhook notifications
type Notifier struct {
	client *http.Client
}

// NewNotifier creates a new webhook notifier
func NewNotifier() *Notifier {
	return &Notifier{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Notify sends a webhook notification with retry logic
func (n *Notifier) Notify(url string, payload JobResult) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Try twice: initial attempt + one retry
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send webhook: %w", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return lastErr
}
