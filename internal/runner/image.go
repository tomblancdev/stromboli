package runner

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultCLIImageName is the default name of the image containing Claude CLI
// This can be overridden via configuration
const DefaultCLIImageName = "ghcr.io/tomblancdev/stromboli-agent:latest"

// ClaudeCLIImageName is kept for backward compatibility
// Deprecated: Use CLIImageConfig instead
var ClaudeCLIImageName = "stromboli-claude-cli"

// ClaudeCLIMountPath is where the Claude CLI image is mounted inside containers
const ClaudeCLIMountPath = "/opt/claude"

// incompatibleImagePatterns lists images that won't work with Claude CLI image mount
// These use musl libc (Alpine) or are too minimal (distroless/scratch)
var incompatibleImagePatterns = []string{
	"*alpine*",
	"*-alpine",
	"alpine:*",
	"gcr.io/distroless/*",
	"scratch",
	"busybox*",
}

// ImageValidator validates container images against allowed patterns
type ImageValidator struct {
	patterns       []string
	defaultImage   string
	checkCompat    bool // Whether to check for incompatible images
}

// NewImageValidator creates a new image validator
func NewImageValidator(allowedPatterns []string, defaultImage string) *ImageValidator {
	return &ImageValidator{
		patterns:     allowedPatterns,
		defaultImage: defaultImage,
		checkCompat:  false, // Compatibility check disabled by default
	}
}

// NewImageValidatorWithCompatCheck creates a validator that checks for incompatible images
func NewImageValidatorWithCompatCheck(allowedPatterns []string, defaultImage string) *ImageValidator {
	return &ImageValidator{
		patterns:     allowedPatterns,
		defaultImage: defaultImage,
		checkCompat:  true,
	}
}

// Validate checks if an image is allowed and compatible
func (v *ImageValidator) Validate(image string) error {
	if image == "" {
		return nil // Empty means use default
	}

	if !v.IsAllowed(image) {
		return fmt.Errorf("image %q not in allowed patterns", image)
	}

	// Check for known incompatible images (Alpine, distroless, etc.)
	if v.checkCompat && v.IsIncompatible(image) {
		return fmt.Errorf("image %q is incompatible with Claude CLI mount (Alpine/musl-based images not supported, use glibc-based alternatives like debian/slim)", image)
	}

	return nil
}

// IsIncompatible checks if an image is known to be incompatible with Claude CLI mount
// Alpine and musl-based images won't work because the mounted node binary requires glibc
func (v *ImageValidator) IsIncompatible(image string) bool {
	normalizedImage := strings.ToLower(image)
	for _, pattern := range incompatibleImagePatterns {
		if matched, _ := filepath.Match(pattern, normalizedImage); matched {
			return true
		}
		// Also check the tag part
		if strings.Contains(normalizedImage, pattern) {
			return true
		}
	}
	return false
}

// IsAllowed checks if an image matches any allowed pattern
func (v *ImageValidator) IsAllowed(image string) bool {
	// If no patterns configured, allow all (backward compatible)
	if len(v.patterns) == 0 {
		return true
	}

	for _, pattern := range v.patterns {
		if matchImagePattern(pattern, image) {
			return true
		}
	}

	return false
}

// Resolve returns the image to use (request image or default)
func (v *ImageValidator) Resolve(requestImage string) string {
	if requestImage != "" {
		return requestImage
	}
	return v.defaultImage
}

// matchImagePattern matches an image against a pattern
// Supports glob patterns: * matches any sequence of characters
func matchImagePattern(pattern, image string) bool {
	// Normalize: add docker.io/library/ prefix if no registry specified
	normalizedImage := normalizeImageName(image)
	normalizedPattern := normalizeImageName(pattern)

	// Try exact match first
	if normalizedPattern == normalizedImage {
		return true
	}

	// Try glob match
	matched, _ := filepath.Match(normalizedPattern, normalizedImage)
	if matched {
		return true
	}

	// Try matching without normalization (for explicit patterns)
	matched, _ = filepath.Match(pattern, image)
	return matched
}

// normalizeImageName adds default registry/library if not specified
func normalizeImageName(image string) string {
	// Already has registry
	if strings.Contains(image, "/") && (strings.Contains(strings.Split(image, "/")[0], ".") || strings.Contains(strings.Split(image, "/")[0], ":")) {
		return image
	}

	// Has library but no registry (e.g., library/python)
	if strings.HasPrefix(image, "library/") {
		return "docker.io/" + image
	}

	// Just image name (e.g., python:3.12)
	if !strings.Contains(image, "/") {
		return "docker.io/library/" + image
	}

	return image
}
