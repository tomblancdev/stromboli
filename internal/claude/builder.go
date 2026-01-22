package claude

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CommandBuilder builds claude CLI commands with a fluent API
// Supports ALL Claude CLI options for headless mode
type CommandBuilder struct {
	// Core
	prompt string

	// Session management
	sessionID       string // --session-id (sets the session UUID)
	resume          bool   // --resume (continue existing conversation)
	continueSession bool   // --continue
	forkSession     bool   // --fork-session
	noPersistence   bool   // --no-session-persistence

	// Model configuration
	model         string // --model
	fallbackModel string // --fallback-model

	// System prompt
	systemPrompt       string // --system-prompt
	appendSystemPrompt string // --append-system-prompt

	// Tools configuration
	tools           []string // --tools (built-in set)
	allowedTools    []string // --allowedTools
	disallowedTools []string // --disallowedTools

	// Permissions
	permissionMode                   string // --permission-mode
	dangerouslySkipPermissions       bool   // --dangerously-skip-permissions
	allowDangerouslySkipPermissions  bool   // --allow-dangerously-skip-permissions

	// Input/Output format
	inputFormat            string // --input-format
	outputFormat           string // --output-format
	includePartialMessages bool   // --include-partial-messages
	replayUserMessages     bool   // --replay-user-messages

	// Structured output
	jsonSchema string // --json-schema

	// Budget control
	maxBudgetUSD float64 // --max-budget-usd

	// MCP configuration
	mcpConfigs      []string // --mcp-config
	strictMcpConfig bool     // --strict-mcp-config

	// Agents
	agent       string            // --agent
	agentsJSON  map[string]any    // --agents

	// Additional directories
	addDirs []string // --add-dir

	// Plugins
	pluginDirs []string // --plugin-dir

	// Settings
	settings       string   // --settings (file path or JSON)
	settingSources []string // --setting-sources

	// Files
	files []string // --file (file_id:path format)

	// Beta features
	betas []string // --betas

	// Misc
	verbose              bool // --verbose
	debug                string // --debug (optional filter)
	disableSlashCommands bool // --disable-slash-commands
}

// NewCommandBuilder creates a new claude command builder
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// --- Core ---

// WithPrompt sets the prompt to send to Claude
func (b *CommandBuilder) WithPrompt(prompt string) *CommandBuilder {
	b.prompt = prompt
	return b
}

// --- Session Management ---

// WithSessionID sets a specific session ID (must be valid UUID)
func (b *CommandBuilder) WithSessionID(id string) *CommandBuilder {
	b.sessionID = id
	return b
}

// WithResume enables resuming an existing conversation (requires session ID to be set)
func (b *CommandBuilder) WithResume() *CommandBuilder {
	b.resume = true
	return b
}

// WithContinue continues the most recent conversation in current directory
func (b *CommandBuilder) WithContinue() *CommandBuilder {
	b.continueSession = true
	return b
}

// WithForkSession creates a new session ID when resuming
func (b *CommandBuilder) WithForkSession() *CommandBuilder {
	b.forkSession = true
	return b
}

// WithNoSessionPersistence disables session saving to disk
func (b *CommandBuilder) WithNoSessionPersistence() *CommandBuilder {
	b.noPersistence = true
	return b
}

// --- Model Configuration ---

// WithModel sets the model (e.g., "opus", "sonnet", or full name)
func (b *CommandBuilder) WithModel(model string) *CommandBuilder {
	b.model = model
	return b
}

// WithFallbackModel sets fallback model when default is overloaded
func (b *CommandBuilder) WithFallbackModel(model string) *CommandBuilder {
	b.fallbackModel = model
	return b
}

// --- System Prompt ---

// WithSystemPrompt sets a custom system prompt (replaces default)
func (b *CommandBuilder) WithSystemPrompt(prompt string) *CommandBuilder {
	b.systemPrompt = prompt
	return b
}

// WithAppendSystemPrompt appends to the default system prompt
func (b *CommandBuilder) WithAppendSystemPrompt(prompt string) *CommandBuilder {
	b.appendSystemPrompt = prompt
	return b
}

// --- Tools Configuration ---

// WithTools sets available tools from built-in set
// Use "" to disable all, "default" for all, or specific names
func (b *CommandBuilder) WithTools(tools ...string) *CommandBuilder {
	b.tools = tools
	return b
}

// WithAllowedTools sets allowed tools (e.g., "Bash(git:*) Edit")
func (b *CommandBuilder) WithAllowedTools(tools ...string) *CommandBuilder {
	b.allowedTools = tools
	return b
}

