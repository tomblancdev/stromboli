package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tomblanc/stromboli/internal/job"
	"github.com/tomblanc/stromboli/internal/runner"
)

// AsyncRunResponse represents the response from starting an async run
// @Description Response from starting an async Claude execution
type AsyncRunResponse struct {
	JobID string `json:"job_id" example:"job-abc123def456"`
}

// JobResponse represents a job status response
// @Description Job status and result
type JobResponse struct {
	ID        string     `json:"id" example:"job-abc123def456"`
	Status    job.Status `json:"status" example:"running"`
	Output    string     `json:"output,omitempty" example:"Hello!"`
	Error     string     `json:"error,omitempty"`
	SessionID string     `json:"session_id,omitempty" example:"sess-abc123def456"`
	CreatedAt string     `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt string     `json:"updated_at" example:"2024-01-15T10:31:00Z"`
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
		c.JSON(http.StatusBadRequest, RunResponse{
			Status: "error",
			Error:  "Invalid request: " + err.Error(),
		})
		return
	}

	// Check if configured
	if !s.claudeClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, RunResponse{
			Status: "error",
			Error:  "Claude not configured. Run 'make claude-setup' first",
		})
		return
	}

	// Generate job ID and create job
	jobID := generateJobID()
	s.jobMgr.Create(jobID)
	s.jobMgr.Update(jobID, job.StatusRunning, "", "", "")

	// Build runner request
	runnerReq := runner.Request{
		Prompt:    req.Prompt,
		Workspace: req.Workspace,
		Claude:    req.Claude,
		Podman:    req.Podman,
	}

	// Start async execution
	// Use background context since the HTTP request context will be cancelled
	s.runner.RunAsync(context.Background(), runnerReq, jobID, func(result *runner.Result, err error) {
		if err != nil {
			s.jobMgr.Update(jobID, job.StatusFailed, "", err.Error(), "")
			return
		}
		s.jobMgr.Update(jobID, job.StatusCompleted, result.Output, "", result.SessionID)
	})

	c.JSON(http.StatusAccepted, AsyncRunResponse{
		JobID: jobID,
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
		ID:        j.ID,
		Status:    j.Status,
		Output:    j.Output,
		Error:     j.Error,
		SessionID: j.SessionID,
		CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
			ID:        j.ID,
			Status:    j.Status,
			Output:    j.Output,
			Error:     j.Error,
			SessionID: j.SessionID,
			CreatedAt: j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: j.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	c.JSON(http.StatusOK, resp)
}

// generateJobID creates a unique job ID
func generateJobID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "job-" + hex.EncodeToString(bytes)
}
