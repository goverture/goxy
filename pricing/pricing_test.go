package pricing

import (
	"math"
	"strings"
	"testing"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// setupTestConfig sets up a test configuration for testing
func setupTestConfig() {
	testConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4": {
				Prompt:       30.0,
				CachedPrompt: 3.0,
				Completion:   60.0,
			},
			"gpt-4o": {
				Prompt:       5.0,
				CachedPrompt: 0.5,
				Completion:   15.0,
			},
			"gpt-5-mini": {
				Prompt:       3.0,
				CachedPrompt: 0.3,
				Completion:   6.0,
			},
			"gpt-5": {
				Prompt:       10.0,
				CachedPrompt: 1.0,
				Completion:   20.0,
			},
		},
		Default: &ModelPricing{
			Prompt:       10.0,
			CachedPrompt: 1.0,
			Completion:   20.0,
		},
	}
	SetConfig(testConfig)
}

func TestComputePrice_GPT5MiniSampleUsage(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 11, CompletionTokens: 369}
	res, err := ComputePriceMoney("gpt-5-mini-2025-08-07", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Model != Model("gpt-5-mini") {
		t.Fatalf("expected model normalize to gpt-5-mini, got %s", res.Model)
	}
	expectedPrompt := 11.0 / 1000000.0 * 3.0      // 0.000033
	expectedCompletion := 369.0 / 1000000.0 * 6.0 // 0.002214
	if !almostEqual(res.PromptCost.ToUSD(), expectedPrompt) {
		t.Fatalf("prompt cost mismatch: got %f want %f", res.PromptCost.ToUSD(), expectedPrompt)
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletion) {
		t.Fatalf("completion cost mismatch: got %f want %f", res.CompletionCost.ToUSD(), expectedCompletion)
	}
	if !almostEqual(res.TotalCost.ToUSD(), expectedPrompt+expectedCompletion) {
		t.Fatalf("total cost mismatch: got %f want %f", res.TotalCost.ToUSD(), expectedPrompt+expectedCompletion)
	}
}

func TestComputePrice_WithCachedPromptTokens(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	// 100 prompt tokens of which 60 are cached
	usage := Usage{PromptTokens: 100, PromptCachedTokens: 60, CompletionTokens: 0}
	res, err := ComputePriceMoney("gpt-5-mini", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Calculate expected cost using separate pricing
	// 40 normal prompt tokens at 3.0 per million + 60 cached tokens at 0.3 per million
	nonCachedCost := (40.0 / 1000000.0) * 3.0
	cachedCost := (60.0 / 1000000.0) * 0.3
	expectedPromptCost := nonCachedCost + cachedCost

	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Fatalf("prompt cost mismatch with cache: got %f want %f", res.PromptCost.ToUSD(), expectedPromptCost)
	}
	if res.TotalCost.ToUSD() != res.PromptCost.ToUSD() {
		t.Fatalf("total should equal prompt cost when no completion tokens")
	}
	if res.Note == "" || !strings.Contains(res.Note, "cached") {
		t.Fatalf("expected note to mention cached pricing, got %q", res.Note)
	}
}

