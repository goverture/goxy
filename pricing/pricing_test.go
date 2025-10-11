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
				Prompt:     0.03,
				Completion: 0.06,
				Aliases:    []string{"gpt-4-0613"},
			},
			"gpt-4o": {
				Prompt:     0.005,
				Completion: 0.015,
				Aliases:    []string{"gpt-4o-2024-08-06"},
			},
			"gpt-5-mini": {
				Prompt:     0.003,
				Completion: 0.006,
				Aliases:    []string{"gpt-5-mini-2025-08-07"},
			},
			"gpt-5": {
				Prompt:     0.01,
				Completion: 0.02,
				Aliases:    []string{"gpt-5-2025-08-07"},
			},
		},
		Default: &ModelPricing{
			Prompt:     0.01,
			Completion: 0.02,
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
	if res.Model != ModelGPT5Mini {
		t.Fatalf("expected model normalize to gpt-5-mini, got %s", res.Model)
	}
	expectedPrompt := 11.0 / 1000.0 * 0.003      // 0.000033
	expectedCompletion := 369.0 / 1000.0 * 0.006 // 0.002214
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
	expectedPromptCost := (billedPrompt / 1000.0) * 0.003
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
	expectedPromptCost := (100.0 / 1000.0) * 0.01     // default prompt rate
	expectedCompletionCost := (200.0 / 1000.0) * 0.02 // default completion rate
	if !almostEqual(res.PromptCostUSD, expectedPromptCost) {
		t.Fatalf("expected default prompt pricing, got %f want %f", res.PromptCostUSD, expectedPromptCost)
	}
	if !almostEqual(res.CompletionCostUSD, expectedCompletionCost) {
		t.Fatalf("expected default completion pricing, got %f want %f", res.CompletionCostUSD, expectedCompletionCost)
	}
}

func TestNormalizeModelVariants(t *testing.T) {
	cases := map[string]Model{
		"gpt-4o-2024-08-06": ModelGPT4o,
		"gpt-4-0613":        ModelGPT4,
	}
	for raw, want := range cases {
		got := NormalizeModel(raw)
		if got != want {
			t.Fatalf("NormalizeModel(%q) -> %q want %q", raw, got, want)
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
	expected := (1.0 / 1000.0) * 0.005
	if !almostEqual(res.PromptCostUSD, expected) {
		t.Fatalf("expected clamped cached pricing got %f want %f", res.PromptCostUSD, expected)
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
