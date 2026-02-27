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
	// Note: "claude" is NOT included - container ENTRYPOINT provides it
	assert.Equal(t, []string{"-p", "explain this code"}, cmd)
}

func TestWithModel(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithModel("opus").
		Build()
	assert.Contains(t, cmd, "--model")
	assert.Contains(t, cmd, "opus")
}

func TestWithSessionID_NewSession(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("start new session").
		WithSessionID("abc-123-def").
		Build()
	assert.Contains(t, cmd, "--session-id")
	assert.Contains(t, cmd, "abc-123-def")
	assert.NotContains(t, cmd, "--resume")
}

func TestWithSessionID_Resume(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("continue working").
		WithSessionID("abc-123-def").
		WithResume().
		Build()
	// Resume uses only --resume, not both flags
	assert.Contains(t, cmd, "--resume")
	assert.Contains(t, cmd, "abc-123-def")
	assert.NotContains(t, cmd, "--session-id")
}

func TestWithContinue(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("keep going").
		WithContinue().
		Build()
	assert.Contains(t, cmd, "--continue")
}

func TestWithForkSession(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithForkSession().
		Build()
	assert.Contains(t, cmd, "--fork-session")
}

func TestWithNoSessionPersistence(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithNoSessionPersistence().
		Build()
	assert.Contains(t, cmd, "--no-session-persistence")
}

func TestWithFallbackModel(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithFallbackModel("sonnet").
		Build()
	assert.Contains(t, cmd, "--fallback-model")
	assert.Contains(t, cmd, "sonnet")
}

func TestWithSystemPrompt(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithSystemPrompt("You are a helpful assistant").
		Build()
	assert.Contains(t, cmd, "--system-prompt")
	assert.Contains(t, cmd, "You are a helpful assistant")
}

func TestWithAppendSystemPrompt(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithAppendSystemPrompt("Additional instructions").
		Build()
	assert.Contains(t, cmd, "--append-system-prompt")
	assert.Contains(t, cmd, "Additional instructions")
}

func TestWithTools(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithTools("Bash", "Read", "Edit").
		Build()
	assert.Contains(t, cmd, "--tools")
	assert.Contains(t, cmd, "Bash")
	assert.Contains(t, cmd, "Read")
	assert.Contains(t, cmd, "Edit")
}

func TestWithAllowedTools(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithAllowedTools("Bash", "Read").
		Build()
	assert.Contains(t, cmd, "--allowedTools")
	assert.Contains(t, cmd, "Bash,Read")
}

func TestWithDisallowedTools(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithDisallowedTools("Edit", "Write").
		Build()
	assert.Contains(t, cmd, "--disallowedTools")
	assert.Contains(t, cmd, "Edit,Write")
}

func TestWithPermissionMode(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithPermissionMode("bypassPermissions").
		Build()
	assert.Contains(t, cmd, "--permission-mode")
	assert.Contains(t, cmd, "bypassPermissions")
}

func TestWithDangerouslySkipPermissions(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithDangerouslySkipPermissions().
		Build()
	assert.Contains(t, cmd, "--dangerously-skip-permissions")
}

func TestWithAllowDangerouslySkipPermissions(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithAllowDangerouslySkipPermissions().
		Build()
	assert.Contains(t, cmd, "--allow-dangerously-skip-permissions")
}

func TestWithOutputFormat(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithOutputFormat("json").
		Build()
	assert.Contains(t, cmd, "--output-format")
	assert.Contains(t, cmd, "json")
}

func TestWithInputFormat(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithInputFormat("stream-json").
		Build()
	assert.Contains(t, cmd, "--input-format")
	assert.Contains(t, cmd, "stream-json")
}

func TestWithIncludePartialMessages(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithIncludePartialMessages().
		Build()
	assert.Contains(t, cmd, "--include-partial-messages")
}

func TestWithReplayUserMessages(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithReplayUserMessages().
		Build()
	assert.Contains(t, cmd, "--replay-user-messages")
}

func TestWithJSONSchema(t *testing.T) {
	schema := `{"type":"object"}`
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithJSONSchema(schema).
		Build()
	assert.Contains(t, cmd, "--json-schema")
	assert.Contains(t, cmd, schema)
}

func TestWithMaxBudgetUSD(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithMaxBudgetUSD(5.0).
		Build()
	assert.Contains(t, cmd, "--max-budget-usd")
	assert.Contains(t, cmd, "5.00")
}