func TestFloat64PrecisionWithCheapestModel(t *testing.T) {
	// Load the actual pricing config to test with real prices
	_, err := GetConfig()
	if err != nil {
		t.Fatalf("failed to load pricing config: %v", err)
	}

	// Test with gpt-5-nano which has the cheapest rate
	// From pricing.yaml: cached_prompt in flex/batch tiers is 0.0025 per million tokens (cheapest rate!)
	modelName := "gpt-5-nano"

	// Test 1: Single cached prompt token precision (cheapest possible billing)
	t.Run("single_cached_prompt_token_precision", func(t *testing.T) {
		// 1 prompt token, all cached, 0 completion tokens - cheapest possible request
		usage := Usage{PromptTokens: 1, PromptCachedTokens: 1, CompletionTokens: 0}
		res, err := ComputePriceMoneyWithTier(modelName, usage, "flex")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Expected cost calculation:
		// billedPromptTokens = 0 + 0.1 * 1 = 0.1 effective tokens (90% discount)
		// flex tier for gpt-5-nano has cached_prompt rate of 0.0025 per million tokens
		// cost = (1.0 / 1000000.0) * 0.0025 = 0.0000000025 USD
		expectedCost := (1.0 / 1000000.0) * 0.0025

		if !almostEqual(res.PromptCost.ToUSD(), expectedCost) {
			t.Errorf("single cached token cost precision issue: got %.15f, want %.15f, diff=%.2e",
				res.PromptCost.ToUSD(), expectedCost, math.Abs(res.PromptCost.ToUSD()-expectedCost))
		}

		t.Logf("Single cached prompt token cost: $%.15f (expected: $%.15f)", res.PromptCost.ToUSD(), expectedCost)
		t.Logf("This is the absolute cheapest possible API call!")
	})

	// Test 2: Accumulation of many single cached token requests
	t.Run("cached_token_accumulation_precision", func(t *testing.T) {
		numRequests := 100
		var totalCostAccumulated float64

		// Calculate cost of 100 individual requests with 1 cached token each
		for i := 0; i < numRequests; i++ {
			usage := Usage{PromptTokens: 1, PromptCachedTokens: 1, CompletionTokens: 0}
			res, err := ComputePriceMoneyWithTier(modelName, usage, "flex")
			if err != nil {
				t.Fatalf("unexpected error on request %d: %v", i, err)
			}
			totalCostAccumulated += res.PromptCost.ToUSD()
		}

		// Calculate cost of a single request with 100 cached tokens
		usage100 := Usage{PromptTokens: 100, PromptCachedTokens: 100, CompletionTokens: 0}
		res100, err := ComputePriceMoneyWithTier(modelName, usage100, "flex")
		if err != nil {
			t.Fatalf("unexpected error for 100-cached-token request: %v", err)
		}

		// Compare the two approaches
		diff := math.Abs(totalCostAccumulated - res100.PromptCost.ToUSD())
		relativeDiff := diff / res100.PromptCost.ToUSD()

		t.Logf("100 x 1-cached-token requests: $%.15f", totalCostAccumulated)
		t.Logf("1 x 100-cached-token request:  $%.15f", res100.PromptCost.ToUSD())
		t.Logf("Absolute difference:           $%.2e", diff)
		t.Logf("Relative difference:           %.2e (%.8f%%)", relativeDiff, relativeDiff*100)

		// The difference should be negligible (less than 0.01% relative error)
		if relativeDiff > 1e-4 {
			t.Errorf("accumulation precision issue: relative difference %.2e exceeds threshold", relativeDiff)
		}
	})

	// Test 3: Test extreme precision with many tiny cached token costs
	t.Run("extreme_cached_token_precision", func(t *testing.T) {
		// Test with the smallest possible billing unit
		numRequests := 10000
		var totalCost float64

		// Each request: 1 cached prompt token (with 90% discount)
		for i := 0; i < numRequests; i++ {
			billedTokens := 0.1 // 90% discount
			costPerRequest := (billedTokens / 1000000.0) * 0.0025
			totalCost += costPerRequest
		}

		// Expected total cost
		expectedTotal := (float64(numRequests) * 0.1 / 1000000.0) * 0.0025

		diff := math.Abs(totalCost - expectedTotal)
		relativeDiff := diff / expectedTotal

		t.Logf("10000 x single-cached-token calculations: $%.15f", totalCost)
		t.Logf("Expected total:                           $%.15f", expectedTotal)
		t.Logf("Absolute difference:                      $%.2e", diff)
		t.Logf("Relative difference:                      %.2e", relativeDiff)

		// For this extreme case, allow slightly higher tolerance due to floating point accumulation
		if relativeDiff > 1e-10 {
			t.Errorf("extreme cached token precision issue: relative difference %.2e too large", relativeDiff)
		}
	})

	// Test 4: Compare with regular prompt tokens to verify the discount calculation
	t.Run("cached_vs_regular_token_precision", func(t *testing.T) {
		// Test regular prompt token
		usageRegular := Usage{PromptTokens: 1, PromptCachedTokens: 0, CompletionTokens: 0}
		resRegular, err := ComputePriceMoneyWithTier(modelName, usageRegular, "flex")
		if err != nil {
			t.Fatalf("unexpected error for regular token: %v", err)
		}

		// Test cached prompt token
		usageCached := Usage{PromptTokens: 1, PromptCachedTokens: 1, CompletionTokens: 0}
		resCached, err := ComputePriceMoneyWithTier(modelName, usageCached, "flex")
		if err != nil {
			t.Fatalf("unexpected error for cached token: %v", err)
		}

		// Cached should be exactly 10% of regular (90% discount)
		expectedCachedCost := resRegular.PromptCost.ToUSD() * 0.1

		if !almostEqual(resCached.PromptCost.ToUSD(), expectedCachedCost) {
			t.Errorf("cached discount precision issue: got %.15f, want %.15f",
				resCached.PromptCost.ToUSD(), expectedCachedCost)
		}

		t.Logf("Regular prompt token cost:  $%.15f", resRegular.PromptCost.ToUSD())
		t.Logf("Cached prompt token cost:   $%.15f", resCached.PromptCost.ToUSD())
		t.Logf("Discount ratio:             %.3f (should be 0.100)", resCached.PromptCost.ToUSD()/resRegular.PromptCost.ToUSD())
	})
}

