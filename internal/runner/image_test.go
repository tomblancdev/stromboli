package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageValidator_IsAllowed(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		image    string
		expected bool
	}{
		{
			name:     "no patterns allows all",
			patterns: []string{},
			image:    "anything:latest",
			expected: true,
		},
		{
			name:     "exact match",
			patterns: []string{"python:3.12"},
			image:    "python:3.12",
			expected: true,
		},
		{
			name:     "wildcard tag",
			patterns: []string{"python:*"},
			image:    "python:3.12",
			expected: true,
		},
		{
			name:     "wildcard image",
			patterns: []string{"docker.io/library/*"},
			image:    "docker.io/library/python:3.12",
			expected: true,
		},
		{
			name:     "short name matches docker.io pattern",
			patterns: []string{"docker.io/library/*"},
			image:    "python:3.12",
			expected: true,
		},
		{
			name:     "golang pattern",
			patterns: []string{"golang:*"},
			image:    "golang:1.22",
			expected: true,
		},
		{
			name:     "not allowed",
			patterns: []string{"python:*"},
			image:    "golang:1.22",
			expected: false,
		},
		{
			name:     "private registry allowed",
			patterns: []string{"ghcr.io/myorg/*"},
			image:    "ghcr.io/myorg/myimage:v1",
			expected: true,
		},
		{
			name:     "stromboli-agent pattern",
			patterns: []string{"stromboli-agent*"},
			image:    "stromboli-agent:latest",
			expected: true,
		},
		{
			name:     "stromboli-agent-go pattern",
			patterns: []string{"stromboli-agent*"},
			image:    "stromboli-agent-go:1.22",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewImageValidator(tt.patterns, "default:latest")
			result := v.IsAllowed(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageValidator_Validate(t *testing.T) {
	v := NewImageValidator([]string{"python:*", "golang:*"}, "python:3.12")

	t.Run("empty image is valid (uses default)", func(t *testing.T) {
		err := v.Validate("")
		assert.NoError(t, err)
	})

	t.Run("allowed image is valid", func(t *testing.T) {
		err := v.Validate("python:3.11")
		assert.NoError(t, err)
	})

	t.Run("disallowed image returns error", func(t *testing.T) {
		err := v.Validate("rust:latest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed patterns")
	})
}

func TestImageValidator_Resolve(t *testing.T) {
	v := NewImageValidator([]string{}, "default:latest")

	t.Run("returns request image when provided", func(t *testing.T) {
		result := v.Resolve("custom:v1")
		assert.Equal(t, "custom:v1", result)
	})

	t.Run("returns default when request empty", func(t *testing.T) {
		result := v.Resolve("")
		assert.Equal(t, "default:latest", result)
	})
}

func TestNormalizeImageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"python:3.12", "docker.io/library/python:3.12"},
		{"library/python:3.12", "docker.io/library/python:3.12"},
		{"docker.io/library/python:3.12", "docker.io/library/python:3.12"},
		{"ghcr.io/org/image:v1", "ghcr.io/org/image:v1"},
		{"localhost:5000/myimage:latest", "localhost:5000/myimage:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeImageName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "stromboli-claude-cli", ClaudeCLIImageName)
	assert.Equal(t, "/opt/claude", ClaudeCLIMountPath)
}

func TestImageValidator_IsIncompatible(t *testing.T) {
	v := NewImageValidatorWithCompatCheck([]string{"*"}, "default:latest")

	tests := []struct {
		image        string
		incompatible bool
	}{
		{"alpine:latest", true},
		{"alpine:3.18", true},
		{"python:3.12-alpine", true},
		{"python:alpine", true},
		{"node:20-alpine", true},
		{"gcr.io/distroless/base", true},
		{"busybox:latest", true},
		{"scratch", true},
		// Compatible images
		{"python:3.12", false},
		{"python:3.12-slim", false},
		{"node:20-slim", false},
		{"debian:bookworm", false},
		{"ubuntu:22.04", false},
		{"golang:1.22", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			result := v.IsIncompatible(tt.image)
			assert.Equal(t, tt.incompatible, result, "image: %s", tt.image)
		})
	}
}

func TestImageValidator_ValidateWithCompatCheck(t *testing.T) {
	v := NewImageValidatorWithCompatCheck([]string{"*"}, "default:latest")

	t.Run("rejects alpine images", func(t *testing.T) {
		err := v.Validate("python:3.12-alpine")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incompatible")
	})

	t.Run("accepts glibc images", func(t *testing.T) {
		err := v.Validate("python:3.12-slim")
		assert.NoError(t, err)
	})
}
