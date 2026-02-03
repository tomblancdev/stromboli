// Package images provides a thin wrapper around Podman's image management commands.
// It enables listing, inspecting, searching, and pulling container images
// with built-in compatibility detection for Claude CLI environments.
//
// Images are sorted by compatibility ranking to help users choose
// the most suitable base images for their Claude agents.
package images

import (
	"errors"
	"time"
)

// Domain errors
var (
	// ErrImageNotFound is returned when an image doesn't exist locally
	ErrImageNotFound = errors.New("image not found")

	// ErrSearchFailed is returned when a registry search fails
	ErrSearchFailed = errors.New("search failed")

	// ErrPullFailed is returned when pulling an image fails
	ErrPullFailed = errors.New("pull failed")

	// ErrValidation is returned when input validation fails
	ErrValidation = errors.New("validation error")
)

// Stromboli label constants for image compatibility detection
const (
	// LabelCompatible indicates the image is verified compatible with Stromboli
	LabelCompatible = "ai.stromboli.compatible"

	// LabelTools lists the tools available in the image (comma-separated)
	LabelTools = "ai.stromboli.tools"

	// LabelClaudeCLI indicates the image has Claude CLI pre-installed
	LabelClaudeCLI = "ai.stromboli.claude-cli"

	// LabelDescription provides a human-readable description
	LabelDescription = "ai.stromboli.description"
)

// Compatibility ranks for sorting images
const (
	// RankStromboliCompatible - highest rank for images with ai.stromboli.compatible=true
	RankStromboliCompatible = 1

	// RankClaudeCLI - images with Claude CLI pre-installed
	RankClaudeCLI = 2

	// RankGlibcBased - standard glibc-based images (debian, ubuntu, etc.)
	RankGlibcBased = 3

	// RankIncompatible - known incompatible images (Alpine, musl, distroless)
	RankIncompatible = 4
)

// Constants
const (
	// DefaultOperationTimeout is the default timeout for Podman operations
	DefaultOperationTimeout = 30 * time.Second

	// DefaultSearchLimit is the default number of search results
	DefaultSearchLimit = 25

	// MaxSearchLimit is the maximum allowed search limit
	MaxSearchLimit = 100

	// MaxImageNameLength is the maximum allowed length for image names
	MaxImageNameLength = 255

	// Command constants
	cmdPodman  = "podman"
	cmdImages  = "images"
	cmdInspect = "inspect"
	cmdSearch  = "search"
	cmdPull    = "pull"

	// Default registry for search queries without explicit registry
	defaultSearchRegistry = "docker.io"

	// Error detection patterns
	errPatternNotFound = "image not known"
)

// ImageInfo contains metadata about a local container image
type ImageInfo struct {
	// ID is the unique image identifier
	ID string `json:"id"`

	// Repository is the image repository name (e.g., "docker.io/library/python")
	Repository string `json:"repository"`

	// Tag is the image tag (e.g., "3.12-slim")
	Tag string `json:"tag"`

	// Size is the image size in bytes
	Size int64 `json:"size"`

	// Created is when the image was created
	Created time.Time `json:"created"`

	// Labels contains all image labels
	Labels map[string]string `json:"labels,omitempty"`

	// CompatibilityRank indicates how compatible the image is with Stromboli (1=best, 4=worst)
	CompatibilityRank int `json:"compatibility_rank"`

	// Compatible indicates if the image is verified compatible
	Compatible bool `json:"compatible"`

	// Tools lists any pre-installed tools detected via labels
	Tools []string `json:"tools,omitempty"`

	// HasClaudeCLI indicates if Claude CLI is pre-installed
	HasClaudeCLI bool `json:"has_claude_cli"`

	// Description is a human-readable description from labels
	Description string `json:"description,omitempty"`
}

// podmanImageInfo represents the JSON output from `podman images --format '{{json .}}'`
type podmanImageInfo struct {
	ID         string   `json:"Id"`
	Repository string   `json:"Repository"`
	Tag        string   `json:"Tag"`
	Size       int64    `json:"Size"`
	Created    int64    `json:"Created"` // Unix timestamp
	Names      []string `json:"Names"`
}

// podmanInspectResult represents the JSON output from `podman inspect --type image`
type podmanInspectResult struct {
	ID      string            `json:"Id"`
	Created string            `json:"Created"`
	Size    int64             `json:"Size"`
	Config  podmanImageConfig `json:"Config"`
	RepoTags []string         `json:"RepoTags"`
}

// podmanImageConfig represents the Config section of image inspect output
type podmanImageConfig struct {
	Labels map[string]string `json:"Labels"`
}

// SearchResult represents a search result from a registry
type SearchResult struct {
	// Index is the registry index (e.g., "docker.io")
	Index string `json:"index"`

	// Name is the image name
	Name string `json:"name"`

	// Description is the image description
	Description string `json:"description"`

	// Stars is the number of stars/likes
	Stars int `json:"stars"`

	// Official indicates if it's an official image
	Official bool `json:"official"`

	// Automated indicates if the build is automated
	Automated bool `json:"automated"`
}

// podmanSearchResult represents the JSON output from `podman search --format json`
type podmanSearchResult struct {
	Index       string `json:"Index"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Stars       int    `json:"Stars"`
	Official    string `json:"Official"`
	Automated   string `json:"Automated"`
}

// SearchOptions configures a registry search operation
type SearchOptions struct {
	// Limit is the maximum number of results to return
	Limit int

	// NoTrunc prevents truncation of output
	NoTrunc bool
}

// PullOptions configures an image pull operation
type PullOptions struct {
	// Quiet suppresses pull progress output
	Quiet bool

	// Platform specifies the platform to pull for (e.g., "linux/amd64")
	Platform string
}