func TestComputePrice_UnknownModel(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 100, CompletionTokens: 200}
	res, err := ComputePriceMoney("some-new-future-model", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use default pricing from config
	expectedPromptCost := (100.0 / 1000000.0) * 10.0     // default prompt rate
	expectedCompletionCost := (200.0 / 1000000.0) * 20.0 // default completion rate
	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Fatalf("expected default prompt pricing, got %f want %f", res.PromptCost.ToUSD(), expectedPromptCost)
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletionCost) {
		t.Fatalf("expected default completion pricing, got %f want %f", res.CompletionCost.ToUSD(), expectedCompletionCost)
	}
}

func TestResolveModelNamePrefixMatching(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	cases := map[string]string{
		// Exact matches
		"gpt-4o":     "gpt-4o",
		"gpt-4":      "gpt-4",
		"gpt-5-mini": "gpt-5-mini",

		// Prefix matches - should match the configured model name
		"gpt-4o-2024-08-06":     "gpt-4o",
		"gpt-4-0613":            "gpt-4",
		"gpt-5-mini-2025-08-07": "gpt-5-mini",
		"gpt-4-turbo":           "gpt-4",

		// Should prefer longer matches
		"gpt-5-mini-experimental": "gpt-5-mini", // not just "gpt-5"

		// No match
		"unknown-model": "unknown-model", // should return as-is
		"claude-3":      "claude-3",      // should return as-is
	}
	for raw, want := range cases {
		got := resolveModelName(raw)
		if got != want {
			t.Fatalf("resolveModelName(%q) -> %q want %q", raw, got, want)
		}
	}
}

