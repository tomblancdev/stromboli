package job

import (
	"testing"

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
