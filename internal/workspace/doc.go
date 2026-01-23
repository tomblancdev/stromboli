// Package workspace provides workspace path validation and security.
//
// # Overview
//
// Workspaces are host directories mounted into containers for Claude to access.
// This package ensures only approved directories can be mounted, preventing
// unauthorized access to the host filesystem.
//
// # Usage
//
// Create a validator with allowed paths:
//
//	v := workspace.NewValidator([]string{
//	    "/home/user/projects",
//	    "/var/data/repos",
//	})
//
// Validate a workspace path:
//
//	if err := v.Validate("/home/user/projects/myapp"); err != nil {
//	    // Path not allowed
//	}
//
// # Security
//
// The validator checks that:
//   - The path exists on the host
//   - The path is under one of the allowed base directories
//   - Symlinks are resolved to prevent traversal attacks
//   - Empty allowed list means all paths are allowed (use with caution)
//
// # Path Normalization
//
// Paths are cleaned and resolved before validation:
//   - Relative paths are made absolute
//   - Symlinks are resolved
//   - ".." components are resolved
package workspace
