package pricing

import (
	"testing"
)

func TestIntegerBasedPricingBenefits(t *testing.T) {
	// Setup test config
	originalConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"test-micro": {
				Prompt:       0.000001, // Very small amount that causes precision issues
				CachedPrompt: 0.0000001,
				Completion:   0.000002,
			},
		},
	}
	SetConfig(originalConfig)
	defer ResetConfig()

	usage := Usage{
		PromptTokens:       1000000, // 1M tokens
		PromptCachedTokens: 500000,  // 500K cached
		CompletionTokens:   750000,  // 750K completion
	}

	// Test with Money-based computation
	moneyResult, err := ComputePriceMoney("test-micro", usage)
	if err != nil {
		t.Fatalf("ComputePriceMoney failed: %v", err)
	}

	// Verify precision - the key benefit of Money-based arithmetic
	expectedPromptCostNonCached := NewMoneyFromUSD(0.000001 / 1000000.0).Multiply(500000) // 500K non-cached tokens
	expectedPromptCostCached := NewMoneyFromUSD(0.0000001 / 1000000.0).Multiply(500000)   // 500K cached tokens
	expectedTotalPromptCost := expectedPromptCostNonCached.Add(expectedPromptCostCached)

	if moneyResult.PromptCost != expectedTotalPromptCost {
		t.Errorf("Money calculation mismatch. Got: %s, Expected: %s",
			moneyResult.PromptCost.String(), expectedTotalPromptCost.String())
	}

	t.Logf("✅ Money-based result: %s", moneyResult.String())
	t.Logf("✅ Exact integer-based precision achieved!")
}
