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

	// Example 2: Using a custom configuration
	fmt.Println("\n=== Example 2: Custom Configuration ===")
	customConfig := &pricing.PricingConfig{
		Models: map[string]pricing.ModelPricing{
			"custom-model": {
				Prompt:     0.001,
				Completion: 0.002,
			},
			"gpt-4": {
				Prompt:     0.035, // Updated pricing
				Completion: 0.065,
				Aliases:    []string{"gpt-4-0613", "gpt-4-latest"},
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
	fmt.Println("\n=== Example 3: With Cached Tokens ===")
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

	// Example 4: Loading from a custom YAML file
	fmt.Println("\n=== Example 4: Loading Custom YAML ===")
	// Reset to load from file
	pricing.ResetConfig()

	// This would load from pricing/pricing.yaml by default
	// You can also use: pricing.LoadConfig("/path/to/custom/pricing.yaml")

	result4, err := pricing.ComputePrice("gpt-5-mini", usage)
	if err != nil {
		log.Printf("Error computing price: %v", err)
	} else {
		fmt.Println(result4.String())
	}
}
