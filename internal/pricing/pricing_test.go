package pricing

import (
	"math"
	"testing"
)

func TestLookupRates(t *testing.T) {
	cases := []struct {
		model       string
		wantInput   float64
		wantKnown   bool
	}{
		{"claude-haiku-4-5-20251001", 0.25, true},
		{"claude-sonnet-4-6", 3.00, true},
		{"claude-opus-4-7", 15.00, true},
		{"CLAUDE-OPUS-4-7", 15.00, true},
		{"some-future-tier", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		t.Run(c.model, func(t *testing.T) {
			r, ok := LookupRates(c.model)
			if ok != c.wantKnown {
				t.Fatalf("known=%v, want %v", ok, c.wantKnown)
			}
			if r.InputPerMTok != c.wantInput {
				t.Fatalf("InputPerMTok=%v, want %v", r.InputPerMTok, c.wantInput)
			}
		})
	}
}

func TestEstimateUSD(t *testing.T) {
	rates := Rates{
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	}

	t.Run("zero tokens is zero cost", func(t *testing.T) {
		got := EstimateUSD(Tokens{}, rates)
		if got != 0 {
			t.Fatalf("got %v, want 0", got)
		}
	})

	t.Run("input + output", func(t *testing.T) {
		// 1M input + 1M output should equal input + output rates.
		got := EstimateUSD(Tokens{Input: 1_000_000, Output: 1_000_000}, rates)
		want := 18.0
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("cache buckets", func(t *testing.T) {
		// 1M cache write + 1M cache read.
		got := EstimateUSD(Tokens{CacheCreation: 1_000_000, CacheRead: 1_000_000}, rates)
		want := 4.05
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("realistic mix", func(t *testing.T) {
		// 150 input + 42 output + 336 cache write + 18121 cache read at sonnet rates.
		got := EstimateUSD(Tokens{
			Input:         150,
			Output:        42,
			CacheCreation: 336,
			CacheRead:     18121,
		}, rates)
		// Hand-computed:
		// 150*3 + 42*15 + 336*3.75 + 18121*0.3 = 450 + 630 + 1260 + 5436.3 = 7776.3 / 1e6
		want := 0.0077763
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}

func TestEstimateUSD_UnknownModel(t *testing.T) {
	// A zero-value Rates should give zero cost for any token counts, so
	// callers fall back to surfacing tokens without a bogus dollar figure.
	got := EstimateUSD(Tokens{Input: 1_000_000, Output: 1_000_000}, Rates{})
	if got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}