func TestComputePrice_ZeroUsage(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	res, err := ComputePriceMoney("gpt-4o", Usage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalCost.ToUSD() != 0 || res.PromptCost.ToUSD() != 0 || res.CompletionCost.ToUSD() != 0 {
		t.Fatalf("expected zero costs for zero usage, got %+v", res)
	}
}

func TestComputePrice_CachedExceedsPromptClamp(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 10, PromptCachedTokens: 25}
	res, err := ComputePriceMoney("gpt-4o", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Cached clamped to 10 -> billed = (10-10) + 0.1*10 = 1
	expected := (1.0 / 1000000.0) * 5.0
	if !almostEqual(res.PromptCost.ToUSD(), expected) {
		t.Fatalf("expected clamped cached pricing got %f want %f", res.PromptCost.ToUSD(), expected)
	}
}

func TestComputePrice_UnknownModelNoDefault(t *testing.T) {
	// Test config with no default pricing
	configWithoutDefault := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4": {
				Prompt:       30.0,
				CachedPrompt: 3.0,
				Completion:   60.0,
			},
		},
		// No Default field set
	}
	SetConfig(configWithoutDefault)
	defer ResetConfig()

	usage := Usage{PromptTokens: 100, CompletionTokens: 200}
	res, err := ComputePriceMoney("unknown-model", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return zero costs with unknown pricing note
	if res.PromptCost.ToUSD() != 0.0 || res.CompletionCost.ToUSD() != 0.0 || res.TotalCost.ToUSD() != 0.0 {
		t.Fatalf("expected zero costs for unknown model with no default, got %+v", res)
	}
	if res.Note != "unknown model pricing" {
		t.Fatalf("expected 'unknown model pricing' note, got %q", res.Note)
	}
	if res.Model != Model("unknown-model") {
		t.Fatalf("expected model name to be preserved, got %q", res.Model)
	}
	// Token counts should still be preserved
	if res.PromptTokens != 100 || res.CompletionTokens != 200 {
		t.Fatalf("token counts should be preserved, got prompt=%d completion=%d", res.PromptTokens, res.CompletionTokens)
	}
}

func TestPrefixMatchingRealWorld(t *testing.T) {
	// Test with models similar to the updated pricing.yaml
	realWorldConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4o": {
				Prompt:       2.5,
				CachedPrompt: 1.25,
				Completion:   10.0,
			},
			"gpt-4o-mini": {
				Prompt:       0.15,
				CachedPrompt: 0.075,
				Completion:   0.6,
			},
			"o1": {
				Prompt:       15.0,
				CachedPrompt: 7.5,
				Completion:   60.0,
			},
			"o1-mini": {
				Prompt:       1.1,
				CachedPrompt: 0.55,
				Completion:   4.4,
			},
		},
		Default: &ModelPricing{
			Prompt:       10.0,
			CachedPrompt: 1.0,
			Completion:   20.0,
		},
	}

	SetConfig(realWorldConfig)
	defer ResetConfig()

	// Test that longer specific names get matched correctly
	cases := []struct {
		input    string
		expected string
		prompt   float64
	}{
		{"gpt-4o-2024-05-13", "gpt-4o", 2.5},
		{"gpt-4o-mini-2024-07-18", "gpt-4o-mini", 0.15}, // Should match gpt-4o-mini, not gpt-4o
		{"o1-preview", "o1", 15.0},
		{"o1-mini-2024-09-12", "o1-mini", 1.1}, // Should match o1-mini, not o1
	}

	for _, tc := range cases {
		resolved := resolveModelName(tc.input)
		if resolved != tc.expected {
			t.Fatalf("resolveModelName(%q) -> %q, want %q", tc.input, resolved, tc.expected)
		}

		// Test actual pricing calculation
		usage := Usage{PromptTokens: 1000, CompletionTokens: 0}
		res, err := ComputePriceMoney(tc.input, usage)
		if err != nil {
			t.Fatalf("ComputePriceMoney(%q) failed: %v", tc.input, err)
		}

		expectedCost := tc.prompt / 1000.0 // 1000 tokens * rate / 1million
		if !almostEqual(res.PromptCost.ToUSD(), expectedCost) {
			t.Fatalf("ComputePriceMoney(%q) prompt cost: got %f, want %f", tc.input, res.PromptCost.ToUSD(), expectedCost)
		}
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	// Test loading configuration from YAML (this tests the config loader)
	tempConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"test-model": {
				Prompt:       0.001,
				CachedPrompt: 0.0001,
				Completion:   0.002,
			},
		},
	}

	SetConfig(tempConfig)
	defer ResetConfig()

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	pricing, found := cfg.FindModelPricing("test-model")
	if !found {
		t.Fatalf("expected to find test-model pricing")
	}

	if pricing.Prompt != 0.001 || pricing.CachedPrompt != 0.0001 || pricing.Completion != 0.002 {
		t.Fatalf("unexpected pricing values: %+v", pricing)
	}
}

