package job

import (
	"strings"
	"sync"
	"time"
)

// Status represents the current state of a job
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCrashed   Status = "crashed"
	StatusCancelled Status = "cancelled"
)

// Usage contains aggregated token usage and estimated cost for a job.
type Usage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	TotalTokens              int     `json:"total_tokens"`
	EstimatedCostUSD         float64 `json:"estimated_cost_usd"`
}

// EstimateCost returns an estimated USD cost based on the model name and token
// counts. Pricing tiers are matched by substring (haiku / opus / sonnet).
// Cache creation is billed at 1.25× the input rate; cache reads at 0.1×.
func EstimateCost(model string, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int) float64 {
	var inputPricePerM, outputPricePerM float64

	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "haiku"):
		inputPricePerM = 0.25
		outputPricePerM = 1.25
	case strings.Contains(m, "opus"):
		inputPricePerM = 15.00
		outputPricePerM = 75.00
	default: // sonnet and unrecognised models
		inputPricePerM = 3.00
		outputPricePerM = 15.00
	}

	cost := float64(inputTokens) * inputPricePerM / 1_000_000
	cost += float64(outputTokens) * outputPricePerM / 1_000_000
	cost += float64(cacheCreationTokens) * inputPricePerM * 1.25 / 1_000_000
	cost += float64(cacheReadTokens) * inputPricePerM * 0.1 / 1_000_000
	return cost
}

// CrashInfo contains details about a process crash
type CrashInfo struct {
	// Exit code (if available)
	ExitCode int `json:"exit_code,omitempty"`
	// Signal that killed the process (SIGSEGV, SIGKILL, etc.)
	Signal string `json:"signal,omitempty"`
	// Human-readable crash reason
	Reason string `json:"reason,omitempty"`
	// Partial output captured before crash
	PartialOutput string `json:"partial_output,omitempty"`
	// Whether the task appeared to complete before crashing
	TaskCompleted bool `json:"task_completed,omitempty"`
}

// Job represents an async execution job
type Job struct {
	ID          string     `json:"id"`
	Status      Status     `json:"status"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	SessionID   string     `json:"session_id,omitempty"`
	CrashInfo   *CrashInfo `json:"crash_info,omitempty"`
	Usage       *Usage     `json:"usage,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

// Manager manages async jobs
type Manager struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	stopChan   chan struct{}
	cleanupMu  sync.Mutex
}

// NewManager creates a new job manager
func NewManager() *Manager {
	return &Manager{
		jobs: make(map[string]*Job),
	}
}

// Create creates a new pending job
func (m *Manager) Create(id string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	job := &Job{
		ID:        id,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.jobs[id] = job
	return job
}

// Get retrieves a job by ID
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	// Return a copy to avoid race conditions
	jobCopy := *job
	return &jobCopy, true
}

// Update updates a job's status and output
func (m *Manager) Update(id string, status Status, output, errMsg, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return
	}

	job.Status = status
	job.Output = output
	job.Error = errMsg
	job.SessionID = sessionID
	job.UpdatedAt = time.Now()
}

// UpdateWithCrash updates a job with crash information
func (m *Manager) UpdateWithCrash(id string, crashInfo *CrashInfo, partialOutput, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return
	}

	job.Status = StatusCrashed
	job.CrashInfo = crashInfo
	job.Output = partialOutput
	job.SessionID = sessionID
	job.UpdatedAt = time.Now()
}

// UpdateUsage sets the token usage for a job.
func (m *Manager) UpdateUsage(id string, usage *Usage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return
	}
	job.Usage = usage
	job.UpdatedAt = time.Now()
}

// List returns all jobs
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		// Return copies to avoid race conditions
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs
}

// Delete removes a job
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.jobs[id]; !ok {
		return false
	}
	delete(m.jobs, id)
	return true
}

// Cancel cancels a pending or running job
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return false
	}

	// Only cancel pending or running jobs
	if job.Status != StatusPending && job.Status != StatusRunning {
		return false
	}

	now := time.Now()
	job.Status = StatusCancelled
	job.CancelledAt = &now
	job.UpdatedAt = now
	return true
}

// StartCleanup starts a background goroutine that removes completed/failed/cancelled jobs older than TTL
func (m *Manager) StartCleanup(ttl time.Duration, interval time.Duration) {
	m.cleanupMu.Lock()
	defer m.cleanupMu.Unlock()

	// If already running, don't start another
	if m.stopChan != nil {
		return
	}

	m.stopChan = make(chan struct{})
	go m.cleanupLoop(ttl, interval)
}

// StopCleanup stops the cleanup goroutine
func (m *Manager) StopCleanup() {
	m.cleanupMu.Lock()
	defer m.cleanupMu.Unlock()

	if m.stopChan != nil {
		close(m.stopChan)
		m.stopChan = nil
	}
}

// cleanupLoop runs the cleanup process at regular intervals
func (m *Manager) cleanupLoop(ttl time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup(ttl)
		case <-m.stopChan:
			return
		}
	}
}

// cleanup removes jobs older than TTL that are in terminal states
func (m *Manager) cleanup(ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, job := range m.jobs {
		// Only clean up terminal states (including crashed)
		if job.Status != StatusCompleted && job.Status != StatusFailed && job.Status != StatusCancelled && job.Status != StatusCrashed {
			continue
		}

		// Check if job is older than TTL
		if now.Sub(job.UpdatedAt) > ttl {
			delete(m.jobs, id)
		}
	}
}
