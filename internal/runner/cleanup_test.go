package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsOlderThan(t *testing.T) {
	tests := []struct {
		name       string
		runningFor string
		maxAge     time.Duration
		expected   bool
	}{
		{
			name:       "3 days older than 1 hour",
			runningFor: "3 days",
			maxAge:     1 * time.Hour,
			expected:   true,
		},
		{
			name:       "30 minutes younger than 1 hour",
			runningFor: "30 minutes",
			maxAge:     1 * time.Hour,
			expected:   false,
		},
		{
			name:       "2 hours older than 1 hour",
			runningFor: "2 hours",
			maxAge:     1 * time.Hour,
			expected:   true,
		},
		{
			name:       "45 seconds younger than 1 minute",
			runningFor: "45 seconds",
			maxAge:     1 * time.Minute,
			expected:   false,
		},
		{
			name:       "1 week older than 1 day",
			runningFor: "1 week",
			maxAge:     24 * time.Hour,
			expected:   true,
		},
		{
			name:       "About a minute (edge case)",
			runningFor: "About a minute",
			maxAge:     1 * time.Hour,
			expected:   false, // Can't parse "About"
		},
		{
			name:       "empty string",
			runningFor: "",
			maxAge:     1 * time.Hour,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOlderThan(tt.runningFor, tt.maxAge)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainerNamePrefix(t *testing.T) {
	assert.Equal(t, "stromboli-agent-", ContainerNamePrefix)
}
