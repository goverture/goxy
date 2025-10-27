package pricing

import (
	"fmt"
)

// parseUsageFromResponse extracts usage information from API responses based on object type
func ParseUsageFromResponse(parsed map[string]interface{}) (Usage, bool) {
	objectType, _ := parsed["object"].(string)

	switch objectType {
	case "chat.completion":
		return parseChatCompletionUsage(parsed)
	case "response":
		return parseResponseAPIUsage(parsed)
	case "": // Missing object field - default to chat completion format for backward compatibility
		return parseChatCompletionUsage(parsed)
	default:
		fmt.Printf("[proxy] Warning: Unsupported object type '%s' for pricing calculation\n", objectType)
		return Usage{}, false
	}
}

// parseChatCompletionUsage extracts usage from chat completions API responses
func parseChatCompletionUsage(parsed map[string]interface{}) (Usage, bool) {
	usageRaw, ok := parsed["usage"].(map[string]interface{})
	if !ok {
		return Usage{}, false
	}

	u := Usage{}

	// Chat completions API uses prompt_tokens/completion_tokens
	if v, ok := usageRaw["prompt_tokens"].(float64); ok {
		u.PromptTokens = int(v)
	}
	if v, ok := usageRaw["completion_tokens"].(float64); ok {
		u.CompletionTokens = int(v)
	}

	// nested prompt_tokens_details.cached_tokens
	if detailsRaw, ok := usageRaw["prompt_tokens_details"].(map[string]interface{}); ok {
		if v, ok := detailsRaw["cached_tokens"].(float64); ok {
			u.PromptCachedTokens = int(v)
		}
	}

	return u, true
}

// parseResponseAPIUsage extracts usage from responses API responses
func parseResponseAPIUsage(parsed map[string]interface{}) (Usage, bool) {
	usageRaw, ok := parsed["usage"].(map[string]interface{})
	if !ok {
		return Usage{}, false
	}

	u := Usage{}

	// Responses API uses input_tokens/output_tokens
	if v, ok := usageRaw["input_tokens"].(float64); ok {
		u.PromptTokens = int(v)
	}
	if v, ok := usageRaw["output_tokens"].(float64); ok {
		u.CompletionTokens = int(v)
	}

	// nested input_tokens_details.cached_tokens
	if detailsRaw, ok := usageRaw["input_tokens_details"].(map[string]interface{}); ok {
		if v, ok := detailsRaw["cached_tokens"].(float64); ok {
			u.PromptCachedTokens = int(v)
		}
	}

	return u, true
}
