// Package types defines shared data types used across packages.
//
// # Overview
//
// This package contains data structures that are used by multiple packages
// to avoid circular dependencies. It includes request/response types and
// configuration options.
//
// # Types
//
// ClaudeOptions: Configuration for Claude CLI execution
//   - Model selection (opus, sonnet, haiku)
//   - Session management (ID, resume, fork)
//   - Permission modes and allowed tools
//   - System prompt customization
//   - Output format (text, json, stream-json)
//
// PodmanOptions: Configuration for container execution
//   - Resource limits (memory, CPU, timeout)
//   - Network settings
//   - Environment variables
//   - Volume mounts
//
// # Usage
//
//	req := types.ClaudeOptions{
//	    Model:          "sonnet",
//	    SessionID:      "sess-123",
//	    Resume:         true,
//	    PermissionMode: "bypassPermissions",
//	}
package types
