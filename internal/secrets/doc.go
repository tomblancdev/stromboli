// Package secrets provides Podman secret management for secure token handling.
//
// # Overview
//
// Podman secrets are the recommended way to pass sensitive data to containers.
// This package wraps Podman's secret commands to create and manage secrets
// that can be mounted into containers at /run/secrets/<name>.
//
// # Usage
//
// Create or update a secret from a file:
//
//	mgr := secrets.NewManager(executor)
//	err := mgr.EnsureSecret("claude-token", "/path/to/secrets-file")
//
// The secret is created if it doesn't exist, or updated if the content changed.
// Secrets are available to containers via --secret flag.
//
// # Security
//
// Secrets are:
//   - Stored encrypted by Podman
//   - Only accessible to containers that explicitly request them
//   - Mounted read-only at /run/secrets/<name>
//   - Not visible in container inspection or logs
package secrets
