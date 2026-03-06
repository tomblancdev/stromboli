package job

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	require.NotNil(t, m)
	assert.Empty(t, m.List())
}

func TestManager_Create(t *testing.T) {
	m := NewManager()

	job := m.Create("job-123")

	require.NotNil(t, job)
	assert.Equal(t, "job-123", job.ID)
	assert.Equal(t, StatusPending, job.Status)
	assert.Empty(t, job.Output)
	assert.Empty(t, job.Error)
	assert.Empty(t, job.SessionID)
	assert.False(t, job.CreatedAt.IsZero())
	assert.False(t, job.UpdatedAt.IsZero())
}

func TestManager_Get(t *testing.T) {
	m := NewManager()

	t.Run("existing job", func(t *testing.T) {
		m.Create("job-123")

		job, ok := m.Get("job-123")

		require.True(t, ok)
		assert.Equal(t, "job-123", job.ID)
	})

	t.Run("non-existing job", func(t *testing.T) {
		job, ok := m.Get("non-existent")

		assert.False(t, ok)
		assert.Nil(t, job)
	})
}

func TestManager_Update(t *testing.T) {
	m := NewManager()
	m.Create("job-123")

	m.Update("job-123", StatusCompleted, "output text", "", "sess-456")

	job, ok := m.Get("job-123")
	require.True(t, ok)
	assert.Equal(t, StatusCompleted, job.Status)
	assert.Equal(t, "output text", job.Output)
	assert.Empty(t, job.Error)
	assert.Equal(t, "sess-456", job.SessionID)
}

func TestManager_Update_WithError(t *testing.T) {
	m := NewManager()
	m.Create("job-123")

	m.Update("job-123", StatusFailed, "", "something went wrong", "")

	job, ok := m.Get("job-123")
	require.True(t, ok)
	assert.Equal(t, StatusFailed, job.Status)
	assert.Empty(t, job.Output)
	assert.Equal(t, "something went wrong", job.Error)
}

func TestManager_Update_NonExistent(t *testing.T) {
	m := NewManager()

	// Should not panic
	m.Update("non-existent", StatusRunning, "", "", "")

	_, ok := m.Get("non-existent")
	assert.False(t, ok)
}

func TestManager_List(t *testing.T) {
	m := NewManager()

	t.Run("empty manager", func(t *testing.T) {
		jobs := m.List()
		assert.Empty(t, jobs)
	})

	t.Run("with jobs", func(t *testing.T) {
		m.Create("job-1")
		m.Create("job-2")
		m.Create("job-3")

		jobs := m.List()

		assert.Len(t, jobs, 3)

		ids := make(map[string]bool)
		for _, j := range jobs {
			ids[j.ID] = true
		}
		assert.True(t, ids["job-1"])
		assert.True(t, ids["job-2"])
		assert.True(t, ids["job-3"])
	})
}

func TestManager_Delete(t *testing.T) {
	m := NewManager()

	t.Run("existing job", func(t *testing.T) {
		m.Create("job-123")

		ok := m.Delete("job-123")

		assert.True(t, ok)
		_, exists := m.Get("job-123")
		assert.False(t, exists)
	})

	t.Run("non-existing job", func(t *testing.T) {
		ok := m.Delete("non-existent")
		assert.False(t, ok)
	})
}

func TestManager_Get_ReturnsCopy(t *testing.T) {
	m := NewManager()
	m.Create("job-123")

	// Get a copy
	job1, _ := m.Get("job-123")
	job1.Status = StatusCompleted

	// Original should be unchanged
	job2, _ := m.Get("job-123")
	assert.Equal(t, StatusPending, job2.Status)
}

func TestManager_List_ReturnsCopies(t *testing.T) {
	m := NewManager()
	m.Create("job-123")

	// Get list
	jobs := m.List()
	jobs[0].Status = StatusCompleted

	// Original should be unchanged
	job, _ := m.Get("job-123")
	assert.Equal(t, StatusPending, job.Status)
}

func TestManager_Cancel(t *testing.T) {
	m := NewManager()

	t.Run("cancel existing pending job", func(t *testing.T) {
		m.Create("job-123")

		ok := m.Cancel("job-123")

		require.True(t, ok)
		job, exists := m.Get("job-123")
		require.True(t, exists)
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotNil(t, job.CancelledAt)
		assert.False(t, job.CancelledAt.IsZero())
	})

	t.Run("cancel existing running job", func(t *testing.T) {
		m.Create("job-456")
		m.Update("job-456", StatusRunning, "", "", "")

		ok := m.Cancel("job-456")

		require.True(t, ok)
		job, exists := m.Get("job-456")
		require.True(t, exists)
		assert.Equal(t, StatusCancelled, job.Status)
		assert.NotNil(t, job.CancelledAt)
	})

	t.Run("cannot cancel completed job", func(t *testing.T) {
		m.Create("job-completed")
		m.Update("job-completed", StatusCompleted, "output", "", "sess-1")

		ok := m.Cancel("job-completed")

		assert.False(t, ok)
		job, _ := m.Get("job-completed")
		assert.Equal(t, StatusCompleted, job.Status)
		assert.Nil(t, job.CancelledAt)
	})

	t.Run("cannot cancel failed job", func(t *testing.T) {
		m.Create("job-failed")
		m.Update("job-failed", StatusFailed, "", "error", "")

		ok := m.Cancel("job-failed")

		assert.False(t, ok)
		job, _ := m.Get("job-failed")
		assert.Equal(t, StatusFailed, job.Status)
		assert.Nil(t, job.CancelledAt)
	})

	t.Run("cannot cancel already cancelled job", func(t *testing.T) {
		m.Create("job-cancelled")
		m.Cancel("job-cancelled")

		ok := m.Cancel("job-cancelled")

		assert.False(t, ok)
		job, _ := m.Get("job-cancelled")
		assert.Equal(t, StatusCancelled, job.Status)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		ok := m.Cancel("non-existent")
		assert.False(t, ok)
	})
}

