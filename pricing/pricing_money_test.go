package pricing

import (
	"testing"
)

func setupTestConfigMoney() *PricingConfigMoney {
	return &PricingConfigMoney{
		Models: map[string]ModelPricingMoney{
			"gpt-4": {
				Prompt:       NewMoneyFromUSD(30.0),
				CachedPrompt: NewMoneyFromUSD(3.0),
				Completion:   NewMoneyFromUSD(60.0),
			},
			"gpt-4o": {
				Prompt:       NewMoneyFromUSD(5.0),
				CachedPrompt: NewMoneyFromUSD(0.5),
				Completion:   NewMoneyFromUSD(15.0),
			},
		},
		Default: &ModelPricingMoney{
			Prompt:       NewMoneyFromUSD(10.0),
			CachedPrompt: NewMoneyFromUSD(1.0),
			Completion:   NewMoneyFromUSD(20.0),
		},
	}
}

func TestComputePriceMoney_Basic(t *testing.T) {
	// Setup test config
	originalConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4o": {
				Prompt:       5.0,
				CachedPrompt: 0.5,
				Completion:   15.0,
			},
		},
	}
	SetConfig(originalConfig)
	defer ResetConfig()

	usage := Usage{
		PromptTokens:     1000,
		CompletionTokens: 500,
	}

	result, err := ComputePriceMoney("gpt-4o", usage)
	if err != nil {
		t.Fatalf("ComputePriceMoney failed: %v", err)
	}

	// Expected calculations:
	// Prompt: 1000 tokens * $5.0 per 1M tokens = $0.005
	// Completion: 500 tokens * $15.0 per 1M tokens = $0.0075
	// Total: $0.0125

	expectedPrompt := NewMoneyFromUSD(0.005)
	expectedCompletion := NewMoneyFromUSD(0.0075)
	expectedTotal := NewMoneyFromUSD(0.0125)

	if result.PromptCost != expectedPrompt {
		t.Errorf("PromptCost: got %s, want %s", result.PromptCost.String(), expectedPrompt.String())
	}

	if result.CompletionCost != expectedCompletion {
		t.Errorf("CompletionCost: got %s, want %s", result.CompletionCost.String(), expectedCompletion.String())
	}

	if result.TotalCost != expectedTotal {
		t.Errorf("TotalCost: got %s, want %s", result.TotalCost.String(), expectedTotal.String())
	}

	// Test conversion to legacy format
	legacy := result.ToLegacy()
	if !almostEqual(legacy.PromptCostUSD, 0.005) {
		t.Errorf("Legacy PromptCostUSD: got %f, want %f", legacy.PromptCostUSD, 0.005)
	}

	if !almostEqual(legacy.TotalCostUSD, 0.0125) {
		t.Errorf("Legacy TotalCostUSD: got %f, want %f", legacy.TotalCostUSD, 0.0125)
	}
}

func TestComputePriceMoney_WithCachedTokens(t *testing.T) {
	// Setup test config
	originalConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4o": {
				Prompt:       5.0,
				CachedPrompt: 0.5,
				Completion:   15.0,
			},
		},
	}
	SetConfig(originalConfig)
	defer ResetConfig()

	usage := Usage{
		PromptTokens:       1000,
		PromptCachedTokens: 200, // 200 cached out of 1000 total
		CompletionTokens:   500,
	}

	result, err := ComputePriceMoney("gpt-4o", usage)
	if err != nil {
		t.Fatalf("ComputePriceMoney failed: %v", err)
	}

	// Expected calculations:
	// Non-cached prompt: 800 tokens * $5.0 per 1M tokens = $0.004
	// Cached prompt: 200 tokens * $0.5 per 1M tokens = $0.0001
	// Total prompt: $0.0041
	// Completion: 500 tokens * $15.0 per 1M tokens = $0.0075
	// Total: $0.0116

	expectedNonCachedPrompt := NewMoneyFromUSD(0.004)
	expectedCachedPrompt := NewMoneyFromUSD(0.0001)
	expectedTotalPrompt := expectedNonCachedPrompt.Add(expectedCachedPrompt)
	expectedCompletion := NewMoneyFromUSD(0.0075)
	expectedTotal := expectedTotalPrompt.Add(expectedCompletion)

	if result.PromptCost != expectedTotalPrompt {
		t.Errorf("PromptCost: got %s, want %s", result.PromptCost.String(), expectedTotalPrompt.String())
	}

	if result.CompletionCost != expectedCompletion {
		t.Errorf("CompletionCost: got %s, want %s", result.CompletionCost.String(), expectedCompletion.String())
	}

	if result.TotalCost != expectedTotal {
		t.Errorf("TotalCost: got %s, want %s", result.TotalCost.String(), expectedTotal.String())
	}
}

func TestMoneyVsFloat_PrecisionComparison(t *testing.T) {
	// Setup test config
	originalConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"test-model": {
				Prompt:       0.123456789, // A number that might cause precision issues
				CachedPrompt: 0.012345678,
				Completion:   0.987654321,
			},
		},
	}
	SetConfig(originalConfig)
	defer ResetConfig()

	usage := Usage{
		PromptTokens:       123456,
		PromptCachedTokens: 12345,
		CompletionTokens:   98765,
	}

	// Compute with both methods
	legacyResult, err1 := ComputePrice("test-model", usage)
	if err1 != nil {
		t.Fatalf("ComputePrice failed: %v", err1)
	}

	moneyResult, err2 := ComputePriceMoney("test-model", usage)
	if err2 != nil {
		t.Fatalf("ComputePriceMoney failed: %v", err2)
	}

	// Convert Money result back to legacy for comparison
	moneyAsLegacy := moneyResult.ToLegacy()

	// The results should be very close (within reasonable floating point precision)
	promptDiff := legacyResult.PromptCostUSD - moneyAsLegacy.PromptCostUSD
	completionDiff := legacyResult.CompletionCostUSD - moneyAsLegacy.CompletionCostUSD
	totalDiff := legacyResult.TotalCostUSD - moneyAsLegacy.TotalCostUSD

	t.Logf("Legacy prompt cost: $%.12f", legacyResult.PromptCostUSD)
	t.Logf("Money prompt cost:  $%.12f", moneyAsLegacy.PromptCostUSD)
	t.Logf("Prompt difference:  $%.15f", promptDiff)
	t.Logf("Total difference:   $%.15f", totalDiff)

	// Verify they're close enough (within 1e-8 for monetary calculations, allowing for rounding differences)
	if abs(promptDiff) > 1e-8 {
		t.Errorf("Prompt cost difference too large: %e", promptDiff)
	}
	if abs(completionDiff) > 1e-8 {
		t.Errorf("Completion cost difference too large: %e", completionDiff)
	}
	if abs(totalDiff) > 1e-8 {
		t.Errorf("Total cost difference too large: %e", totalDiff)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
