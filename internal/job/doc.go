// Package job provides async job management for long-running Claude execution tasks.
//
// # Features
//
//   - Create and track async jobs
//   - Query job status (pending, running, completed, error)
//   - Automatic cleanup of expired jobs
//   - Thread-safe job storage
//
// # Job Lifecycle
//
//	pending -> running -> completed
//	                   -> error
//
// # Usage
//
// Create a job manager:
//
//	mgr := job.NewManager()
//	mgr.StartCleanup(time.Hour, 5*time.Minute)
//	defer mgr.StopCleanup()
//
// Create and manage jobs:
//
//	jobID := mgr.Create()
//	mgr.SetRunning(jobID)
//	mgr.Complete(jobID, "output", "session-id")
//
// Query jobs:
//
//	j, exists := mgr.Get(jobID)
//	all := mgr.List()
//
// # Cleanup
//
// Jobs are automatically cleaned up based on:
//   - Retention time: How long completed jobs are kept
//   - Cleanup interval: How often cleanup runs
package job
