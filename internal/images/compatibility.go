package images

import (
	"path/filepath"
	"sort"
	"strings"
)

// IncompatibleImagePatterns lists images that won't work with Claude CLI image mount.
// These use musl libc (Alpine) or are too minimal (distroless/scratch).
// This is the single source of truth - runner/image.go imports this.
var IncompatibleImagePatterns = []string{
	"*alpine*",
	"*-alpine",
	"alpine:*",
	"alpine",
	"gcr.io/distroless/*",
	"distroless/*",
	"scratch",
	"busybox*",
	"busybox",
}

// ComputeCompatibilityRank determines the compatibility rank for an image.
// Labels are optional - if nil, ranking is based on image name only.
//
// Ranking:
//   - 1: Image has ai.stromboli.compatible=true label
//   - 2: Image has Claude CLI pre-installed (ai.stromboli.claude-cli=true)
//   - 3: Standard glibc-based images (default)
//   - 4: Known incompatible images (Alpine, musl, distroless, etc.)
func ComputeCompatibilityRank(repository, tag string, labels map[string]string) int {
	// Check labels first (if available)
	if labels != nil {
		// Rank 1: Explicitly marked as Stromboli compatible
		if val, ok := labels[LabelCompatible]; ok && val == "true" {
			return RankStromboliCompatible
		}

		// Rank 2: Has Claude CLI pre-installed
		if val, ok := labels[LabelClaudeCLI]; ok && val == "true" {
			return RankClaudeCLI
		}
	}

	// Check for known incompatible patterns
	imageName := repository
	if tag != "" && tag != "latest" {
		imageName = repository + ":" + tag
	}

	if IsIncompatibleImage(imageName) {
		return RankIncompatible
	}

	// Default: assume glibc-based compatibility
	return RankGlibcBased
}

// IsIncompatibleImage checks if an image matches known incompatible patterns.
// This is the exported version for use by other packages (e.g., runner).
func IsIncompatibleImage(imageName string) bool {
	normalizedImage := strings.ToLower(imageName)

	for _, pattern := range IncompatibleImagePatterns {
		// Try glob match on normalized image name
		if matched, _ := filepath.Match(pattern, normalizedImage); matched {
			return true
		}

		// Only substring match non-glob patterns (Issue #6 fix)
		if !strings.ContainsAny(pattern, "*?[]") && strings.Contains(normalizedImage, pattern) {
			return true
		}
	}

	return false
}

// SortByCompatibility sorts images by compatibility rank (best first),
// then alphabetically by repository name for stable ordering.
func SortByCompatibility(images []ImageInfo) {
	sort.Slice(images, func(i, j int) bool {
		// Primary sort by compatibility rank (lower is better)
		if images[i].CompatibilityRank != images[j].CompatibilityRank {
			return images[i].CompatibilityRank < images[j].CompatibilityRank
		}

		// Secondary sort by repository name (alphabetical)
		if images[i].Repository != images[j].Repository {
			return images[i].Repository < images[j].Repository
		}

		// Tertiary sort by tag (alphabetical)
		return images[i].Tag < images[j].Tag
	})
}

// IsCompatible returns true if the image is considered compatible with Claude CLI mount.
// An image is compatible if its rank is 1 (explicitly compatible) or 2 (has CLI) or 3 (glibc-based).
func IsCompatible(rank int) bool {
	return rank <= RankGlibcBased
}

// RankDescription returns a human-readable description of the compatibility rank.
func RankDescription(rank int) string {
	switch rank {
	case RankStromboliCompatible:
		return "Stromboli verified compatible"
	case RankClaudeCLI:
		return "Claude CLI pre-installed"
	case RankGlibcBased:
		return "Standard glibc-based (compatible)"
	case RankIncompatible:
		return "Incompatible (Alpine/musl/distroless)"
	default:
		return "Unknown"
	}
}
