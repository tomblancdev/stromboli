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
)

// Job represents an async execution job
type Job struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager manages async jobs
type Manager struct {
	jobs map[string]*Job
	mu   sync.RWMutex
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
