package runner

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ClaudeCLIVolumeName is the name of the Podman volume containing Claude CLI
const ClaudeCLIVolumeName = "claude-cli"

// ClaudeCLIPath is where Claude CLI is mounted inside containers
const ClaudeCLIPath = "/usr/local/bin"

// ImageValidator validates container images against allowed patterns
type ImageValidator struct {
	patterns     []string
	defaultImage string
}

// NewImageValidator creates a new image validator
func NewImageValidator(allowedPatterns []string, defaultImage string) *ImageValidator {
	return &ImageValidator{
		patterns:     allowedPatterns,
		defaultImage: defaultImage,
	}
}

// Validate checks if an image is allowed
func (v *ImageValidator) Validate(image string) error {
	if image == "" {
		return nil // Empty means use default
	}

	if !v.IsAllowed(image) {
		return fmt.Errorf("image %q not in allowed patterns", image)
	}

	return nil
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
