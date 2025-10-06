package pricing

import (
	"math"
	"strings"
	"testing"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestComputePrice_GPT5MiniSampleUsage(t *testing.T) {
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
	usage := Usage{PromptTokens: 100, CompletionTokens: 200}
	res, err := ComputePrice("some-new-future-model", usage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Note != "unknown model pricing" {
		t.Fatalf("expected unknown model note, got %q", res.Note)
	}
	if res.TotalCostUSD != 0 || res.PromptCostUSD != 0 || res.CompletionCostUSD != 0 {
		t.Fatalf("expected zero costs for unknown model, got %+v", res)
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
	res, err := ComputePrice("gpt-4o", Usage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.TotalCostUSD != 0 || res.PromptCostUSD != 0 || res.CompletionCostUSD != 0 {
		t.Fatalf("expected zero costs for zero usage, got %+v", res)
	}
}

func TestComputePrice_CachedExceedsPromptClamp(t *testing.T) {
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
