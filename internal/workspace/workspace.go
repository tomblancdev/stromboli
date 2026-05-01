package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Validator validates workspace paths against an allowlist
type Validator struct {
	allowedPaths    []string
	allowAllVolumes bool // Explicit opt-in to allow all paths (dangerous!)
}

// NewValidator creates a workspace validator with allowed paths
// SECURITY: When allowedPaths is empty and allowAllVolumes is false (default),
// all volume mounts are DENIED. This is the secure default.
func NewValidator(allowedPaths []string) *Validator {
	return &Validator{
		allowedPaths:    allowedPaths,
		allowAllVolumes: false,
	}
}

// NewValidatorWithAllowAll creates a validator that allows all paths when allowlist is empty
// WARNING: This is dangerous and should only be used in development!
func NewValidatorWithAllowAll(allowedPaths []string, allowAll bool) *Validator {
	return &Validator{
		allowedPaths:    allowedPaths,
		allowAllVolumes: allowAll,
	}
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

	// Resolve symlinks to prevent symlink-based traversal attacks. We tolerate
	// a missing target (the workspace is created later) but reject every other
	// EvalSymlinks failure — symlink loops, permission errors, or unreadable
	// intermediate directories are exactly the kind of edge cases an attacker
	// would use to bypass the allowlist by making validation fall back to the
	// pre-resolution path.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("cannot validate workspace path %q: %w", path, err)
		}
		if !filepath.IsAbs(absPath) {
			return "", fmt.Errorf("path must be absolute after resolution")
		}
		resolvedPath = absPath
	}
	resolvedPath = filepath.Clean(resolvedPath)

	// SECURITY: If no allowlist configured, check allowAllVolumes flag
	if len(v.allowedPaths) == 0 {
		if v.allowAllVolumes {
			return resolvedPath, nil
		}
		return "", fmt.Errorf("no volume allowlist configured; set allowed_volumes or enable allow_all_volumes for development")
	}

	// Check if resolved path is under any allowed path
	for _, allowed := range v.allowedPaths {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		allowedAbs = filepath.Clean(allowedAbs)

		// Also resolve symlinks in allowed paths for consistent comparison
		allowedResolved, err := filepath.EvalSymlinks(allowedAbs)
		if err != nil {
			allowedResolved = allowedAbs
		}
		allowedResolved = filepath.Clean(allowedResolved)

		// Check if resolvedPath is under allowedResolved
		if resolvedPath == allowedResolved || strings.HasPrefix(resolvedPath, allowedResolved+string(filepath.Separator)) {
			return resolvedPath, nil
		}
	}

	return "", fmt.Errorf("workspace path %q is not in allowed directories", path)
}

// IsConfigured returns true if allowlist is configured
func (v *Validator) IsConfigured() bool {
	return len(v.allowedPaths) > 0
}

// AllowsAll returns true if the validator allows all paths (dangerous mode)
func (v *Validator) AllowsAll() bool {
	return v.allowAllVolumes && len(v.allowedPaths) == 0
}