// WithDisallowedTools sets denied tools
func (b *CommandBuilder) WithDisallowedTools(tools ...string) *CommandBuilder {
	b.disallowedTools = tools
	return b
}

// --- Permissions ---

// WithPermissionMode sets permission mode
// Options: "acceptEdits", "bypassPermissions", "default", "delegate", "dontAsk", "plan"
func (b *CommandBuilder) WithPermissionMode(mode string) *CommandBuilder {
	b.permissionMode = mode
	return b
}

// WithDangerouslySkipPermissions bypasses all permission checks
func (b *CommandBuilder) WithDangerouslySkipPermissions() *CommandBuilder {
	b.dangerouslySkipPermissions = true
	return b
}

// WithAllowDangerouslySkipPermissions enables bypass as an option
func (b *CommandBuilder) WithAllowDangerouslySkipPermissions() *CommandBuilder {
	b.allowDangerouslySkipPermissions = true
	return b
}

// --- Input/Output Format ---

// WithInputFormat sets input format ("text" or "stream-json")
func (b *CommandBuilder) WithInputFormat(format string) *CommandBuilder {
	b.inputFormat = format
	return b
}

// WithOutputFormat sets output format ("text", "json", "stream-json")
func (b *CommandBuilder) WithOutputFormat(format string) *CommandBuilder {
	b.outputFormat = format
	return b
}

// WithIncludePartialMessages includes partial message chunks
func (b *CommandBuilder) WithIncludePartialMessages() *CommandBuilder {
	b.includePartialMessages = true
	return b
}

// WithReplayUserMessages re-emits user messages on stdout
func (b *CommandBuilder) WithReplayUserMessages() *CommandBuilder {
	b.replayUserMessages = true
	return b
}

// --- Structured Output ---

// WithJSONSchema sets JSON Schema for structured output validation
func (b *CommandBuilder) WithJSONSchema(schema string) *CommandBuilder {
	b.jsonSchema = schema
	return b
}

// --- Budget Control ---

// WithMaxBudgetUSD sets maximum dollar amount for API calls
func (b *CommandBuilder) WithMaxBudgetUSD(amount float64) *CommandBuilder {
	b.maxBudgetUSD = amount
	return b
}

// --- MCP Configuration ---

// WithMCPConfig adds MCP server configs (file paths or JSON strings)
func (b *CommandBuilder) WithMCPConfig(configs ...string) *CommandBuilder {
	b.mcpConfigs = append(b.mcpConfigs, configs...)
	return b
}

// WithStrictMCPConfig only uses MCP servers from --mcp-config
func (b *CommandBuilder) WithStrictMCPConfig() *CommandBuilder {
	b.strictMcpConfig = true
	return b
}

// --- Agents ---

// WithAgent sets the agent for current session
func (b *CommandBuilder) WithAgent(agent string) *CommandBuilder {
	b.agent = agent
	return b
}

// WithAgents sets custom agents as JSON object
func (b *CommandBuilder) WithAgents(agents map[string]any) *CommandBuilder {
	b.agentsJSON = agents
	return b
}

// --- Additional Resources ---

// WithAddDir adds directories for tool access
func (b *CommandBuilder) WithAddDir(dirs ...string) *CommandBuilder {
	b.addDirs = append(b.addDirs, dirs...)
	return b
}

// WithPluginDir loads plugins from directories
func (b *CommandBuilder) WithPluginDir(dirs ...string) *CommandBuilder {
	b.pluginDirs = append(b.pluginDirs, dirs...)
	return b
}

// WithFiles adds file resources (format: file_id:path)
func (b *CommandBuilder) WithFiles(files ...string) *CommandBuilder {
	b.files = append(b.files, files...)
	return b
}

// --- Settings ---

// WithSettings sets path to settings JSON file or JSON string
func (b *CommandBuilder) WithSettings(settings string) *CommandBuilder {
	b.settings = settings
	return b
}

// WithSettingSources sets which setting sources to load
func (b *CommandBuilder) WithSettingSources(sources ...string) *CommandBuilder {
	b.settingSources = sources
	return b
}

// --- Beta Features ---

// WithBetas adds beta headers for API requests
func (b *CommandBuilder) WithBetas(betas ...string) *CommandBuilder {
	b.betas = append(b.betas, betas...)
	return b
}

// --- Misc ---

// WithVerbose enables verbose mode
func (b *CommandBuilder) WithVerbose() *CommandBuilder {
	b.verbose = true
	return b
}