func TestManager_StartStopCleanup(t *testing.T) {
	m := NewManager()

	t.Run("cleanup removes old completed jobs", func(t *testing.T) {
		// Create an old completed job
		m.Create("old-job")
		m.Update("old-job", StatusCompleted, "done", "", "")
		// Set UpdatedAt AFTER Update (which resets it)
		m.mu.Lock()
		m.jobs["old-job"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		// Create a recent job
		m.Create("recent-job")
		m.Update("recent-job", StatusCompleted, "done", "", "")

		// Start cleanup with 1 hour TTL, 10ms interval
		m.StartCleanup(1*time.Hour, 10*time.Millisecond)

		// Wait for cleanup to run
		time.Sleep(50 * time.Millisecond)

		// Stop cleanup
		m.StopCleanup()

		// Old job should be deleted
		_, exists := m.Get("old-job")
		assert.False(t, exists)

		// Recent job should still exist
		_, exists = m.Get("recent-job")
		assert.True(t, exists)
	})

	t.Run("cleanup removes old failed jobs", func(t *testing.T) {
		m := NewManager()
		m.Create("old-failed")
		m.Update("old-failed", StatusFailed, "", "error", "")
		m.mu.Lock()
		m.jobs["old-failed"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("old-failed")
		assert.False(t, exists)
	})

	t.Run("cleanup removes old cancelled jobs", func(t *testing.T) {
		m := NewManager()
		m.Create("old-cancelled")
		m.Cancel("old-cancelled")
		m.mu.Lock()
		m.jobs["old-cancelled"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("old-cancelled")
		assert.False(t, exists)
	})

	t.Run("cleanup does not remove pending jobs", func(t *testing.T) {
		m := NewManager()
		m.Create("old-pending")
		m.mu.Lock()
		m.jobs["old-pending"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("old-pending")
		assert.True(t, exists)
	})

	t.Run("cleanup does not remove running jobs", func(t *testing.T) {
		m := NewManager()
		m.Create("old-running")
		m.Update("old-running", StatusRunning, "", "", "")
		m.mu.Lock()
		m.jobs["old-running"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("old-running")
		assert.True(t, exists)
	})

	t.Run("stop cleanup when not started", func(t *testing.T) {
		m := NewManager()
		// Should not panic
		m.StopCleanup()
	})

	t.Run("multiple start calls create only one cleanup goroutine", func(t *testing.T) {
		m := NewManager()
		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		m.StartCleanup(1*time.Hour, 10*time.Millisecond)

		time.Sleep(20 * time.Millisecond)
		m.StopCleanup()

		// No assertion needed, just ensure no panic or goroutine leak
	})
}

func TestManager_UpdateWithCrash(t *testing.T) {
	m := NewManager()

	t.Run("update with crash info", func(t *testing.T) {
		m.Create("job-crash")

		crashInfo := &CrashInfo{
			ExitCode:      139,
			Signal:        "SIGSEGV",
			Reason:        "Process killed by SIGSEGV",
			PartialOutput: "some output before crash",
			TaskCompleted: true,
		}

		m.UpdateWithCrash("job-crash", crashInfo, "partial output", "sess-123")

		job, ok := m.Get("job-crash")
		require.True(t, ok)
		assert.Equal(t, StatusCrashed, job.Status)
		assert.Equal(t, "partial output", job.Output)
		assert.Equal(t, "sess-123", job.SessionID)
		assert.NotNil(t, job.CrashInfo)
		assert.Equal(t, 139, job.CrashInfo.ExitCode)
		assert.Equal(t, "SIGSEGV", job.CrashInfo.Signal)
		assert.True(t, job.CrashInfo.TaskCompleted)
	})

	t.Run("update with crash non-existent job", func(t *testing.T) {
		crashInfo := &CrashInfo{
			ExitCode: 139,
		}

		// Should not panic
		m.UpdateWithCrash("non-existent", crashInfo, "output", "")

		_, ok := m.Get("non-existent")
		assert.False(t, ok)
	})
}

func TestManager_Cleanup_CrashedJobs(t *testing.T) {
	m := NewManager()

	t.Run("cleanup removes old crashed jobs", func(t *testing.T) {
		m.Create("old-crashed")
		m.UpdateWithCrash("old-crashed", &CrashInfo{ExitCode: 139}, "output", "sess-1")
		m.mu.Lock()
		m.jobs["old-crashed"].UpdatedAt = time.Now().Add(-2 * time.Hour)
		m.mu.Unlock()

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("old-crashed")
		assert.False(t, exists)
	})

	t.Run("recent crashed jobs are not cleaned up", func(t *testing.T) {
		m := NewManager()
		m.Create("recent-crashed")
		m.UpdateWithCrash("recent-crashed", &CrashInfo{ExitCode: 139}, "output", "sess-1")

		m.StartCleanup(1*time.Hour, 10*time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		m.StopCleanup()

		_, exists := m.Get("recent-crashed")
		assert.True(t, exists)
	})
}

func TestManager_UpdateUsage(t *testing.T) {
	m := NewManager()

	t.Run("update usage for existing job", func(t *testing.T) {
		m.Create("job-usage")
		m.Update("job-usage", StatusCompleted, "output", "", "sess-1")

		usage := &Usage{
			InputTokens:              100,
			OutputTokens:             50,
			CacheCreationInputTokens: 200,
			CacheReadInputTokens:     1000,
			TotalTokens:              1350,
			EstimatedCostUSD:         0.00123,
		}
		m.UpdateUsage("job-usage", usage)

		job, ok := m.Get("job-usage")
		require.True(t, ok)
		require.NotNil(t, job.Usage)
		assert.Equal(t, 100, job.Usage.InputTokens)
		assert.Equal(t, 50, job.Usage.OutputTokens)
		assert.Equal(t, 200, job.Usage.CacheCreationInputTokens)
		assert.Equal(t, 1000, job.Usage.CacheReadInputTokens)
		assert.Equal(t, 1350, job.Usage.TotalTokens)
		assert.InDelta(t, 0.00123, job.Usage.EstimatedCostUSD, 1e-9)
	})

	t.Run("update usage for non-existent job does not panic", func(t *testing.T) {
		usage := &Usage{InputTokens: 10}
		m.UpdateUsage("non-existent", usage)
	})

	t.Run("job usage is nil by default", func(t *testing.T) {
		m.Create("job-nousage")
		job, ok := m.Get("job-nousage")
		require.True(t, ok)
		assert.Nil(t, job.Usage)
	})
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		input        int
		output       int
		cacheCreate  int
		cacheRead    int
		wantApproxUSD float64
	}{
		{
			name:          "haiku model",
			model:         "claude-haiku-4-5-20251001",
			input:         1_000_000,
			output:        1_000_000,
			cacheCreate:   0,
			cacheRead:     0,
			wantApproxUSD: 1.50, // 0.25 + 1.25
		},
		{
			name:          "sonnet model",
			model:         "claude-sonnet-4-5-20251101",
			input:         1_000_000,
			output:        1_000_000,
			cacheCreate:   0,
			cacheRead:     0,
			wantApproxUSD: 18.00, // 3.00 + 15.00
		},
		{
			name:          "opus model",
			model:         "claude-opus-4-5-20251101",
			input:         1_000_000,
			output:        1_000_000,
			cacheCreate:   0,
			cacheRead:     0,
			wantApproxUSD: 90.00, // 15.00 + 75.00
		},
		{
			name:          "unknown model defaults to sonnet pricing",
			model:         "unknown-model",
			input:         1_000_000,
			output:        0,
			cacheCreate:   0,
			cacheRead:     0,
			wantApproxUSD: 3.00,
		},
		{
			name:          "zero tokens",
			model:         "claude-sonnet-4-5-20251101",
			input:         0,
			output:        0,
			cacheCreate:   0,
			cacheRead:     0,
			wantApproxUSD: 0.0,
		},
		{
			name:          "cache tokens included in cost",
			model:         "claude-sonnet-4-5-20251101",
			input:         0,
			output:        0,
			cacheCreate:   1_000_000, // 1.25x input = 3.75
			cacheRead:     1_000_000, // 0.1x input  = 0.30
			wantApproxUSD: 4.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateCost(tt.model, tt.input, tt.output, tt.cacheCreate, tt.cacheRead)
			assert.InDelta(t, tt.wantApproxUSD, got, 0.001)
		})
	}
}

func TestCrashInfo_Struct(t *testing.T) {
	info := CrashInfo{
		ExitCode:      139,
		Signal:        "SIGSEGV",
		Reason:        "Process killed by SIGSEGV",
		PartialOutput: "some output",
		TaskCompleted: true,
	}

	assert.Equal(t, 139, info.ExitCode)
	assert.Equal(t, "SIGSEGV", info.Signal)
	assert.Equal(t, "Process killed by SIGSEGV", info.Reason)
	assert.Equal(t, "some output", info.PartialOutput)
	assert.True(t, info.TaskCompleted)
}
