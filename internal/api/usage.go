package api

import (
	"log/slog"
	"math"

	"stromboli/internal/history"
	"stromboli/internal/job"
	"stromboli/internal/pricing"
)

// buildUsage aggregates the session's JSONL token totals and estimates cost.
// Returns nil when the file is missing, no usage was reported, or aggregation
// fails — token usage is a best-effort field on top of the run result, never
// a fatal error.
func buildUsage(reader *history.Reader, sessionID string) *job.Usage {
	if reader == nil || sessionID == "" {
		return nil
	}
	summary, err := reader.AggregateUsage(sessionID)
	if err != nil {
		slog.Debug("usage aggregation skipped", "session_id", sessionID, "error", err)
		return nil
	}
	if summary == nil {
		return nil
	}

	rates, _ := pricing.LookupRates(summary.Model)
	cost := pricing.EstimateUSD(pricing.Tokens{
		Input:         summary.InputTokens,
		Output:        summary.OutputTokens,
		CacheCreation: summary.CacheCreationInputTokens,
		CacheRead:     summary.CacheReadInputTokens,
	}, rates)

	return &job.Usage{
		Model:                    summary.Model,
		InputTokens:              summary.InputTokens,
		OutputTokens:             summary.OutputTokens,
		CacheCreationInputTokens: summary.CacheCreationInputTokens,
		CacheReadInputTokens:     summary.CacheReadInputTokens,
		TotalTokens:              summary.TotalTokens,
		// Round to 6 decimal places so JSON consumers don't see binary noise
		// like 0.00777629999... for clean inputs.
		EstimatedCostUSD: math.Round(cost*1e6) / 1e6,
	}
}
