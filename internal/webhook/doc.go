// Package webhook provides HTTP webhook notifications for async job completion.
//
// # Features
//
//   - POST notifications with JSON payload
//   - Automatic retry on failure (1 retry after 100ms)
//   - 5 second timeout per request
//   - Standard job result payload format
//
// # Usage
//
//	notifier := webhook.NewNotifier()
//	err := notifier.Notify("https://example.com/webhook", webhook.JobResult{
//	    JobID:     "job-123",
//	    Status:    "completed",
//	    Output:    "Task completed successfully",
//	    SessionID: "sess-456",
//	})
//
// # Payload Format
//
// The webhook sends a POST request with JSON body:
//
//	{
//	    "job_id": "job-123",
//	    "status": "completed|error",
//	    "output": "...",
//	    "error": "...",
//	    "session_id": "sess-456"
//	}
package webhook