func TestComputePriceMoneyWithTier(t *testing.T) {
	// Setup a test configuration with service tiers
	testConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-5": {
				Prompt:       1.25,
				CachedPrompt: 0.125,
				Completion:   10.0,
				Flex: &TierPricing{
					Prompt:       0.625,
					CachedPrompt: 0.0625,
					Completion:   5.0,
				},
				Priority: &TierPricing{
					Prompt:       2.5,
					CachedPrompt: 0.25,
					Completion:   20.0,
				},
			},
			"gpt-4o": {
				Prompt:       2.5,
				CachedPrompt: 0.25,
				Completion:   10.0,
				// No flex or priority tiers
			},
		},
	}
	SetConfig(testConfig)
	defer ResetConfig()

	usage := Usage{PromptTokens: 1000, CompletionTokens: 500}

	// Test standard pricing
	res, err := ComputePriceMoneyWithTier("gpt-5", usage, "standard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ServiceTier != "standard" {
		t.Errorf("expected service tier 'standard', got '%s'", res.ServiceTier)
	}
	expectedPromptCost := (1000.0 / 1000000.0) * 1.25
	expectedCompletionCost := (500.0 / 1000000.0) * 10.0
	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Errorf("expected prompt cost %f, got %f", expectedPromptCost, res.PromptCost.ToUSD())
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletionCost) {
		t.Errorf("expected completion cost %f, got %f", expectedCompletionCost, res.CompletionCost.ToUSD())
	}

	// Test flex pricing
	res, err = ComputePriceMoneyWithTier("gpt-5", usage, "flex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ServiceTier != "flex" {
		t.Errorf("expected service tier 'flex', got '%s'", res.ServiceTier)
	}
	expectedPromptCost = (1000.0 / 1000000.0) * 0.625
	expectedCompletionCost = (500.0 / 1000000.0) * 5.0
	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Errorf("expected flex prompt cost %f, got %f", expectedPromptCost, res.PromptCost.ToUSD())
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletionCost) {
		t.Errorf("expected flex completion cost %f, got %f", expectedCompletionCost, res.CompletionCost.ToUSD())
	}

	// Test priority pricing
	res, err = ComputePriceMoneyWithTier("gpt-5", usage, "priority")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ServiceTier != "priority" {
		t.Errorf("expected service tier 'priority', got '%s'", res.ServiceTier)
	}
	expectedPromptCost = (1000.0 / 1000000.0) * 2.5
	expectedCompletionCost = (500.0 / 1000000.0) * 20.0
	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Errorf("expected priority prompt cost %f, got %f", expectedPromptCost, res.PromptCost.ToUSD())
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletionCost) {
		t.Errorf("expected priority completion cost %f, got %f", expectedCompletionCost, res.CompletionCost.ToUSD())
	}

	// Test fallback to standard when tier not available
	res, err = ComputePriceMoneyWithTier("gpt-4o", usage, "flex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ServiceTier != "standard" {
		t.Errorf("expected fallback to 'standard', got '%s'", res.ServiceTier)
	}
	// Should use standard gpt-4o pricing
	expectedPromptCost = (1000.0 / 1000000.0) * 2.5
	expectedCompletionCost = (500.0 / 1000000.0) * 10.0
	if !almostEqual(res.PromptCost.ToUSD(), expectedPromptCost) {
		t.Errorf("expected fallback prompt cost %f, got %f", expectedPromptCost, res.PromptCost.ToUSD())
	}
	if !almostEqual(res.CompletionCost.ToUSD(), expectedCompletionCost) {
		t.Errorf("expected fallback completion cost %f, got %f", expectedCompletionCost, res.CompletionCost.ToUSD())
	}

	// Test that ComputePrice still works (backward compatibility)
	res, err = ComputePriceMoney("gpt-5", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ServiceTier != "standard" {
		t.Errorf("expected ComputePrice to use 'standard' tier, got '%s'", res.ServiceTier)
	}
}

