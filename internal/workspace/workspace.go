package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Validator validates workspace paths against an allowlist
type Validator struct {
	allowedPaths []string
}

// NewValidator creates a workspace validator with allowed paths
// If allowedPaths is empty, all paths are allowed (backward compatible)
func NewValidator(allowedPaths []string) *Validator {
	return &Validator{allowedPaths: allowedPaths}
}

// Validate checks if the workspace path is allowed
// Returns the cleaned absolute path if valid, or error if not
func (v *Validator) Validate(path string) (string, error) {
	if path == "" {
		return "", nil // Empty path is valid (no workspace)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid workspace path: %w", err)
	}

	// Clean the path to prevent traversal
	absPath = filepath.Clean(absPath)

	// If no allowlist, all paths are allowed
	if len(v.allowedPaths) == 0 {
		return absPath, nil
	}

	// Check if path is under any allowed path
	for _, allowed := range v.allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		allowedAbs = filepath.Clean(allowedAbs)

		// Check if absPath is under allowedAbs
		if absPath == allowedAbs || strings.HasPrefix(absPath, allowedAbs+string(filepath.Separator)) {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("workspace path %q is not in allowed directories", path)
}

// IsConfigured returns true if allowlist is configured
func (v *Validator) IsConfigured() bool {
	return len(v.allowedPaths) > 0
}
