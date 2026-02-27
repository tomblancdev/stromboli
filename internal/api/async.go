package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"stromboli/internal/job"
	"stromboli/internal/runner"
	"stromboli/internal/session"
	"stromboli/internal/webhook"
)

// AsyncRunResponse represents the response from starting an async run
// @Description Response from starting an async Claude execution
type AsyncRunResponse struct {
	JobID     string `json:"job_id" example:"job-abc123def456"`
	SessionID string `json:"session_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// JobResponse represents a job status response
// @Description Job status and result
type JobResponse struct {
	ID               string          `json:"id" example:"job-abc123def456"`
	Status           job.Status      `json:"status" example:"running"`
	Output           string          `json:"output,omitempty" example:"Hello!"`
	StructuredOutput json.RawMessage `json:"structured_output,omitempty" swaggertype:"object"`
	Error            string          `json:"error,omitempty"`
	SessionID        string          `json:"session_id,omitempty" example:"sess-abc123def456"`
	CrashInfo        *job.CrashInfo  `json:"crash_info,omitempty"`
	CreatedAt        string          `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt        string          `json:"updated_at" example:"2024-01-15T10:31:00Z"`
}

// JobListResponse represents a list of jobs
// @Description List of async jobs
type JobListResponse struct {
	Jobs []*JobResponse `json:"jobs"`
}

// runAsync starts Claude execution asynchronously
// @Summary Run Claude async
// @Description Starts Claude Code execution asynchronously and returns a job ID
// @Tags execution
// @Accept json
// @Produce json
// @Param request body RunRequest true "Run request"
// @Success 202 {object} AsyncRunResponse
// @Failure 400 {object} RunResponse "Invalid request"
// @Failure 503 {object} RunResponse "Claude not configured"
// @Router /run/async [post]
func (s *Server) runAsync(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		handleBadRequest(c, err)
		return
	}

	if !validateClaudeConfigured(c, s.claudeClient) {
		return
	}

	jobID := generateJobID()

	// Pre-generate session_id so it can be returned immediately for real-time tracking.
	// If the caller provided one, use it; otherwise generate a new UUID.
	sessionID := req.Claude.SessionID
	if sessionID == "" {
		sessionID = session.GenerateID()
		req.Claude.SessionID = sessionID
	}

	s.jobMgr.Create(jobID)
	s.jobMgr.Update(jobID, job.StatusRunning, "", "", sessionID)

	runnerReq := buildRunnerRequest(req)
	webhookURL := req.WebhookURL

	// Start async execution
	// Use background context since the HTTP request context will be cancelled
	s.runner.RunAsync(context.Background(), runnerReq, jobID, func(result *runner.Result, err error) {
		var status job.Status
		var output, errMsg, sessionID string

		if err != nil {
			status = job.StatusFailed
			errMsg = err.Error()
		} else if result.CrashInfo != nil {
			// Claude crashed but we have partial results
			s.jobMgr.UpdateWithCrash(jobID, result.CrashInfo, result.Output, result.SessionID)
			status = job.StatusCrashed
			output = result.Output
			sessionID = result.SessionID

			slog.Warn("Claude execution crashed",
				"job_id", jobID,
				"session_id", sessionID,
				"exit_code", result.CrashInfo.ExitCode,
				"signal", result.CrashInfo.Signal,
				"task_completed", result.CrashInfo.TaskCompleted)
		} else {
			status = job.StatusCompleted
			output = result.Output
			sessionID = result.SessionID
		}

		// Only update via normal path if not crashed (crash already updated)
		if status != job.StatusCrashed {
			s.jobMgr.Update(jobID, status, output, errMsg, sessionID)
		}

		// Send webhook notification if URL provided
		if webhookURL != "" {
			notifier := webhook.NewNotifier()
			payload := webhook.JobResult{
				JobID:     jobID,
				Status:    string(status),
				Output:    output,
				Error:     errMsg,
				SessionID: sessionID,
			}

			// Send webhook in background, don't block on failure
			go func() {
				if err := notifier.Notify(webhookURL, payload); err != nil {
					slog.Warn("Failed to send webhook notification",
						"job_id", jobID,
						"webhook_url", webhookURL,
						"error", err)
				}
			}()
		}
	})

	c.JSON(http.StatusAccepted, AsyncRunResponse{
		JobID:     jobID,
		SessionID: sessionID,
	})
}

// getJob retrieves a job's status and result
// @Summary Get job status
// @Description Returns the status and result of an async job
// @Tags jobs
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} JobResponse
// @Failure 404 {object} RunResponse "Job not found"
// @Router /jobs/{id} [get]
func (s *Server) getJob(c *gin.Context) {
	jobID := c.Param("id")

	j, ok := s.jobMgr.Get(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, RunResponse{
			Status: "error",
			Error:  "job not found",
		})
		return
	}

	c.JSON(http.StatusOK, JobResponse{
		ID:               j.ID,
		Status:           j.Status,
		Output:           j.Output,
		StructuredOutput: extractStructuredOutput(j.Output),
		Error:            j.Error,
		SessionID:        j.SessionID,
		CrashInfo:        j.CrashInfo,
		CreatedAt:        j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// listJobs returns all jobs
// @Summary List jobs
// @Description Returns all async jobs
// @Tags jobs
// @Produce json
// @Success 200 {object} JobListResponse
// @Router /jobs [get]
func (s *Server) listJobs(c *gin.Context) {
	jobs := s.jobMgr.List()

	resp := JobListResponse{
		Jobs: make([]*JobResponse, 0, len(jobs)),
	}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, &JobResponse{
			ID:               j.ID,
			Status:           j.Status,
			Output:           j.Output,
			StructuredOutput: extractStructuredOutput(j.Output),
			Error:            j.Error,
			SessionID:        j.SessionID,
			CrashInfo:        j.CrashInfo,
			CreatedAt:        j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, resp)
}

// cancelJob cancels a pending or running job
// @Summary Cancel job
// @Description Cancels a pending or running job
// @Tags jobs
// @Produce json
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} RunResponse "Job not found"
// @Failure 409 {object} RunResponse "Job cannot be cancelled"
// @Router /jobs/{id} [delete]
func (s *Server) cancelJob(c *gin.Context) {
	jobID := c.Param("id")

	// Check if job exists
	_, ok := s.jobMgr.Get(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, RunResponse{
			Status: "error",
			Error:  "job not found",
		})
		return
	}

	// Try to cancel
	cancelled := s.jobMgr.Cancel(jobID)
	if !cancelled {
		c.JSON(http.StatusConflict, RunResponse{
			Status: "error",
			Error:  "cannot cancel job (already completed, failed, or cancelled)",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cancelled": true,
		"job_id":    jobID,
	})
}

// generateJobID creates a unique job ID
func generateJobID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes) // Error ignored: crypto/rand.Read always succeeds on supported platforms
	return "job-" + hex.EncodeToString(bytes)
}
