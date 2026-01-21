package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCommandBuilder_DefaultsPrintMode(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		Build()
	assert.Contains(t, cmd, "-p")
}

func TestWithPrompt(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("explain this code").
		Build()
	assert.Equal(t, []string{"claude", "-p", "explain this code"}, cmd)
}

func TestWithModel(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithModel("opus").
		Build()
	assert.Contains(t, cmd, "--model")
	assert.Contains(t, cmd, "opus")
}

func TestWithSessionID_Resume(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("continue working").
		WithSessionID("abc-123-def").
		Build()
	assert.Contains(t, cmd, "--resume")
	assert.Contains(t, cmd, "abc-123-def")
}

func TestWithContinue(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("keep going").
		WithContinue().
		Build()
	assert.Contains(t, cmd, "--continue")
}

func TestWithSystemPrompt(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithSystemPrompt("You are a helpful assistant").
		Build()
	assert.Contains(t, cmd, "--system-prompt")
	assert.Contains(t, cmd, "You are a helpful assistant")
}

func TestWithOutputFormat(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithOutputFormat("json").
		Build()
	assert.Contains(t, cmd, "--output-format")
	assert.Contains(t, cmd, "json")
}

func TestWithAllowedTools(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithAllowedTools("Bash", "Read").
		Build()
	assert.Contains(t, cmd, "--allowedTools")
	assert.Contains(t, cmd, "Bash")
	assert.Contains(t, cmd, "Read")
}

func TestWithDisallowedTools(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithDisallowedTools("Edit", "Write").
		Build()
	assert.Contains(t, cmd, "--disallowedTools")
}

func TestWithDangerouslySkipPermissions(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithDangerouslySkipPermissions().
		Build()
	assert.Contains(t, cmd, "--dangerously-skip-permissions")
}

func TestWithMaxBudget(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithMaxBudget(5.0).
		Build()
	assert.Contains(t, cmd, "--max-budget-usd")
	assert.Contains(t, cmd, "5.00")
}

func TestFullBuilder(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("refactor this function").
		WithModel("sonnet").
		WithSessionID("session-456").
		WithSystemPrompt("Be concise").
		WithOutputFormat("json").
		WithDangerouslySkipPermissions().
		Build()

	assert.Equal(t, "claude", cmd[0])
	assert.Contains(t, cmd, "-p")
	assert.Contains(t, cmd, "refactor this function")
	assert.Contains(t, cmd, "--model")
	assert.Contains(t, cmd, "sonnet")
	assert.Contains(t, cmd, "--resume")
	assert.Contains(t, cmd, "session-456")
	assert.Contains(t, cmd, "--system-prompt")
	assert.Contains(t, cmd, "Be concise")
	assert.Contains(t, cmd, "--output-format")
	assert.Contains(t, cmd, "json")
	assert.Contains(t, cmd, "--dangerously-skip-permissions")
}
