package runner

import (
	"fmt"
	"strings"
	"time"

	"stromboli/internal/types"
)

// Validation limits for lifecycle hooks
const (
	// MaxHookCommandArgs is the maximum number of arguments in a single hook command
	MaxHookCommandArgs = 100
	// MaxHookArgLength is the maximum length of a single hook argument
	MaxHookArgLength = 4096
	// MaxTotalHookLength is the maximum total length of all hook arguments combined
	MaxTotalHookLength = 65536
	// MaxTotalHookArgs is the maximum total number of arguments across all hooks
	MaxTotalHookArgs = 200
)

// ValidateLifecycleHooks validates lifecycle hook inputs for security and sanity.
// Returns an error if any hook exceeds the defined limits.
func ValidateLifecycleHooks(hooks types.LifecycleHooks) error {
	totalLength := 0
	totalArgs := 0

	// Validate each hook type
	if err := validateHookCommand("onCreateCommand", hooks.OnCreateCommand, &totalLength, &totalArgs); err != nil {
		return err
	}
	if err := validateHookCommand("postCreate", hooks.PostCreate, &totalLength, &totalArgs); err != nil {
		return err
	}
	if err := validateHookCommand("postStart", hooks.PostStart, &totalLength, &totalArgs); err != nil {
		return err
	}

	if totalLength > MaxTotalHookLength {
		return fmt.Errorf("total hook length %d exceeds maximum %d", totalLength, MaxTotalHookLength)
	}

	if totalArgs > MaxTotalHookArgs {
		return fmt.Errorf("total hook arguments %d exceeds maximum %d", totalArgs, MaxTotalHookArgs)
	}

	// Validate hooks_timeout if specified
	if hooks.HooksTimeout != "" {
		duration, err := time.ParseDuration(hooks.HooksTimeout)
		if err != nil {
			return fmt.Errorf("invalid hooks_timeout %q: %w", hooks.HooksTimeout, err)
		}
		if duration <= 0 {
			return fmt.Errorf("hooks_timeout must be positive, got %q", hooks.HooksTimeout)
		}
	}

	return nil
}

// validateHookCommand validates a single hook command
func validateHookCommand(name string, cmd []string, totalLength *int, totalArgs *int) error {
	// Skip nil slices (valid - no hook configured)
	if cmd == nil {
		return nil
	}

	// Reject non-nil but empty slices (e.g., []string{})
	// This would produce invalid "sh -c ''" commands
	if len(cmd) == 0 {
		return fmt.Errorf("%s: hook configured but empty (must have at least one command)", name)
	}

	if len(cmd) > MaxHookCommandArgs {
		return fmt.Errorf("%s: too many arguments (%d > %d)", name, len(cmd), MaxHookCommandArgs)
	}

	for i, arg := range cmd {
		// Check for empty arguments which would produce invalid shell commands
		if arg == "" {
			return fmt.Errorf("%s: argument %d is empty", name, i)
		}
		// Check for null bytes which would be silently truncated by the shell
		if strings.ContainsAny(arg, "\x00") {
			return fmt.Errorf("%s: argument %d contains null byte", name, i)
		}
		if len(arg) > MaxHookArgLength {
			return fmt.Errorf("%s: argument %d too long (%d > %d)", name, i, len(arg), MaxHookArgLength)
		}
		*totalLength += len(arg)
	}

	*totalArgs += len(cmd)

	return nil
}

// HasAnyHooks returns true if any lifecycle hooks are configured
func HasAnyHooks(hooks types.LifecycleHooks) bool {
	return len(hooks.OnCreateCommand) > 0 || len(hooks.PostCreate) > 0 || len(hooks.PostStart) > 0
}

// HasInitHooks returns true if any init-time hooks are configured (onCreateCommand or postCreate)
func HasInitHooks(hooks types.LifecycleHooks) bool {
	return len(hooks.OnCreateCommand) > 0 || len(hooks.PostCreate) > 0
}

// WrapCommandWithHooks wraps the main command with lifecycle hook execution.
// This is used when we can't use podman exec (e.g., with --rm containers).
// The returned command will:
// 1. Run init hooks if runInit is true
// 2. Run postStart hooks
// 3. Run the main command
//
// If HooksTimeout is specified, hooks (but not the main command) are wrapped
// with the `timeout` command to enforce a time limit.
//
// REQUIREMENT: Container images must have a POSIX-compatible /bin/sh shell.
// Hooks are executed using `sh -c` which requires this shell to be present.
// Alpine-based images typically use busybox ash which is compatible.
// Distroless images may need modification to include a shell.
func WrapCommandWithHooks(mainCmd []string, hooks types.LifecycleHooks, runInit bool) []string {
	if !HasAnyHooks(hooks) {
		return mainCmd
	}

	var hookCommands []string

	// Add init hooks (only on first run)
	if runInit {
		if len(hooks.OnCreateCommand) > 0 {
			hookCommands = append(hookCommands, escapeShellCommand(hooks.OnCreateCommand))
		}
		if len(hooks.PostCreate) > 0 {
			hookCommands = append(hookCommands, escapeShellCommand(hooks.PostCreate))
		}
	}

	// Add postStart hook (every run)
	if len(hooks.PostStart) > 0 {
		hookCommands = append(hookCommands, escapeShellCommand(hooks.PostStart))
	}

	// No hooks to run, return original command
	if len(hookCommands) == 0 {
		return mainCmd
	}

	// Build the final shell command
	var shellCmd string
	mainCmdEscaped := escapeShellCommand(mainCmd)

	if hooks.HooksTimeout != "" {
		// Apply timeout to hooks only (not to main command)
		// HooksTimeout has already been validated by ValidateLifecycleHooks
		timeout, _ := time.ParseDuration(hooks.HooksTimeout)
		hooksShellCmd := strings.Join(hookCommands, " && ")
		// timeout with --kill-after sends SIGKILL after grace period if process doesn't terminate
		shellCmd = fmt.Sprintf("timeout --kill-after=10s %.0fs sh -c %s && %s",
			timeout.Seconds(), EscapeShellArg(hooksShellCmd), mainCmdEscaped)
	} else {
		// No timeout - join hooks and main command with &&
		allCommands := append(hookCommands, mainCmdEscaped)
		shellCmd = strings.Join(allCommands, " && ")
	}

	return []string{"sh", "-c", shellCmd}
}

// EscapeShellArg escapes a single argument for safe shell execution using single quotes.
//
// This function uses single-quote escaping which is the safest approach for shell:
// - Single quotes preserve all characters literally except single quotes themselves
// - Single quotes inside are escaped by ending the quote, adding an escaped quote, and reopening
// - Example: "it's" becomes "'it'\''s'" (end quote, escaped quote, reopen quote)
//
// This approach is safe against:
// - Variable expansion ($VAR, ${VAR})
// - Command substitution ($(cmd), `cmd`)
// - Glob expansion (*, ?, [...])
// - Special characters (;, |, &, <, >, newlines, etc.)
// - Backslashes (treated literally inside single quotes)
//
// Note: Null bytes in arguments would be truncated by the shell - callers should
// validate inputs before calling this function.
func EscapeShellArg(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

// escapeShellCommand escapes a command array for safe shell execution.
// It joins all arguments with spaces after escaping each one with EscapeShellArg.
func escapeShellCommand(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}

	escaped := make([]string, 0, len(cmd)) // Pre-allocate for efficiency
	for _, arg := range cmd {
		escaped = append(escaped, EscapeShellArg(arg))
	}

	return strings.Join(escaped, " ")
}