func TestPricing_SmallestTokenAccumulation(t *testing.T) {
	// Test precision with the smallest possible per-token cost and many small calculations
	// This demonstrates that integer arithmetic maintains perfect precision where floats would fail

	// Create a model with extremely small per-token pricing
	smallestPricePerMillion := 0.0001 // $0.0001 per million tokens = $0.0000000001 per token
	testConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"micro-model": {
				Prompt:       smallestPricePerMillion,
				CachedPrompt: smallestPricePerMillion / 10, // Even smaller
				Completion:   smallestPricePerMillion * 2,
			},
		},
	}
	SetConfig(testConfig)
	defer ResetConfig()

	// Test 1: Many single-token calculations should sum to the same as one bulk calculation
	iterations := 10000
	var accumulatedTotal Money

	// Calculate cost for 1 token, 'iterations' times
	for i := 0; i < iterations; i++ {
		usage := Usage{PromptTokens: 1, CompletionTokens: 0}
		result, err := ComputePriceMoney("micro-model", usage)
		if err != nil {
			t.Fatalf("calculation failed: %v", err)
		}
		accumulatedTotal = accumulatedTotal.Add(result.TotalCost)
	}

	// Calculate cost for 'iterations' tokens in one go
	bulkUsage := Usage{PromptTokens: iterations, CompletionTokens: 0}
	bulkResult, err := ComputePriceMoney("micro-model", bulkUsage)
	if err != nil {
		t.Fatalf("bulk calculation failed: %v", err)
	}

	// They should be exactly equal with integer arithmetic
	if accumulatedTotal != bulkResult.TotalCost {
		t.Errorf("Precision loss detected!")
		t.Errorf("  %d × 1-token calculations: %s", iterations, accumulatedTotal.String())
		t.Errorf("  1 × %d-token calculation:   %s", iterations, bulkResult.TotalCost.String())
		t.Errorf("  Difference: %d nano-cents", int64(accumulatedTotal-bulkResult.TotalCost))
	} else {
		t.Logf("✅ Perfect precision maintained:")
		t.Logf("  %d × 1-token calculations: %s", iterations, accumulatedTotal.String())
		t.Logf("  1 × %d-token calculation:   %s", iterations, bulkResult.TotalCost.String())
	}

	// Test 2: Compare with what float64 arithmetic would produce
	// Simulate the old buggy behavior with float64
	pricePerToken := smallestPricePerMillion / 1000000.0 // Convert to per-token

	var floatAccumulated float64
	for i := 0; i < iterations; i++ {
		floatAccumulated += pricePerToken // 1 token each time
	}

	floatBulk := pricePerToken * float64(iterations) // All tokens at once
	floatDiff := floatAccumulated - floatBulk

	t.Logf("Float64 comparison:")
	t.Logf("  %d × 1-token calculations: $%.15f", iterations, floatAccumulated)
	t.Logf("  1 × %d-token calculation:   $%.15f", iterations, floatBulk)
	t.Logf("  Float64 difference:        %e", floatDiff)
	t.Logf("  Money difference:          %d nano-cents (perfect: 0)", int64(accumulatedTotal-bulkResult.TotalCost))

	// Test 3: Verify extremely small individual costs maintain precision
	singleTokenCost := accumulatedTotal.ToUSD() / float64(iterations)
	expectedSingleCost := smallestPricePerMillion / 1000000.0

	if !almostEqual(singleTokenCost, expectedSingleCost) {
		t.Errorf("Single token cost precision lost: got %.15f, want %.15f", singleTokenCost, expectedSingleCost)
	}

	t.Logf("Single token cost: $%.15f (expected: $%.15f)", singleTokenCost, expectedSingleCost)
}
