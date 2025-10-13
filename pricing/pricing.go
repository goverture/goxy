package pricing

import "fmt"

// Model represents a model name for pricing.
type Model string

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

// resolveModelName determines the canonical model name to use for pricing lookup.
// This first checks if the model exists directly in config, then tries to find the longest matching prefix.
// If found via prefix match, returns the canonical name; otherwise returns the original name.
func resolveModelName(raw string) string {
	cfg, err := GetConfig()
	if err != nil {
		return raw // fallback to original name if config can't be loaded
	}

	// Try direct lookup first
	if _, exists := cfg.Models[raw]; exists {
		return raw
	}

	// Check for prefix matches - find the longest matching prefix
	var bestMatch string

	for configModel := range cfg.Models {
		// Check if the input model name starts with this config model name
		if len(configModel) > len(bestMatch) && len(raw) >= len(configModel) &&
			raw[:len(configModel)] == configModel {
			bestMatch = configModel
		}
	}

	if bestMatch != "" {
		return bestMatch
	}

	// Not found in config or via prefix, return as-is
	return raw
}

// ComputePrice calculates cost given usage and model.
func ComputePrice(modelRaw string, u Usage) (PriceResult, error) {
	modelName := resolveModelName(modelRaw)
	m := Model(modelName)
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
