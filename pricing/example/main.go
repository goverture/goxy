package main

import (
	"fmt"
	"log"

	"github.com/goverture/goxy/pricing"
)

func main() {
	// Example 1: Using default configuration (loads from pricing/pricing.yaml)
	fmt.Println("=== Example 1: Default Configuration ===")
	usage := pricing.Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}

	result, err := pricing.ComputePrice("gpt-4o", usage)
	if err != nil {
		log.Printf("Error computing price: %v", err)
	} else {
		fmt.Println(result.String())
	}

	// Example 2: Service Tier Pricing
	fmt.Println("\n=== Example 2: Service Tier Pricing ===")
	models := []string{"gpt-5", "gpt-4o", "o3"}
	tiers := []string{"standard", "flex", "priority"}

	for _, model := range models {
		fmt.Printf("\nModel: %s\n", model)
		fmt.Println("---------------")

		for _, tier := range tiers {
			result, err := pricing.ComputePriceWithTier(model, usage, tier)
			if err != nil {
				log.Printf("Error computing price for %s with tier %s: %v", model, tier, err)
				continue
			}

			tierInfo := result.ServiceTier
			if tierInfo != tier {
				tierInfo = fmt.Sprintf("%s (fallback from %s)", result.ServiceTier, tier)
			}

			fmt.Printf("  %s: $%.6f\n", tierInfo, result.TotalCostUSD)
		}
	}

	// Example 3: Cached Token Pricing with Service Tiers
	fmt.Println("\n=== Example 3: Cached Token Pricing ===")
	usageWithCached := pricing.Usage{
		PromptTokens:       1000,
		PromptCachedTokens: 200, // 200 tokens from cache
		CompletionTokens:   500,
	}

	for _, tier := range []string{"standard", "flex", "priority"} {
		result, err := pricing.ComputePriceWithTier("gpt-5", usageWithCached, tier)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("gpt-5 %s with cached tokens: $%.6f\n", result.ServiceTier, result.TotalCostUSD)
	}

	// Example 4: Using a custom configuration
	fmt.Println("\n=== Example 4: Custom Configuration ===")
	customConfig := &pricing.PricingConfig{
		Models: map[string]pricing.ModelPricing{
			"custom-model": {
				Prompt:     0.001,
				Completion: 0.002,
			},
		},
		Default: &pricing.ModelPricing{
			Prompt:     0.005,
			Completion: 0.01,
		},
		CachedTokenDiscount: 0.05, // 95% discount for cached tokens
	}

	// Set the custom configuration
	pricing.SetConfig(customConfig)

	// Test with the custom model
	result2, err := pricing.ComputePrice("custom-model", usage)
	if err != nil {
		log.Printf("Error computing price: %v", err)
	} else {
		fmt.Println(result2.String())
	}

	// Test with cached tokens
	fmt.Println("\n=== Example 5: With Cached Tokens ===")
	usageWithCache := pricing.Usage{
		PromptTokens:       1000,
		PromptCachedTokens: 600, // 60% of tokens are cached
		CompletionTokens:   500,
	}

	result3, err := pricing.ComputePrice("custom-model", usageWithCache)
	if err != nil {
		log.Printf("Error computing price: %v", err)
	} else {
		fmt.Println(result3.String())
		fmt.Printf("Note: %s\n", result3.Note)
	}

	// Reset configuration back to default
	pricing.ResetConfig()
}
