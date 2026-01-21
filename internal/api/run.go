package api

// RunRequest represents a request to run Claude in a container
type RunRequest struct {
	Prompt    string `json:"prompt" binding:"required"`
	Workspace string `json:"workspace,omitempty"`
	Model     string `json:"model,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// RunResponse represents the response from a Claude run
type RunResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}
