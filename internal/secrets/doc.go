// Package secrets provides Podman secret management for Claude credentials.
//
// # Overview
//
// This package manages the Claude credentials securely using Podman secrets.
// The credentials file (~/.claude/.credentials.json) is read from the host
// and stored as a Podman secret, which is then mounted into agent containers.
//
// # Architecture
//
// 1. Stromboli server reads ~/.claude/.credentials.json from host
// 2. Creates/updates a Podman secret named "claude-credentials"
// 3. Agent containers receive the secret mounted at ~/.claude/.credentials.json
// 4. When credentials refresh on host, call UpdateSecret() to sync
//
// # Usage
//
// Create a manager and ensure the secret exists:
//
//	mgr := secrets.NewManagerWithPath("~/.claude/.credentials.json")
//	err := mgr.EnsureExists(ctx, "")
//
// The secret can be mounted into containers using:
//
//	--secret claude-credentials,target=/home/user/.claude/.credentials.json
//
// # Security
//
// Using Podman secrets provides:
//   - Encrypted storage (secrets are not stored in plain text)
//   - Access control (only containers that request the secret can access it)
//   - No exposure in container inspection or logs
//   - Read-only mounting in containers
package secrets
