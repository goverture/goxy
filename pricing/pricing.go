package pricing

import "fmt"

// Model represents a supported model for pricing.
type Model string

const (
	ModelGPT4     Model = "gpt-4"
	ModelGPT4o    Model = "gpt-4o"
	ModelGPT5Mini Model = "gpt-5-mini"
	ModelGPT5     Model = "gpt-5" // placeholder name
)

// Usage contains token usage fields we care about.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// PriceResult holds the computed pricing info.
type PriceResult struct {
	Model             Model
	PromptTokens      int
	CompletionTokens  int
	PromptCostUSD     float64
	CompletionCostUSD float64
	TotalCostUSD      float64
	Note              string
}

// Hard-coded per-1K token pricing (USD). These are illustrative and may not reflect current real pricing.
// Update these values as pricing changes. (Numbers chosen for example.)
var perK = map[Model]struct {
	Prompt     float64
	Completion float64
}{
	ModelGPT4:     {Prompt: 0.03, Completion: 0.06},
	ModelGPT4o:    {Prompt: 0.005, Completion: 0.015},
	ModelGPT5Mini: {Prompt: 0.003, Completion: 0.006},
	ModelGPT5:     {Prompt: 0.01, Completion: 0.02},
}

// NormalizeModel tries to map a raw model string to one of our known buckets.
func NormalizeModel(raw string) Model {
	switch {
	case raw == "gpt-4" || raw == "gpt-4-0613":
		return ModelGPT4
	case raw == "gpt-4o" || raw == "gpt-4o-2024-08-06":
		return ModelGPT4o
	case raw == "gpt-5-mini" || raw == "gpt-5-mini-2025-08-07":
		return ModelGPT5Mini
	case raw == "gpt-5" || raw == "gpt-5-2025-08-07":
		return ModelGPT5
	default:
		// naive fallback: detect prefix
		if len(raw) >= 5 && raw[:5] == "gpt-4" {
			return ModelGPT4
		}
		if len(raw) >= 5 && raw[:5] == "gpt-5" {
			return ModelGPT5
		}
		return Model(raw)
	}
}

// ComputePrice calculates cost given usage and model.
func ComputePrice(modelRaw string, u Usage) (PriceResult, error) {
	m := NormalizeModel(modelRaw)
	tiers, ok := perK[m]
	if !ok {
		return PriceResult{Model: m, PromptTokens: u.PromptTokens, CompletionTokens: u.CompletionTokens, Note: "unknown model pricing"}, nil
	}
	ptCost := (float64(u.PromptTokens) / 1000.0) * tiers.Prompt
	ctCost := (float64(u.CompletionTokens) / 1000.0) * tiers.Completion
	total := ptCost + ctCost
	return PriceResult{
		Model:             m,
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		PromptCostUSD:     ptCost,
		CompletionCostUSD: ctCost,
		TotalCostUSD:      total,
		Note:              "prices hard-coded; verify against https://openai.com/api/pricing/",
	}, nil
}

func (pr PriceResult) String() string {
	return fmt.Sprintf("[pricing] model=%s prompt=%d completion=%d cost_prompt=$%.6f cost_completion=$%.6f total=$%.6f", pr.Model, pr.PromptTokens, pr.CompletionTokens, pr.PromptCostUSD, pr.CompletionCostUSD, pr.TotalCostUSD)
}
