package claude

import "fmt"

// CommandBuilder builds claude CLI commands with a fluent API
type CommandBuilder struct {
	prompt                     string
	model                      string
	sessionID                  string
	continueSession            bool
	systemPrompt               string
	outputFormat               string
	allowedTools               []string
	disallowedTools            []string
	dangerouslySkipPermissions bool
	maxBudget                  float64
}

// NewCommandBuilder creates a new claude command builder
func NewCommandBuilder() *CommandBuilder {
	return &CommandBuilder{}
}

// WithPrompt sets the prompt to send to Claude
func (b *CommandBuilder) WithPrompt(prompt string) *CommandBuilder {
	b.prompt = prompt
	return b
}

// WithModel sets the model to use (e.g., "opus", "sonnet")
func (b *CommandBuilder) WithModel(model string) *CommandBuilder {
	b.model = model
	return b
}

// WithSessionID sets the session ID to resume
func (b *CommandBuilder) WithSessionID(id string) *CommandBuilder {
	b.sessionID = id
	return b
}

// WithContinue continues the most recent conversation
func (b *CommandBuilder) WithContinue() *CommandBuilder {
	b.continueSession = true
	return b
}

// WithSystemPrompt sets a custom system prompt
func (b *CommandBuilder) WithSystemPrompt(prompt string) *CommandBuilder {
	b.systemPrompt = prompt
	return b
}

// WithOutputFormat sets the output format (text, json, stream-json)
func (b *CommandBuilder) WithOutputFormat(format string) *CommandBuilder {
	b.outputFormat = format
	return b
}

// WithAllowedTools sets the allowed tools
func (b *CommandBuilder) WithAllowedTools(tools ...string) *CommandBuilder {
	b.allowedTools = tools
	return b
}

// WithDisallowedTools sets the disallowed tools
func (b *CommandBuilder) WithDisallowedTools(tools ...string) *CommandBuilder {
	b.disallowedTools = tools
	return b
}

// WithDangerouslySkipPermissions bypasses permission checks
func (b *CommandBuilder) WithDangerouslySkipPermissions() *CommandBuilder {
	b.dangerouslySkipPermissions = true
	return b
}

// WithMaxBudget sets the maximum budget in USD
func (b *CommandBuilder) WithMaxBudget(amount float64) *CommandBuilder {
	b.maxBudget = amount
	return b
}

// Build generates the final claude command
func (b *CommandBuilder) Build() []string {
	args := []string{"claude"}

	// Print mode for non-interactive use
	args = append(args, "-p", b.prompt)

	if b.model != "" {
		args = append(args, "--model", b.model)
	}

	if b.sessionID != "" {
		args = append(args, "--resume", b.sessionID)
	}

	if b.continueSession {
		args = append(args, "--continue")
	}

	if b.systemPrompt != "" {
		args = append(args, "--system-prompt", b.systemPrompt)
	}

	if b.outputFormat != "" {
		args = append(args, "--output-format", b.outputFormat)
	}

	if len(b.allowedTools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, b.allowedTools...)
	}

	if len(b.disallowedTools) > 0 {
		args = append(args, "--disallowedTools")
		args = append(args, b.disallowedTools...)
	}

	if b.dangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	if b.maxBudget > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", b.maxBudget))
	}

	return args
}
