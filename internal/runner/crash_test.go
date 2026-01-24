package runner

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCrash_NilError(t *testing.T) {
	assert.False(t, IsCrash(nil))
}

func TestIsCrash_NonExitError(t *testing.T) {
	err := exec.ErrNotFound
	assert.False(t, IsCrash(err))
}

func TestExtractCrashInfo_NilError(t *testing.T) {
	info := ExtractCrashInfo(nil, "")
	assert.Nil(t, info)
}

func TestExtractCrashInfo_PartialOutput(t *testing.T) {
	// Create a simple exit error to test output handling
	err := exec.ErrNotFound
	output := "some partial output before crash"

	info := ExtractCrashInfo(err, output)
	assert.NotNil(t, info)
	assert.Equal(t, output, info.PartialOutput)
}

func TestExtractCrashInfo_LongOutput(t *testing.T) {
	// Create a very long output
	longOutput := make([]byte, 3000)
	for i := range longOutput {
		longOutput[i] = 'x'
	}

	err := exec.ErrNotFound
	info := ExtractCrashInfo(err, string(longOutput))
	assert.NotNil(t, info)
	// Should be truncated to last 2000 chars with "..." prefix
	assert.True(t, len(info.PartialOutput) <= 2003)
	assert.True(t, len(info.PartialOutput) > 0)
}

func TestCheckTaskCompleted(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		errMsg   string
		expected bool
	}{
		{
			name:     "No messages returned pattern",
			output:   "Task output\nNo messages returned\n",
			errMsg:   "",
			expected: true,
		},
		{
			name:     "Task completed pattern",
			output:   "Task completed successfully",
			errMsg:   "",
			expected: true,
		},
		{
			name:     "Finished successfully pattern",
			output:   "Work finished successfully",
			errMsg:   "",
			expected: true,
		},
		{
			name:     "Pattern in error message",
			output:   "",
			errMsg:   "Error: No messages returned",
			expected: true,
		},
		{
			name:     "No pattern",
			output:   "Something went wrong",
			errMsg:   "Unexpected error",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkTaskCompleted(tt.output, tt.errMsg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCrashSignals(t *testing.T) {
	// Test that common crash signals are mapped
	assert.Equal(t, "SIGSEGV", crashSignals[11])
	assert.Equal(t, "SIGKILL", crashSignals[9])
	assert.Equal(t, "SIGTERM", crashSignals[15])
	assert.Equal(t, "SIGABRT", crashSignals[6])
}
