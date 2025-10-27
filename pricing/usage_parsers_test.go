package pricing

import (
	"testing"
)

func TestParseUsageFromResponse_ChatCompletion(t *testing.T) {
	response := map[string]interface{}{
		"object": "chat.completion",
		"model":  "gpt-4o",
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(100),
			"completion_tokens": float64(50),
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens": float64(10),
			},
		},
	}

	usage, ok := ParseUsageFromResponse(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       100,
		CompletionTokens:   50,
		PromptCachedTokens: 10,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseUsageFromResponse_ResponseAPI(t *testing.T) {
	response := map[string]interface{}{
		"object": "response",
		"model":  "gpt-4.1",
		"usage": map[string]interface{}{
			"input_tokens":  float64(18),
			"output_tokens": float64(64),
			"input_tokens_details": map[string]interface{}{
				"cached_tokens": float64(5),
			},
		},
	}

	usage, ok := ParseUsageFromResponse(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       18,
		CompletionTokens:   64,
		PromptCachedTokens: 5,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseUsageFromResponse_MissingObjectField(t *testing.T) {
	// Test backward compatibility for responses without object field
	response := map[string]interface{}{
		"model": "gpt-4o",
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(200),
			"completion_tokens": float64(0),
		},
	}

	usage, ok := ParseUsageFromResponse(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:     200,
		CompletionTokens: 0,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseUsageFromResponse_UnsupportedObjectType(t *testing.T) {
	response := map[string]interface{}{
		"object": "embedding",
		"model":  "text-embedding-ada-002",
		"usage": map[string]interface{}{
			"prompt_tokens": float64(100),
		},
	}

	_, ok := ParseUsageFromResponse(response)
	if ok {
		t.Fatal("Expected parsing to fail for unsupported object type")
	}
}

func TestParseUsageFromResponse_MissingUsageField(t *testing.T) {
	response := map[string]interface{}{
		"object": "chat.completion",
		"model":  "gpt-4o",
		// No usage field
	}

	_, ok := ParseUsageFromResponse(response)
	if ok {
		t.Fatal("Expected parsing to fail when usage field is missing")
	}
}

func TestParseChatCompletionUsage_Complete(t *testing.T) {
	response := map[string]interface{}{
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(150),
			"completion_tokens": float64(75),
			"prompt_tokens_details": map[string]interface{}{
				"cached_tokens": float64(20),
			},
		},
	}

	usage, ok := parseChatCompletionUsage(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       150,
		CompletionTokens:   75,
		PromptCachedTokens: 20,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseChatCompletionUsage_NoCachedTokens(t *testing.T) {
	response := map[string]interface{}{
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(100),
			"completion_tokens": float64(50),
			// No prompt_tokens_details
		},
	}

	usage, ok := parseChatCompletionUsage(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       100,
		CompletionTokens:   50,
		PromptCachedTokens: 0, // Should default to 0
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseResponseAPIUsage_Complete(t *testing.T) {
	response := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":  float64(25),
			"output_tokens": float64(120),
			"input_tokens_details": map[string]interface{}{
				"cached_tokens": float64(8),
			},
		},
	}

	usage, ok := parseResponseAPIUsage(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       25,
		CompletionTokens:   120,
		PromptCachedTokens: 8,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseResponseAPIUsage_NoCachedTokens(t *testing.T) {
	response := map[string]interface{}{
		"usage": map[string]interface{}{
			"input_tokens":  float64(30),
			"output_tokens": float64(90),
			// No input_tokens_details
		},
	}

	usage, ok := parseResponseAPIUsage(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	expected := Usage{
		PromptTokens:       30,
		CompletionTokens:   90,
		PromptCachedTokens: 0, // Should default to 0
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}

func TestParseResponseAPIUsage_MissingUsage(t *testing.T) {
	response := map[string]interface{}{
		"model": "gpt-4.1",
		// No usage field
	}

	_, ok := parseResponseAPIUsage(response)
	if ok {
		t.Fatal("Expected parsing to fail when usage field is missing")
	}
}

func TestParseUsageFromResponse_InvalidTokenTypes(t *testing.T) {
	// Test with non-numeric token values
	response := map[string]interface{}{
		"object": "chat.completion",
		"usage": map[string]interface{}{
			"prompt_tokens":     "invalid",
			"completion_tokens": float64(50),
		},
	}

	usage, ok := ParseUsageFromResponse(response)
	if !ok {
		t.Fatal("Expected successful parsing")
	}

	// Should default to 0 for invalid values
	expected := Usage{
		PromptTokens:     0,
		CompletionTokens: 50,
	}

	if usage != expected {
		t.Errorf("Expected %+v, got %+v", expected, usage)
	}
}
