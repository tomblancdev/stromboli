package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeCompatibilityRank_WithLabels(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		tag        string
		labels     map[string]string
		wantRank   int
	}{
		{
			name:       "stromboli compatible label",
			repository: "custom-image",
			tag:        "latest",
			labels:     map[string]string{LabelCompatible: "true"},
			wantRank:   RankStromboliCompatible,
		},
		{
			name:       "claude cli label",
			repository: "stromboli-agent",
			tag:        "v1.0",
			labels:     map[string]string{LabelClaudeCLI: "true"},
			wantRank:   RankClaudeCLI,
		},
		{
			name:       "both labels - compatible wins",
			repository: "custom",
			tag:        "latest",
			labels:     map[string]string{LabelCompatible: "true", LabelClaudeCLI: "true"},
			wantRank:   RankStromboliCompatible,
		},
		{
			name:       "empty labels on glibc image",
			repository: "python",
			tag:        "3.12-slim",
			labels:     map[string]string{},
			wantRank:   RankGlibcBased,
		},
		{
			name:       "empty labels on alpine",
			repository: "alpine",
			tag:        "3.19",
			labels:     map[string]string{},
			wantRank:   RankIncompatible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rank := ComputeCompatibilityRank(tt.repository, tt.tag, tt.labels)
			assert.Equal(t, tt.wantRank, rank)
		})
	}
}

func TestComputeCompatibilityRank_WithoutLabels(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		tag        string
		wantRank   int
	}{
		// Standard glibc-based images
		{"python slim", "python", "3.12-slim", RankGlibcBased},
		{"python bookworm", "python", "3.12-bookworm", RankGlibcBased},
		{"node bookworm", "node", "20-bookworm", RankGlibcBased},
		{"debian", "debian", "bookworm", RankGlibcBased},
		{"ubuntu", "ubuntu", "22.04", RankGlibcBased},
		{"golang", "golang", "1.22", RankGlibcBased},

		// Known incompatible images
		{"alpine base", "alpine", "3.19", RankIncompatible},
		{"alpine latest", "alpine", "latest", RankIncompatible},
		{"python alpine", "python", "3.12-alpine", RankIncompatible},
		{"node alpine", "node", "20-alpine", RankIncompatible},
		{"golang alpine", "golang", "1.22-alpine", RankIncompatible},
		{"distroless", "gcr.io/distroless/base", "latest", RankIncompatible},
		{"scratch", "scratch", "", RankIncompatible},
		{"busybox", "busybox", "latest", RankIncompatible},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rank := ComputeCompatibilityRank(tt.repository, tt.tag, nil)
			assert.Equal(t, tt.wantRank, rank, "expected rank %d for %s:%s", tt.wantRank, tt.repository, tt.tag)
		})
	}
}

func TestIsIncompatible(t *testing.T) {
	tests := []struct {
		imageName  string
		wantResult bool
	}{
		// Incompatible
		{"alpine", true},
		{"alpine:3.19", true},
		{"python:3.12-alpine", true},
		{"node:20-alpine", true},
		{"golang:1.22-alpine", true},
		{"gcr.io/distroless/base", true},
		{"distroless/static", true},
		{"busybox", true},
		{"busybox:latest", true},
		{"scratch", true},
		{"ALPINE", true}, // case insensitive
		{"Python:3.12-Alpine", true},

		// Compatible
		{"python:3.12-slim", false},
		{"python:3.12-bookworm", false},
		{"node:20-bookworm", false},
		{"debian:bookworm", false},
		{"ubuntu:22.04", false},
		{"golang:1.22", false},
		{"custom-image:latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.imageName, func(t *testing.T) {
			result := isIncompatible(tt.imageName)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestSortByCompatibility(t *testing.T) {
	images := []ImageInfo{
		{Repository: "alpine", Tag: "3.19", CompatibilityRank: RankIncompatible},
		{Repository: "python", Tag: "3.12", CompatibilityRank: RankGlibcBased},
		{Repository: "stromboli-agent", Tag: "latest", CompatibilityRank: RankStromboliCompatible},
		{Repository: "node", Tag: "20", CompatibilityRank: RankGlibcBased},
		{Repository: "custom-claude", Tag: "v1", CompatibilityRank: RankClaudeCLI},
	}

	SortByCompatibility(images)

	// Should be sorted: rank 1, rank 2, rank 3 (alpha), rank 3 (alpha), rank 4
	assert.Equal(t, "stromboli-agent", images[0].Repository) // Rank 1
	assert.Equal(t, "custom-claude", images[1].Repository)   // Rank 2
	assert.Equal(t, "node", images[2].Repository)            // Rank 3 (n < p)
	assert.Equal(t, "python", images[3].Repository)          // Rank 3 (n < p)
	assert.Equal(t, "alpine", images[4].Repository)          // Rank 4
}

func TestSortByCompatibility_StableSort(t *testing.T) {
	// Multiple images with same rank and repo should sort by tag
	images := []ImageInfo{
		{Repository: "python", Tag: "3.12", CompatibilityRank: RankGlibcBased},
		{Repository: "python", Tag: "3.11", CompatibilityRank: RankGlibcBased},
		{Repository: "python", Tag: "3.10", CompatibilityRank: RankGlibcBased},
	}

	SortByCompatibility(images)

	assert.Equal(t, "3.10", images[0].Tag)
	assert.Equal(t, "3.11", images[1].Tag)
	assert.Equal(t, "3.12", images[2].Tag)
}

func TestIsCompatible(t *testing.T) {
	assert.True(t, IsCompatible(RankStromboliCompatible))
	assert.True(t, IsCompatible(RankClaudeCLI))
	assert.True(t, IsCompatible(RankGlibcBased))
	assert.False(t, IsCompatible(RankIncompatible))
}

func TestRankDescription(t *testing.T) {
	assert.Equal(t, "Stromboli verified compatible", RankDescription(RankStromboliCompatible))
	assert.Equal(t, "Claude CLI pre-installed", RankDescription(RankClaudeCLI))
	assert.Equal(t, "Standard glibc-based (compatible)", RankDescription(RankGlibcBased))
	assert.Equal(t, "Incompatible (Alpine/musl/distroless)", RankDescription(RankIncompatible))
	assert.Equal(t, "Unknown", RankDescription(99))
}
