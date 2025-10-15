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

// PriceResultMoney holds the computed pricing info using Money type for precision.
type PriceResultMoney struct {
	Model            Model
	ServiceTier      string
	PromptTokens     int
	CompletionTokens int
	PromptCost       Money
	CompletionCost   Money
	TotalCost        Money
	Note             string
}

// getPricing returns the pricing for a given model and service tier from configuration
func getPricing(model Model, serviceTier string) (struct{ Prompt, CachedPrompt, Completion float64 }, string, error) {
	cfg, err := GetConfig()
	if err != nil {
		return struct{ Prompt, CachedPrompt, Completion float64 }{}, "standard", fmt.Errorf("failed to load pricing config: %w", err)
	}

	pricing, found := cfg.FindModelPricing(string(model))
	if found {
		prompt, cachedPrompt, completion, actualTier := pricing.GetTierPricing(serviceTier)
		return struct{ Prompt, CachedPrompt, Completion float64 }{
			Prompt:       prompt,
			CachedPrompt: cachedPrompt,
			Completion:   completion,
		}, actualTier, nil
	}

	// Fallback to default if configured
	if cfg.Default != nil {
		return struct{ Prompt, CachedPrompt, Completion float64 }{
			Prompt:       cfg.Default.Prompt,
			CachedPrompt: cfg.Default.CachedPrompt,
			Completion:   cfg.Default.Completion,
		}, "standard", nil
	}

	return struct{ Prompt, CachedPrompt, Completion float64 }{}, "standard", fmt.Errorf("no pricing found for model %s", model)
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

	// Calculate prompt cost: split between cached and non-cached tokens
	var ptCost float64
	if u.PromptCachedTokens > 0 {
		cached := u.PromptCachedTokens
		if cached > u.PromptTokens {
			cached = u.PromptTokens
		}
		nonCachedTokens := u.PromptTokens - cached

		// Use actual cached prompt pricing from config
		nonCachedCost := (float64(nonCachedTokens) / 1000000.0) * tiers.Prompt
		cachedCost := (float64(cached) / 1000000.0) * tiers.CachedPrompt
		ptCost = nonCachedCost + cachedCost
	} else {
		// All prompt tokens are regular (non-cached)
		ptCost = (float64(u.PromptTokens) / 1000000.0) * tiers.Prompt
	}

	ctCost := (float64(u.CompletionTokens) / 1000000.0) * tiers.Completion
	total := ptCost + ctCost

	note := "prices loaded from config; verify against https://openai.com/api/pricing/"
	if actualTier != "standard" {
		note += fmt.Sprintf(" (using %s tier pricing)", actualTier)
	}
	if u.PromptCachedTokens > 0 {
		note += " (includes cached prompt token pricing)"
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

// getPricingMoney returns the Money-based pricing for a given model and service tier from configuration
func getPricingMoney(model Model, serviceTier string) (prompt, cachedPrompt, completion Money, actualTier string, err error) {
	cfg, configErr := GetConfig()
	if configErr != nil {
		return Money(0), Money(0), Money(0), "standard", fmt.Errorf("failed to load pricing config: %w", configErr)
	}

	cfgMoney := cfg.ToMoney()
	pricing, found := cfgMoney.FindModelPricingMoney(string(model))
	if found {
		prompt, cachedPrompt, completion, actualTier := pricing.GetTierPricingMoney(serviceTier)
		return prompt, cachedPrompt, completion, actualTier, nil
	}

	// Fallback to default if configured
	if cfgMoney.Default != nil {
		prompt, cachedPrompt, completion, _ := cfgMoney.Default.GetTierPricingMoney(serviceTier)
		return prompt, cachedPrompt, completion, "standard", nil
	}

	return Money(0), Money(0), Money(0), "standard", fmt.Errorf("no pricing found for model %s", model)
}

// ComputePriceMoney calculates cost given usage and model (using standard pricing) with Money precision.
func ComputePriceMoney(modelRaw string, u Usage) (PriceResultMoney, error) {
	return ComputePriceMoneyWithTier(modelRaw, u, "standard")
}

// ComputePriceMoneyWithTier calculates cost given usage, model, and service tier using Money precision.
func ComputePriceMoneyWithTier(modelRaw string, u Usage, serviceTier string) (PriceResultMoney, error) {
	modelName := resolveModelName(modelRaw)
	m := Model(modelName)
	promptPrice, cachedPromptPrice, completionPrice, actualTier, err := getPricingMoney(m, serviceTier)
	if err != nil {
		return PriceResultMoney{
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

	// Calculate prompt cost: split between cached and non-cached tokens
	var ptCost Money
	if u.PromptCachedTokens > 0 {
		cached := u.PromptCachedTokens
		if cached > u.PromptTokens {
			cached = u.PromptTokens
		}
		nonCachedTokens := u.PromptTokens - cached

		// Use precise Money arithmetic
		nonCachedCost := promptPrice.Multiply(float64(nonCachedTokens) / 1000000.0)
		cachedCost := cachedPromptPrice.Multiply(float64(cached) / 1000000.0)
		ptCost = nonCachedCost.Add(cachedCost)
	} else {
		// All prompt tokens are regular (non-cached)
		ptCost = promptPrice.Multiply(float64(u.PromptTokens) / 1000000.0)
	}

	ctCost := completionPrice.Multiply(float64(u.CompletionTokens) / 1000000.0)
	total := ptCost.Add(ctCost)

	note := "prices loaded from config; verify against https://openai.com/api/pricing/"
	if actualTier != "standard" {
		note += fmt.Sprintf(" (using %s tier pricing)", actualTier)
	}
	if u.PromptCachedTokens > 0 {
		note += " (includes cached prompt token pricing)"
	}

	return PriceResultMoney{
		Model:            m,
		ServiceTier:      actualTier,
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		PromptCost:       ptCost,
		CompletionCost:   ctCost,
		TotalCost:        total,
		Note:             note,
	}, nil
}

// ToLegacy converts PriceResultMoney to PriceResult for backward compatibility
func (pr PriceResultMoney) ToLegacy() PriceResult {
	return PriceResult{
		Model:             pr.Model,
		ServiceTier:       pr.ServiceTier,
		PromptTokens:      pr.PromptTokens,
		CompletionTokens:  pr.CompletionTokens,
		PromptCostUSD:     pr.PromptCost.ToUSD(),
		CompletionCostUSD: pr.CompletionCost.ToUSD(),
		TotalCostUSD:      pr.TotalCost.ToUSD(),
		Note:              pr.Note,
	}
}

func (pr PriceResultMoney) String() string {
	return fmt.Sprintf("[pricing] model=%s tier=%s prompt=%d completion=%d cost_prompt=%s cost_completion=%s total=%s",
		pr.Model, pr.ServiceTier, pr.PromptTokens, pr.CompletionTokens,
		pr.PromptCost.String(), pr.CompletionCost.String(), pr.TotalCost.String())
}
