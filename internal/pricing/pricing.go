// Package pricing converts token usage into a USD cost estimate based on
// published Anthropic API rates. The rates here are starting points; operators
// who run on enterprise tiers or negotiated contracts should override them.
package pricing

import "strings"

// Rates holds per-model token pricing in USD per 1M tokens.
// CacheWrite and CacheRead are derived from input prices using Anthropic's
// public multipliers (cache writes ≈ 1.25× input, cache reads ≈ 0.1× input)
// when not overridden.
type Rates struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// modelPrefixRates maps a lowercase substring of the model ID to its rates.
// Entries are checked in order — list more specific prefixes first when the
// table grows beyond the three current families.
var modelPrefixRates = []struct {
	prefix string
	rates  Rates
}{
	{prefix: "haiku", rates: Rates{
		InputPerMTok:      0.25,
		OutputPerMTok:     1.25,
		CacheWritePerMTok: 0.30,
		CacheReadPerMTok:  0.025,
	}},
	{prefix: "sonnet", rates: Rates{
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	}},
	{prefix: "opus", rates: Rates{
		InputPerMTok:      15.00,
		OutputPerMTok:     75.00,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.50,
	}},
}

// LookupRates returns the Rates for the given model identifier, plus a
// boolean indicating whether the model was recognized. Unknown models get
// a zero-value Rates so callers can still surface token counts without
// emitting a misleading dollar figure.
func LookupRates(model string) (Rates, bool) {
	m := strings.ToLower(model)
	for _, entry := range modelPrefixRates {
		if strings.Contains(m, entry.prefix) {
			return entry.rates, true
		}
	}
	return Rates{}, false
}

// Tokens is a flat token-count snapshot, mirroring the fields Claude emits
// per assistant message. The pricing package keeps a local copy of this shape
// so it doesn't depend on the history package and can be reused by other
// callers (e.g. webhook payloads).
type Tokens struct {
	Input         int
	Output        int
	CacheCreation int
	CacheRead     int
}

// EstimateUSD returns the USD cost implied by the given token counts under
// the given rates. All four token buckets contribute. Result is in dollars,
// rounded by the caller (we keep full precision here).
func EstimateUSD(t Tokens, r Rates) float64 {
	const perMillion = 1_000_000.0
	return float64(t.Input)*r.InputPerMTok/perMillion +
		float64(t.Output)*r.OutputPerMTok/perMillion +
		float64(t.CacheCreation)*r.CacheWritePerMTok/perMillion +
		float64(t.CacheRead)*r.CacheReadPerMTok/perMillion
}
