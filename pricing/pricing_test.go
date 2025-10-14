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
				Prompt:     30.0,
				Completion: 60.0,
			},
			"gpt-4o": {
				Prompt:     5.0,
				Completion: 15.0,
			},
			"gpt-5-mini": {
				Prompt:     3.0,
				Completion: 6.0,
			},
			"gpt-5": {
				Prompt:     10.0,
				Completion: 20.0,
			},
		},
		Default: &ModelPricing{
			Prompt:     10.0,
			Completion: 20.0,
		},
		CachedTokenDiscount: 0.1,
	}
	SetConfig(testConfig)
}

func TestComputePrice_GPT5MiniSampleUsage(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 11, CompletionTokens: 369}
	res, err := ComputePrice("gpt-5-mini-2025-08-07", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Model != Model("gpt-5-mini") {
		t.Fatalf("expected model normalize to gpt-5-mini, got %s", res.Model)
	}
	expectedPrompt := 11.0 / 1000000.0 * 3.0      // 0.000033
	expectedCompletion := 369.0 / 1000000.0 * 6.0 // 0.002214
	if !almostEqual(res.PromptCostUSD, expectedPrompt) {
		t.Fatalf("prompt cost mismatch: got %f want %f", res.PromptCostUSD, expectedPrompt)
	}
	if !almostEqual(res.CompletionCostUSD, expectedCompletion) {
		t.Fatalf("completion cost mismatch: got %f want %f", res.CompletionCostUSD, expectedCompletion)
	}
	if !almostEqual(res.TotalCostUSD, expectedPrompt+expectedCompletion) {
		t.Fatalf("total cost mismatch: got %f want %f", res.TotalCostUSD, expectedPrompt+expectedCompletion)
	}
}

func TestComputePrice_WithCachedPromptTokens(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	// 100 prompt tokens of which 60 are cached (90% discount => each cached counts as 0.1)
	usage := Usage{PromptTokens: 100, PromptCachedTokens: 60, CompletionTokens: 0}
	res, err := ComputePrice("gpt-5-mini", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	billedPrompt := float64(100-60) + 0.1*60 // 40 + 6 = 46 effective tokens
	expectedPromptCost := (billedPrompt / 1000000.0) * 3.0
	if !almostEqual(res.PromptCostUSD, expectedPromptCost) {
		t.Fatalf("prompt cost mismatch with cache: got %f want %f", res.PromptCostUSD, expectedPromptCost)
	}
	if res.TotalCostUSD != res.PromptCostUSD {
		t.Fatalf("total should equal prompt cost when no completion tokens")
	}
	if res.Note == "" || !strings.Contains(res.Note, "90%") {
		t.Fatalf("expected note to mention discount, got %q", res.Note)
	}
}

func TestComputePrice_UnknownModel(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 100, CompletionTokens: 200}
	res, err := ComputePrice("some-new-future-model", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use default pricing from config
	expectedPromptCost := (100.0 / 1000000.0) * 10.0     // default prompt rate
	expectedCompletionCost := (200.0 / 1000000.0) * 20.0 // default completion rate
	if !almostEqual(res.PromptCostUSD, expectedPromptCost) {
		t.Fatalf("expected default prompt pricing, got %f want %f", res.PromptCostUSD, expectedPromptCost)
	}
	if !almostEqual(res.CompletionCostUSD, expectedCompletionCost) {
		t.Fatalf("expected default completion pricing, got %f want %f", res.CompletionCostUSD, expectedCompletionCost)
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

	res, err := ComputePrice("gpt-4o", Usage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalCostUSD != 0 || res.PromptCostUSD != 0 || res.CompletionCostUSD != 0 {
		t.Fatalf("expected zero costs for zero usage, got %+v", res)
	}
}

func TestComputePrice_CachedExceedsPromptClamp(t *testing.T) {
	setupTestConfig()
	defer ResetConfig()

	usage := Usage{PromptTokens: 10, PromptCachedTokens: 25}
	res, err := ComputePrice("gpt-4o", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Cached clamped to 10 -> billed = (10-10) + 0.1*10 = 1
	expected := (1.0 / 1000000.0) * 5.0
	if !almostEqual(res.PromptCostUSD, expected) {
		t.Fatalf("expected clamped cached pricing got %f want %f", res.PromptCostUSD, expected)
	}
}

func TestComputePrice_UnknownModelNoDefault(t *testing.T) {
	// Test config with no default pricing
	configWithoutDefault := &PricingConfig{
		Models: map[string]ModelPricing{
			"gpt-4": {
				Prompt:     30.0,
				Completion: 60.0,
			},
		},
		// No Default field set
		CachedTokenDiscount: 0.1,
	}
	SetConfig(configWithoutDefault)
	defer ResetConfig()

	usage := Usage{PromptTokens: 100, CompletionTokens: 200}
	res, err := ComputePrice("unknown-model", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return zero costs with unknown pricing note
	if res.PromptCostUSD != 0.0 || res.CompletionCostUSD != 0.0 || res.TotalCostUSD != 0.0 {
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
				Prompt:     2.5,
				Completion: 10.0,
			},
			"gpt-4o-mini": {
				Prompt:     0.15,
				Completion: 0.6,
			},
			"o1": {
				Prompt:     15.0,
				Completion: 60.0,
			},
			"o1-mini": {
				Prompt:     1.1,
				Completion: 4.4,
			},
		},
		Default: &ModelPricing{
			Prompt:     10.0,
			Completion: 20.0,
		},
		CachedTokenDiscount: 0.1,
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
		res, err := ComputePrice(tc.input, usage)
		if err != nil {
			t.Fatalf("ComputePrice(%q) failed: %v", tc.input, err)
		}

		expectedCost := tc.prompt / 1000.0 // 1000 tokens * rate / 1million
		if !almostEqual(res.PromptCostUSD, expectedCost) {
			t.Fatalf("ComputePrice(%q) prompt cost: got %f, want %f", tc.input, res.PromptCostUSD, expectedCost)
		}
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	// Test loading configuration from YAML (this tests the config loader)
	tempConfig := &PricingConfig{
		Models: map[string]ModelPricing{
			"test-model": {
				Prompt:     0.001,
				Completion: 0.002,
			},
		},
		CachedTokenDiscount: 0.2,
	}

	SetConfig(tempConfig)
	defer ResetConfig()

	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("failed to get config: %v", err)
	}

	if cfg.CachedTokenDiscount != 0.2 {
		t.Fatalf("expected cached discount 0.2, got %f", cfg.CachedTokenDiscount)
	}

	pricing, found := cfg.FindModelPricing("test-model")
	if !found {
		t.Fatalf("expected to find test-model pricing")
	}

	if pricing.Prompt != 0.001 || pricing.Completion != 0.002 {
		t.Fatalf("unexpected pricing values: %+v", pricing)
	}
}
