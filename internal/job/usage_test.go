package job

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUsage_BasicFields(t *testing.T) {
	u := NewUsage(100, 50, 20, 10, "claude-sonnet-4-5-20251101")

	assert.Equal(t, 100, u.InputTokens)
	assert.Equal(t, 50, u.OutputTokens)
	assert.Equal(t, 20, u.CacheCreationInputTokens)
	assert.Equal(t, 10, u.CacheReadInputTokens)
	assert.Equal(t, 180, u.TotalTokens) // 100+50+20+10
}

func TestNewUsage_TotalTokens(t *testing.T) {
	u := NewUsage(1000, 200, 300, 400, "claude-haiku-4-5")
	assert.Equal(t, 1900, u.TotalTokens)
}

func TestEstimateCost_Haiku(t *testing.T) {
	// 1M input tokens at $0.25 = $0.25
	cost := estimateCost("claude-haiku-4-5-20251001", 1_000_000, 0, 0, 0)
	assert.InDelta(t, 0.25, cost, 1e-9)

	// 1M output tokens at $1.25 = $1.25
	cost = estimateCost("claude-haiku-4-5", 0, 1_000_000, 0, 0)
	assert.InDelta(t, 1.25, cost, 1e-9)
}

func TestEstimateCost_Sonnet(t *testing.T) {
	// 1M input tokens at $3.00
	cost := estimateCost("claude-sonnet-4-5-20251101", 1_000_000, 0, 0, 0)
	assert.InDelta(t, 3.00, cost, 1e-9)

	// 1M output tokens at $15.00
	cost = estimateCost("claude-sonnet-4-5", 0, 1_000_000, 0, 0)
	assert.InDelta(t, 15.00, cost, 1e-9)
}

func TestEstimateCost_Opus(t *testing.T) {
	// 1M input tokens at $15.00
	cost := estimateCost("claude-opus-4-5-20251101", 1_000_000, 0, 0, 0)
	assert.InDelta(t, 15.00, cost, 1e-9)

	// 1M output tokens at $75.00
	cost = estimateCost("claude-opus-4-5", 0, 1_000_000, 0, 0)
	assert.InDelta(t, 75.00, cost, 1e-9)
}

func TestEstimateCost_CacheTokens(t *testing.T) {
	// 1M cache creation tokens at $3.75 (sonnet)
	cost := estimateCost("claude-sonnet-4-5", 0, 0, 1_000_000, 0)
	assert.InDelta(t, 3.75, cost, 1e-9)

	// 1M cache read tokens at $0.30 (sonnet)
	cost = estimateCost("claude-sonnet-4-5", 0, 0, 0, 1_000_000)
	assert.InDelta(t, 0.30, cost, 1e-9)
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	cost := estimateCost("gpt-4", 1_000_000, 1_000_000, 0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestEstimateCost_EmptyModel(t *testing.T) {
	cost := estimateCost("", 1_000_000, 1_000_000, 0, 0)
	assert.Equal(t, 0.0, cost)
}

func TestNewUsage_CostPopulated(t *testing.T) {
	// 100k input + 10k output with sonnet
	u := NewUsage(100_000, 10_000, 0, 0, "claude-sonnet-4-5")

	expectedCost := 100_000*3.00/1_000_000 + 10_000*15.00/1_000_000
	assert.InDelta(t, expectedCost, u.EstimatedCostUSD, 1e-9)
	assert.False(t, math.IsNaN(u.EstimatedCostUSD))
}

func TestNewUsage_ZeroTokens(t *testing.T) {
	u := NewUsage(0, 0, 0, 0, "claude-sonnet-4-5")
	assert.Equal(t, 0, u.TotalTokens)
	assert.Equal(t, 0.0, u.EstimatedCostUSD)
}
