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
	ServiceTier       string
	PromptTokens      int
	CompletionTokens  int
	PromptCostUSD     float64
	CompletionCostUSD float64
	TotalCostUSD      float64
	Note              string
}

// getPricing returns the pricing for a given model and service tier from configuration
func getPricing(model Model, serviceTier string) (struct{ Prompt, Completion float64 }, string, error) {
	cfg, err := GetConfig()
	if err != nil {
		return struct{ Prompt, Completion float64 }{}, "standard", fmt.Errorf("failed to load pricing config: %w", err)
	}

	pricing, found := cfg.FindModelPricing(string(model))
	if found {
		prompt, completion, actualTier := pricing.GetTierPricing(serviceTier)
		return struct{ Prompt, Completion float64 }{
			Prompt:     prompt,
			Completion: completion,
		}, actualTier, nil
	}

	// Fallback to default if configured
	if cfg.Default != nil {
		return struct{ Prompt, Completion float64 }{
			Prompt:     cfg.Default.Prompt,
			Completion: cfg.Default.Completion,
		}, "standard", nil
	}

	return struct{ Prompt, Completion float64 }{}, "standard", fmt.Errorf("no pricing found for model %s", model)
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
		fmt.Printf("[pricing] Model mapping: %q -> %q (exact match)\n", raw, raw)
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
		fmt.Printf("[pricing] Model mapping: %q -> %q (prefix match)\n", raw, bestMatch)
		return bestMatch
	}

	// Not found in config or via prefix, return as-is
	return raw
}

// ComputePrice calculates cost given usage and model (using standard pricing).
func ComputePrice(modelRaw string, u Usage) (PriceResult, error) {
	return ComputePriceWithTier(modelRaw, u, "standard")
}

// ComputePriceWithTier calculates cost given usage, model, and service tier.
func ComputePriceWithTier(modelRaw string, u Usage, serviceTier string) (PriceResult, error) {
	modelName := resolveModelName(modelRaw)
	m := Model(modelName)
	tiers, actualTier, err := getPricing(m, serviceTier)
	if err != nil {
		return PriceResult{
			Model:            m,
			ServiceTier:      serviceTier,
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			Note:             "unknown model pricing",
		}, nil
	}

	// Log tier fallback if different from requested
	if serviceTier != "standard" && actualTier != serviceTier {
		fmt.Printf("[pricing] Service tier fallback: %q -> %q for model %q\n", serviceTier, actualTier, modelName)
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
	ptCost := (billedPromptTokens / 1000000.0) * tiers.Prompt
	ctCost := (float64(u.CompletionTokens) / 1000000.0) * tiers.Completion
	total := ptCost + ctCost
	note := "prices loaded from config; verify against https://openai.com/api/pricing/"
	if actualTier != "standard" {
		note += fmt.Sprintf(" (using %s tier pricing)", actualTier)
	}
	if u.PromptCachedTokens > 0 {
		discountPercent := int((1.0 - cachedDiscount) * 100)
		note += fmt.Sprintf(" (includes %d%% discount for cached prompt tokens)", discountPercent)
	}
	return PriceResult{
		Model:             m,
		ServiceTier:       actualTier,
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		PromptCostUSD:     ptCost,
		CompletionCostUSD: ctCost,
		TotalCostUSD:      total,
		Note:              note,
	}, nil
}

func (pr PriceResult) String() string {
	return fmt.Sprintf("[pricing] model=%s tier=%s prompt=%d completion=%d cost_prompt=$%.6f cost_completion=$%.6f total=$%.6f", pr.Model, pr.ServiceTier, pr.PromptTokens, pr.CompletionTokens, pr.PromptCostUSD, pr.CompletionCostUSD, pr.TotalCostUSD)
}
