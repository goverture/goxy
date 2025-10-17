package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// FakeOpenAIResponse represents a typical OpenAI chat completion response
type FakeOpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role        string      `json:"role"`
			Content     string      `json:"content"`
			Refusal     interface{} `json:"refusal"`
			Annotations []string    `json:"annotations"`
		} `json:"message"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
			AudioTokens  int `json:"audio_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens          int `json:"reasoning_tokens"`
			AudioTokens              int `json:"audio_tokens"`
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	ServiceTier       string `json:"service_tier"`
	SystemFingerprint string `json:"system_fingerprint"`
}

// ChatCompletionRequest represents the incoming request
type ChatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// HandleChatCompletions handles the chat completions endpoint
func HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Add realistic OpenAI headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	w.Header().Set("Openai-Organization", "fake-org")
	w.Header().Set("Openai-Processing-Ms", "250")
	w.Header().Set("Openai-Project", "proj_fake123")
	w.Header().Set("Openai-Version", "2020-10-01")
	w.Header().Set("X-Request-Id", fmt.Sprintf("req_fake%d", time.Now().Unix()))
	w.Header().Set("X-Ratelimit-Limit-Requests", "10000")
	w.Header().Set("X-Ratelimit-Limit-Tokens", "30000000")
	w.Header().Set("X-Ratelimit-Remaining-Requests", "9999")
	w.Header().Set("X-Ratelimit-Remaining-Tokens", "29999900")
	w.Header().Set("X-Ratelimit-Reset-Requests", "6ms")
	w.Header().Set("X-Ratelimit-Reset-Tokens", "0s")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Extract the user's question for a more realistic response
	userContent := "Hello"
	if len(req.Messages) > 0 {
		userContent = req.Messages[len(req.Messages)-1].Content
	}

	// Generate a fake but realistic response
	response := FakeOpenAIResponse{
		ID:      fmt.Sprintf("chatcmpl-fake%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role        string      `json:"role"`
				Content     string      `json:"content"`
				Refusal     interface{} `json:"refusal"`
				Annotations []string    `json:"annotations"`
			} `json:"message"`
			Logprobs     interface{} `json:"logprobs"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role        string      `json:"role"`
					Content     string      `json:"content"`
					Refusal     interface{} `json:"refusal"`
					Annotations []string    `json:"annotations"`
				}{
					Role:        "assistant",
					Content:     generateFakeResponse(userContent),
					Refusal:     nil,
					Annotations: []string{},
				},
				Logprobs:     nil,
				FinishReason: "stop",
			},
		},
		ServiceTier:       "standard",
		SystemFingerprint: "fp_fake12345",
	}

	// Set realistic token usage based on model
	promptTokens := 0 // estimateTokens(userContent) + 10 // Add some system tokens
	completionTokens := 0 // estimateTokens(response.Choices[0].Message.Content)

	response.Usage.PromptTokens = promptTokens
	response.Usage.CompletionTokens = completionTokens
	response.Usage.TotalTokens = promptTokens + completionTokens
	response.Usage.PromptTokensDetails.CachedTokens = 1
	response.Usage.PromptTokensDetails.AudioTokens = 0
	response.Usage.CompletionTokensDetails.ReasoningTokens = 0
	response.Usage.CompletionTokensDetails.AudioTokens = 0
	response.Usage.CompletionTokensDetails.AcceptedPredictionTokens = 0
	response.Usage.CompletionTokensDetails.RejectedPredictionTokens = 0

	// Add a small delay to simulate processing time
	time.Sleep(100 * time.Millisecond)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	log.Printf("Served fake completion for model=%s, prompt_tokens=%d, completion_tokens=%d",
		req.Model, promptTokens, completionTokens)
}

func generateFakeResponse(userContent string) string {
	responses := map[string]string{
		"capital of france": "The capital of France is Paris.",
		"hello":             "Hello! How can I help you today?",
		"what is 2+2":       "2 + 2 = 4",
		"weather":           "I don't have access to real-time weather data, but you can check a weather website for current conditions.",
		"time":              "I don't have access to real-time information, but you can check your device's clock for the current time.",
		"test":              "This is a test response from the fake OpenAI server.",
	}

	content := strings.ToLower(userContent)
	for keyword, response := range responses {
		if strings.Contains(content, keyword) {
			return response
		}
	}

	return "I'm a fake OpenAI server. I received your message and I'm responding with this generic answer for testing purposes."
}

func estimateTokens(text string) int {
	// Very rough estimation: ~4 characters per token
	return max(1, len(text)/4)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
