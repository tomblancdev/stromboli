package job

import (
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

	stop := make(chan struct{})
	m.stopChan = stop
	// Pass stop explicitly so the loop never touches m.stopChan under no lock.
	go m.cleanupLoop(ttl, interval, stop)
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
func (m *Manager) cleanupLoop(ttl time.Duration, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup(ttl)
		case <-stop:
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
