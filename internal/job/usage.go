package job

import "strings"

// Usage contains aggregated token usage and estimated cost for a run.
// @Description Aggregated token usage and estimated cost
type Usage struct {
	// Input tokens consumed (excluding cached reads)
	InputTokens int `json:"input_tokens"`
	// Output tokens generated
	OutputTokens int `json:"output_tokens"`
	// Tokens written to the prompt cache
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	// Tokens read from the prompt cache
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
	// Sum of all token types
	TotalTokens int `json:"total_tokens"`
	// Estimated cost in USD based on model pricing
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// modelPricing holds per-million-token prices for a model family.
type modelPricing struct {
	inputPerMillion       float64
	outputPerMillion      float64
	cacheCreatePerMillion float64 // prompt cache write (typically 1.25x input)
	cacheReadPerMillion   float64 // prompt cache read (typically 0.1x input)
}

// pricingTable maps model family keywords to their pricing.
// Prices are per 1 million tokens in USD.
var pricingTable = map[string]modelPricing{
	"haiku": {
		inputPerMillion:       0.25,
		outputPerMillion:      1.25,
		cacheCreatePerMillion: 0.30,
		cacheReadPerMillion:   0.03,
	},
	"sonnet": {
		inputPerMillion:       3.00,
		outputPerMillion:      15.00,
		cacheCreatePerMillion: 3.75,
		cacheReadPerMillion:   0.30,
	},
	"opus": {
		inputPerMillion:       15.00,
		outputPerMillion:      75.00,
		cacheCreatePerMillion: 18.75,
		cacheReadPerMillion:   1.50,
	},
}

// estimateCost calculates the estimated USD cost from token counts and model name.
// Returns 0 if the model family is not recognized.
func estimateCost(model string, inputTokens, outputTokens, cacheCreateTokens, cacheReadTokens int) float64 {
	model = strings.ToLower(model)

	var p modelPricing
	var found bool
	if strings.Contains(model, "haiku") {
		p, found = pricingTable["haiku"], true
	} else if strings.Contains(model, "sonnet") {
		p, found = pricingTable["sonnet"], true
	} else if strings.Contains(model, "opus") {
		p, found = pricingTable["opus"], true
	}
	if !found {
		return 0
	}

	const perMillion = 1_000_000.0
	return float64(inputTokens)*p.inputPerMillion/perMillion +
		float64(outputTokens)*p.outputPerMillion/perMillion +
		float64(cacheCreateTokens)*p.cacheCreatePerMillion/perMillion +
		float64(cacheReadTokens)*p.cacheReadPerMillion/perMillion
}

// NewUsage constructs a Usage value from raw token counts and model name,
// computing TotalTokens and EstimatedCostUSD automatically.
func NewUsage(inputTokens, outputTokens, cacheCreationInputTokens, cacheReadInputTokens int, model string) *Usage {
	total := inputTokens + outputTokens + cacheCreationInputTokens + cacheReadInputTokens
	cost := estimateCost(model, inputTokens, outputTokens, cacheCreationInputTokens, cacheReadInputTokens)
	return &Usage{
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheCreationInputTokens: cacheCreationInputTokens,
		CacheReadInputTokens:     cacheReadInputTokens,
		TotalTokens:              total,
		EstimatedCostUSD:         cost,
	}
}
