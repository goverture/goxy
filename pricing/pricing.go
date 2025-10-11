package pricing

import "fmt"

// Model represents a supported model for pricing.
type Model string

const (
	ModelGPT4     Model = "gpt-4"
	ModelGPT4o    Model = "gpt-4o"
	ModelGPT5Mini Model = "gpt-5-mini"
	ModelGPT5     Model = "gpt-5" // placeholder name
)

// Usage contains token usage fields we care about.
type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	// PromptCachedTokens are prompt tokens served from cache and billed at a 90% discount
	// (i.e. they cost 10% of a normal prompt token). If this exceeds PromptTokens it will
	// be clamped to PromptTokens.
	PromptCachedTokens int `json:"cached_prompt_tokens"`
	CompletionTokens   int `json:"completion_tokens"`
	TotalTokens        int `json:"total_tokens"`
}

// PriceResult holds the computed pricing info.
type PriceResult struct {
	Model             Model
	PromptTokens      int
	CompletionTokens  int
	PromptCostUSD     float64
	CompletionCostUSD float64
	TotalCostUSD      float64
	Note              string
}

// getPricing returns the pricing for a given model from configuration
func getPricing(model Model) (struct{ Prompt, Completion float64 }, error) {
	cfg, err := GetConfig()
	if err != nil {
		return struct{ Prompt, Completion float64 }{}, fmt.Errorf("failed to load pricing config: %w", err)
	}

	pricing, found := cfg.FindModelPricing(string(model))
	if found {
		return struct{ Prompt, Completion float64 }{
			Prompt:     pricing.Prompt,
			Completion: pricing.Completion,
		}, nil
	}

	// Fallback to default if configured
	if cfg.Default != nil {
		return struct{ Prompt, Completion float64 }{
			Prompt:     cfg.Default.Prompt,
			Completion: cfg.Default.Completion,
		}, nil
	}

	return struct{ Prompt, Completion float64 }{}, fmt.Errorf("no pricing found for model %s", model)
}

// NormalizeModel tries to map a raw model string to one of our known buckets.
func NormalizeModel(raw string) Model {
	switch {
	case raw == "gpt-4" || raw == "gpt-4-0613":
		return ModelGPT4
	case raw == "gpt-4o" || raw == "gpt-4o-2024-08-06":
		return ModelGPT4o
	case raw == "gpt-5-mini" || raw == "gpt-5-mini-2025-08-07":
		return ModelGPT5Mini
	case raw == "gpt-5" || raw == "gpt-5-2025-08-07":
		return ModelGPT5
	default:
		// naive fallback: detect prefix
		if len(raw) >= 5 && raw[:5] == "gpt-4" {
			return ModelGPT4
		}
		if len(raw) >= 5 && raw[:5] == "gpt-5" {
			return ModelGPT5
		}
		return Model(raw)
	}
}

// ComputePrice calculates cost given usage and model.
func ComputePrice(modelRaw string, u Usage) (PriceResult, error) {
	m := NormalizeModel(modelRaw)
	tiers, err := getPricing(m)
	if err != nil {
		return PriceResult{Model: m, PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, Note: "unknown model pricing"}, nil
	}

	// Get cached token discount from config
	cfg, configErr := GetConfig()
	cachedDiscount := 0.1 // default 90% discount
	if configErr == nil && cfg.CachedTokenDiscount > 0 {
		cachedDiscount = cfg.CachedTokenDiscount
	}

	// Apply cached prompt token discount (cached tokens cost a percentage of normal prompt tokens)
	billedPromptTokens := float64(u.PromptTokens)
	if u.PromptCachedTokens > 0 {
		cached := u.PromptCachedTokens
		if cached > u.PromptTokens {
			cached = u.PromptTokens
		}
		// Effective billed prompt tokens: non-cached + discount% of cached
		billedPromptTokens = float64(u.PromptTokens-cached) + cachedDiscount*float64(cached)
	}
	ptCost := (billedPromptTokens / 1000.0) * tiers.Prompt
	ctCost := (float64(u.CompletionTokens) / 1000.0) * tiers.Completion
	total := ptCost + ctCost
	note := "prices loaded from config; verify against https://openai.com/api/pricing/"
	if u.PromptCachedTokens > 0 {
		discountPercent := int((1.0 - cachedDiscount) * 100)
		note += fmt.Sprintf(" (includes %d%% discount for cached prompt tokens)", discountPercent)
	}
	return PriceResult{
		Model:             m,
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		PromptCostUSD:     ptCost,
		CompletionCostUSD: ctCost,
		TotalCostUSD:      total,
		Note:              note,
	}, nil
}

func (pr PriceResult) String() string {
	return fmt.Sprintf("[pricing] model=%s prompt=%d completion=%d cost_prompt=$%.6f cost_completion=$%.6f total=$%.6f", pr.Model, pr.PromptTokens, pr.CompletionTokens, pr.PromptCostUSD, pr.CompletionCostUSD, pr.TotalCostUSD)
}
