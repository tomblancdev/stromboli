package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tomblanc/stromboli/internal/session"
)

func TestGenerateSessionID_IsUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := session.GenerateID()
		assert.NotEmpty(t, id)
		// UUID format: 8-4-4-4-12 hex digits
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id)
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}

func TestGenerateRunID_IsUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRunID()
		assert.NotEmpty(t, id)
		assert.Contains(t, id, "run-")
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}
}
