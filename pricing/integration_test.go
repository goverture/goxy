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

	// Test with Money-based computation (new way)
	moneyResult, err := ComputePriceMoney("test-micro", usage)
	if err != nil {
		t.Fatalf("ComputePriceMoney failed: %v", err)
	}

	// Test with legacy computation (now delegates to Money internally)
	legacyResult, err := ComputePrice("test-micro", usage)
	if err != nil {
		t.Fatalf("ComputePrice failed: %v", err)
	}

	// Now they should be identical because ComputePrice uses Money internally
	if legacyResult.PromptCostUSD != moneyResult.PromptCost.ToUSD() {
		t.Errorf("Results should be identical now. Legacy: %f, Money: %f",
			legacyResult.PromptCostUSD, moneyResult.PromptCost.ToUSD())
	}

	// Demonstrate that we get exact precision with Money
	expectedPromptCostNonCached := NewMoneyFromUSD(0.000001).Multiply(500000.0 / 1000000.0) // 500K non-cached tokens
	expectedPromptCostCached := NewMoneyFromUSD(0.0000001).Multiply(500000.0 / 1000000.0)   // 500K cached tokens
	expectedTotalPromptCost := expectedPromptCostNonCached.Add(expectedPromptCostCached)

	if moneyResult.PromptCost != expectedTotalPromptCost {
		t.Errorf("Money calculation mismatch. Got: %s, Expected: %s",
			moneyResult.PromptCost.String(), expectedTotalPromptCost.String())
	}

	t.Logf("✅ Money-based result: %s", moneyResult.String())
	t.Logf("✅ Legacy result (now uses Money): Total=$%.8f", legacyResult.TotalCostUSD)
	t.Logf("✅ Results are identical: %v", legacyResult.TotalCostUSD == moneyResult.TotalCost.ToUSD())
}
