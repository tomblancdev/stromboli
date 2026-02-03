package images

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// withTimeout wraps a context with DefaultOperationTimeout if no deadline is set
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, DefaultOperationTimeout)
}

// Executor defines the interface for running shell commands.
// This abstraction enables testing without requiring actual Podman installation.
type Executor interface {
	// Run executes a command with the given arguments and returns its output.
	Run(ctx context.Context, args []string) ([]byte, error)
}

// Registry provides operations for Podman images.
// It wraps the podman image command-line interface and provides
// a safe, validated API for managing images.
type Registry struct {
	executor Executor
}

// NewRegistry creates a new images registry with the given executor.
func NewRegistry(executor Executor) *Registry {
	return &Registry{
		executor: executor,
	}
}

// List returns all local images, sorted by compatibility rank.
// Returns an empty slice (never nil) if no images exist.
func (r *Registry) List(ctx context.Context) ([]ImageInfo, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Use Go template format for JSON - Podman outputs one JSON object per line
	output, err := r.executor.Run(ctx, []string{cmdPodman, cmdImages, "--format", "{{json .}}"})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	// Parse newline-delimited JSON
	images := make([]ImageInfo, 0)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var podmanInfo podmanImageInfo
		if err := json.Unmarshal([]byte(line), &podmanInfo); err != nil {
			return nil, fmt.Errorf("failed to parse image info: %w", err)
		}

		// Skip <none> images (dangling images)
		if podmanInfo.Repository == "<none>" {
			continue
		}

		info := ImageInfo{
			ID:         podmanInfo.ID,
			Repository: podmanInfo.Repository,
			Tag:        podmanInfo.Tag,
			Size:       podmanInfo.Size,
			Created:    time.Unix(podmanInfo.Created, 0),
		}

		// Compute compatibility rank based on image name (labels require inspect)
		info.CompatibilityRank = ComputeCompatibilityRank(info.Repository, info.Tag, nil)
		info.Compatible = info.CompatibilityRank <= RankClaudeCLI

		images = append(images, info)
	}

	// Sort by compatibility rank
	SortByCompatibility(images)

	return images, nil
}

// Inspect returns detailed information about a specific image.
// Returns ErrImageNotFound if the image doesn't exist locally.
func (r *Registry) Inspect(ctx context.Context, name string) (*ImageInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: image name cannot be empty", ErrValidation)
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// podman inspect returns JSON array by default
	output, err := r.executor.Run(ctx, []string{cmdPodman, cmdInspect, name, "--type", "image"})
	if err != nil {
		if strings.Contains(string(output), errPatternNotFound) {
			return nil, ErrImageNotFound
		}
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	var results []podmanInspectResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse image info: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrImageNotFound
	}

	result := results[0]

	// Parse repository and tag from RepoTags
	repository, tag := parseRepoTag(result.RepoTags, name)

	// Parse created time
	created, _ := time.Parse(time.RFC3339, result.Created)

	info := &ImageInfo{
		ID:         result.ID,
		Repository: repository,
		Tag:        tag,
		Size:       result.Size,
		Created:    created,
		Labels:     result.Config.Labels,
	}

	// Extract Stromboli-specific labels
	if result.Config.Labels != nil {
		if val, ok := result.Config.Labels[LabelCompatible]; ok {
			info.Compatible = val == "true"
		}
		if val, ok := result.Config.Labels[LabelTools]; ok && val != "" {
			info.Tools = strings.Split(val, ",")
			for i := range info.Tools {
				info.Tools[i] = strings.TrimSpace(info.Tools[i])
			}
		}
		if val, ok := result.Config.Labels[LabelClaudeCLI]; ok {
			info.HasClaudeCLI = val == "true"
		}
		if val, ok := result.Config.Labels[LabelDescription]; ok {
			info.Description = val
		}
	}

	// Compute compatibility rank with full label info
	info.CompatibilityRank = ComputeCompatibilityRank(info.Repository, info.Tag, result.Config.Labels)

	// Override Compatible based on rank if not explicitly set
	if !info.Compatible {
		info.Compatible = info.CompatibilityRank <= RankClaudeCLI
	}

	return info, nil
}

// Search searches registries for images matching the query.
// Returns an empty slice (never nil) if no results are found.
func (r *Registry) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("%w: search query cannot be empty", ErrValidation)
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Build command args
	args := []string{cmdPodman, cmdSearch, query, "--format", "json"}

	if opts != nil {
		if opts.Limit > 0 {
			limit := opts.Limit
			if limit > MaxSearchLimit {
				limit = MaxSearchLimit
			}
			args = append(args, "--limit", strconv.Itoa(limit))
		}
		if opts.NoTrunc {
			args = append(args, "--no-trunc")
		}
	} else {
		// Apply default limit
		args = append(args, "--limit", strconv.Itoa(DefaultSearchLimit))
	}

	output, err := r.executor.Run(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}

	// Parse JSON array output
	var podmanResults []podmanSearchResult
	if err := json.Unmarshal(output, &podmanResults); err != nil {
		// Empty result is valid
		if strings.TrimSpace(string(output)) == "" || strings.TrimSpace(string(output)) == "[]" {
			return []SearchResult{}, nil
		}
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	results := make([]SearchResult, 0, len(podmanResults))
	for _, pr := range podmanResults {
		results = append(results, SearchResult{
			Index:       pr.Index,
			Name:        pr.Name,
			Description: pr.Description,
			Stars:       pr.Stars,
			Official:    pr.Official == "[OK]",
			Automated:   pr.Automated == "[OK]",
		})
	}

	return results, nil
}

// Pull pulls an image from a registry.
// Returns the image ID on success.
func (r *Registry) Pull(ctx context.Context, name string, opts *PullOptions) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: image name cannot be empty", ErrValidation)
	}

	// Use a longer timeout for pull operations
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// Build command args
	args := []string{cmdPodman, cmdPull, name}

	if opts != nil {
		if opts.Quiet {
			args = append(args, "--quiet")
		}
		if opts.Platform != "" {
			args = append(args, "--platform", opts.Platform)
		}
	}

	output, err := r.executor.Run(ctx, args)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPullFailed, err)
	}

	// Output contains the pulled image ID on the last line
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1]), nil
	}

	return "", nil
}

// parseRepoTag extracts repository and tag from RepoTags list or falls back to name
func parseRepoTag(repoTags []string, fallback string) (string, string) {
	if len(repoTags) > 0 {
		parts := strings.Split(repoTags[0], ":")
		if len(parts) >= 2 {
			return strings.Join(parts[:len(parts)-1], ":"), parts[len(parts)-1]
		}
		return repoTags[0], "latest"
	}

	// Parse from fallback name
	parts := strings.Split(fallback, ":")
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], ":"), parts[len(parts)-1]
	}
	return fallback, "latest"
}