// WithDebug enables debug mode with optional category filter
func (b *CommandBuilder) WithDebug(filter string) *CommandBuilder {
	b.debug = filter
	if filter == "" {
		b.debug = "true" // marker for enabled without filter
	}
	return b
}

// WithDisableSlashCommands disables all skills
func (b *CommandBuilder) WithDisableSlashCommands() *CommandBuilder {
	b.disableSlashCommands = true
	return b
}

// Build generates the final claude command arguments
// Note: Does NOT include "claude" itself - the container ENTRYPOINT provides that
func (b *CommandBuilder) Build() []string {
	args := []string{}

	// Print mode for non-interactive use (required for headless)
	args = append(args, "-p", b.prompt)

	// Session management
	// --session-id sets the UUID for a NEW conversation
	// --resume continues an EXISTING conversation by ID
	// Cannot combine both without --fork-session
	if b.sessionID != "" {
		if b.resume {
			args = append(args, "--resume", b.sessionID)
		} else {
			args = append(args, "--session-id", b.sessionID)
		}
	}
	if b.continueSession {
		args = append(args, "--continue")
	}
	if b.forkSession {
		args = append(args, "--fork-session")
	}
	if b.noPersistence {
		args = append(args, "--no-session-persistence")
	}

	// Model configuration
	if b.model != "" {
		args = append(args, "--model", b.model)
	}
	if b.fallbackModel != "" {
		args = append(args, "--fallback-model", b.fallbackModel)
	}

	// System prompt
	if b.systemPrompt != "" {
		args = append(args, "--system-prompt", b.systemPrompt)
	}
	if b.appendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", b.appendSystemPrompt)
	}

	// Tools configuration
	if len(b.tools) > 0 {
		args = append(args, "--tools")
		args = append(args, b.tools...)
	}
	if len(b.allowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, strings.Join(b.allowedTools, ","))
	}
	if len(b.disallowedTools) > 0 {
		args = append(args, "--disallowedTools")
		args = append(args, strings.Join(b.disallowedTools, ","))
	}

	// Permissions
	if b.permissionMode != "" {
		args = append(args, "--permission-mode", b.permissionMode)
	}
	if b.dangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}
	if b.allowDangerouslySkipPermissions {
		args = append(args, "--allow-dangerously-skip-permissions")
	}

	// Input/Output format
	if b.inputFormat != "" {
		args = append(args, "--input-format", b.inputFormat)
	}
	if b.outputFormat != "" {
		args = append(args, "--output-format", b.outputFormat)
	}
	if b.includePartialMessages {
		args = append(args, "--include-partial-messages")
	}
	if b.replayUserMessages {
		args = append(args, "--replay-user-messages")
	}

	// Structured output
	if b.jsonSchema != "" {
		args = append(args, "--json-schema", b.jsonSchema)
	}

	// Budget control
	if b.maxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", b.maxBudgetUSD))
	}

	// MCP configuration
	if len(b.mcpConfigs) > 0 {
		args = append(args, "--mcp-config")
		args = append(args, b.mcpConfigs...)
	}
	if b.strictMcpConfig {
		args = append(args, "--strict-mcp-config")
	}

	// Agents
	if b.agent != "" {
		args = append(args, "--agent", b.agent)
	}
	if b.agentsJSON != nil {
		jsonBytes, _ := json.Marshal(b.agentsJSON)
		args = append(args, "--agents", string(jsonBytes))
	}

	// Additional resources
	if len(b.addDirs) > 0 {
		args = append(args, "--add-dir")
		args = append(args, b.addDirs...)
	}
	if len(b.pluginDirs) > 0 {
		args = append(args, "--plugin-dir")
		args = append(args, b.pluginDirs...)
	}
	if len(b.files) > 0 {
		args = append(args, "--file")
		args = append(args, b.files...)
	}

	// Settings
	if b.settings != "" {
		args = append(args, "--settings", b.settings)
	}
	if len(b.settingSources) > 0 {
		args = append(args, "--setting-sources", strings.Join(b.settingSources, ","))
	}

	// Beta features
	if len(b.betas) > 0 {
		args = append(args, "--betas")
		args = append(args, b.betas...)
	}

	// Misc
	if b.verbose {
		args = append(args, "--verbose")
	}
	if b.debug != "" {
		if b.debug == "true" {
			args = append(args, "--debug")
		} else {
			args = append(args, "--debug", b.debug)
		}
	}
	if b.disableSlashCommands {
		args = append(args, "--disable-slash-commands")
	}

	return args
}