func TestWithMaxTurns(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithMaxTurns(30).
		Build()
	assert.Contains(t, cmd, "--max-turns")
	assert.Contains(t, cmd, "30")
}

func TestWithMaxTurns_Zero(t *testing.T) {
	// Zero means unlimited — must still be passed to CLI
	cmd := NewCommandBuilder().
		WithPrompt("hello").
		WithMaxTurns(0).
		Build()
	assert.Contains(t, cmd, "--max-turns")
	assert.Contains(t, cmd, "0")
}

func TestWithMCPConfig(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithMCPConfig("/path/to/mcp.json").
		Build()
	assert.Contains(t, cmd, "--mcp-config")
	assert.Contains(t, cmd, "/path/to/mcp.json")
}

func TestWithStrictMCPConfig(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithStrictMCPConfig().
		Build()
	assert.Contains(t, cmd, "--strict-mcp-config")
}

func TestWithAgent(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithAgent("reviewer").
		Build()
	assert.Contains(t, cmd, "--agent")
	assert.Contains(t, cmd, "reviewer")
}

func TestWithAgents(t *testing.T) {
	agents := map[string]any{
		"reviewer": map[string]any{
			"description": "Reviews code",
		},
	}
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithAgents(agents).
		Build()
	assert.Contains(t, cmd, "--agents")
}

func TestWithAddDir(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithAddDir("/extra/dir1", "/extra/dir2").
		Build()
	assert.Contains(t, cmd, "--add-dir")
	assert.Contains(t, cmd, "/extra/dir1")
	assert.Contains(t, cmd, "/extra/dir2")
}

func TestWithPluginDir(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithPluginDir("/plugins").
		Build()
	assert.Contains(t, cmd, "--plugin-dir")
	assert.Contains(t, cmd, "/plugins")
}

func TestWithFiles(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithFiles("file_abc:doc.txt").
		Build()
	assert.Contains(t, cmd, "--file")
	assert.Contains(t, cmd, "file_abc:doc.txt")
}

func TestWithSettings(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithSettings("/path/to/settings.json").
		Build()
	assert.Contains(t, cmd, "--settings")
	assert.Contains(t, cmd, "/path/to/settings.json")
}

func TestWithSettingSources(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithSettingSources("user", "project").
		Build()
	assert.Contains(t, cmd, "--setting-sources")
	assert.Contains(t, cmd, "user,project")
}

func TestWithBetas(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithBetas("beta1", "beta2").
		Build()
	assert.Contains(t, cmd, "--betas")
	assert.Contains(t, cmd, "beta1")
	assert.Contains(t, cmd, "beta2")
}

func TestWithVerbose(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithVerbose().
		Build()
	assert.Contains(t, cmd, "--verbose")
}

func TestWithDebug(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithDebug("api,hooks").
		Build()
	assert.Contains(t, cmd, "--debug")
	assert.Contains(t, cmd, "api,hooks")
}

func TestWithDebugNoFilter(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithDebug("").
		Build()
	assert.Contains(t, cmd, "--debug")
	// Should not have a filter argument after --debug
}

func TestWithDisableSlashCommands(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("test").
		WithDisableSlashCommands().
		Build()
	assert.Contains(t, cmd, "--disable-slash-commands")
}

func TestFullBuilder(t *testing.T) {
	cmd := NewCommandBuilder().
		WithPrompt("refactor this function").
		WithModel("sonnet").
		WithSessionID("session-456").
		WithSystemPrompt("Be concise").
		WithOutputFormat("json").
		WithDangerouslySkipPermissions().
		WithMaxBudgetUSD(10.00).
		WithVerbose().
		Build()

	// Note: "claude" is NOT included - container ENTRYPOINT provides it
	assert.Equal(t, "-p", cmd[0])
	assert.Contains(t, cmd, "-p")
	assert.Contains(t, cmd, "refactor this function")
	assert.Contains(t, cmd, "--model")
	assert.Contains(t, cmd, "sonnet")
	assert.Contains(t, cmd, "--session-id") // New session, not resume
	assert.Contains(t, cmd, "session-456")
	assert.Contains(t, cmd, "--system-prompt")
	assert.Contains(t, cmd, "Be concise")
	assert.Contains(t, cmd, "--output-format")
	assert.Contains(t, cmd, "json")
	assert.Contains(t, cmd, "--dangerously-skip-permissions")
	assert.Contains(t, cmd, "--max-budget-usd")
	assert.Contains(t, cmd, "10.00")
	assert.Contains(t, cmd, "--verbose")
}
