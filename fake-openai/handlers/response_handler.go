package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// ResponseRequest represents the incoming request for the responses API
type ResponseRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// ResponseAPIResponse represents the OpenAI responses API response
type ResponseAPIResponse struct {
	ID         string `json:"id"`
	Object     string `json:"object"`
	CreatedAt  int64  `json:"created_at"`
	Status     string `json:"status"`
	Background bool   `json:"background"`
	Billing    struct {
		Payer string `json:"payer"`
	} `json:"billing"`
	Error              interface{} `json:"error"`
	IncompleteDetails  interface{} `json:"incomplete_details"`
	Instructions       interface{} `json:"instructions"`
	MaxOutputTokens    interface{} `json:"max_output_tokens"`
	MaxToolCalls       interface{} `json:"max_tool_calls"`
	Model              string      `json:"model"`
	Output             []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Content []struct {
			Type        string        `json:"type"`
			Annotations []interface{} `json:"annotations"`
			Logprobs    []interface{} `json:"logprobs"`
			Text        string        `json:"text"`
		} `json:"content"`
		Role string `json:"role"`
	} `json:"output"`
	ParallelToolCalls    bool        `json:"parallel_tool_calls"`
	PreviousResponseID   interface{} `json:"previous_response_id"`
	PromptCacheKey       interface{} `json:"prompt_cache_key"`
	Reasoning            struct {
		Effort  interface{} `json:"effort"`
		Summary interface{} `json:"summary"`
	} `json:"reasoning"`
	SafetyIdentifier string  `json:"safety_identifier"`
	ServiceTier      string  `json:"service_tier"`
	Store            bool    `json:"store"`
	Temperature      float64 `json:"temperature"`
	Text             struct {
		Format struct {
			Type string `json:"type"`
		} `json:"format"`
		Verbosity string `json:"verbosity"`
	} `json:"text"`
	ToolChoice   string        `json:"tool_choice"`
	Tools        []interface{} `json:"tools"`
	TopLogprobs  int           `json:"top_logprobs"`
	TopP         float64       `json:"top_p"`
	Truncation   string        `json:"truncation"`
	Usage        struct {
		InputTokens int `json:"input_tokens"`
		InputTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"input_tokens_details"`
		OutputTokens int `json:"output_tokens"`
		OutputTokensDetails struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"output_tokens_details"`
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	User     interface{} `json:"user"`
	Metadata struct{}    `json:"metadata"`
}

// HandleResponses handles the responses endpoint
func HandleResponses(w http.ResponseWriter, r *http.Request) {
	// Add realistic OpenAI headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
	w.Header().Set("Openai-Organization", "fake-org")
	w.Header().Set("Openai-Processing-Ms", "300")
	w.Header().Set("Openai-Project", "proj_fake123")
	w.Header().Set("Openai-Version", "2020-10-01")
	w.Header().Set("X-Request-Id", fmt.Sprintf("req_fake%d", time.Now().Unix()))
	w.Header().Set("X-Ratelimit-Limit-Requests", "5000")
	w.Header().Set("X-Ratelimit-Limit-Tokens", "20000000")
	w.Header().Set("X-Ratelimit-Remaining-Requests", "4999")
	w.Header().Set("X-Ratelimit-Remaining-Tokens", "19999900")
	w.Header().Set("X-Ratelimit-Reset-Requests", "12ms")
	w.Header().Set("X-Ratelimit-Reset-Tokens", "3ms")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Generate a fake response based on the input
	responseText := generateResponseContent(req.Input)
	
	// Create response ID and message ID
	responseID := fmt.Sprintf("resp_%032x", time.Now().UnixNano())
	messageID := fmt.Sprintf("msg_%032x", time.Now().UnixNano()+1)
	
	// Build the response structure
	response := ResponseAPIResponse{
		ID:         responseID,
		Object:     "response",
		CreatedAt:  time.Now().Unix(),
		Status:     "completed",
		Background: false,
		Billing: struct {
			Payer string `json:"payer"`
		}{
			Payer: "developer",
		},
		Error:             nil,
		IncompleteDetails: nil,
		Instructions:      nil,
		MaxOutputTokens:   nil,
		MaxToolCalls:      nil,
		Model:             req.Model,
		Output: []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Status  string `json:"status"`
			Content []struct {
				Type        string        `json:"type"`
				Annotations []interface{} `json:"annotations"`
				Logprobs    []interface{} `json:"logprobs"`
				Text        string        `json:"text"`
			} `json:"content"`
			Role string `json:"role"`
		}{
			{
				ID:     messageID,
				Type:   "message",
				Status: "completed",
				Content: []struct {
					Type        string        `json:"type"`
					Annotations []interface{} `json:"annotations"`
					Logprobs    []interface{} `json:"logprobs"`
					Text        string        `json:"text"`
				}{
					{
						Type:        "output_text",
						Annotations: []interface{}{},
						Logprobs:    []interface{}{},
						Text:        responseText,
					},
				},
				Role: "assistant",
			},
		},
		ParallelToolCalls:  true,
		PreviousResponseID: nil,
		PromptCacheKey:     nil,
		Reasoning: struct {
			Effort  interface{} `json:"effort"`
			Summary interface{} `json:"summary"`
		}{
			Effort:  nil,
			Summary: nil,
		},
		SafetyIdentifier: "",
		ServiceTier:      "default",
		Store:            true,
		Temperature:      1.0,
		Text: struct {
			Format struct {
				Type string `json:"type"`
			} `json:"format"`
			Verbosity string `json:"verbosity"`
		}{
			Format: struct {
				Type string `json:"type"`
			}{
				Type: "text",
			},
			Verbosity: "medium",
		},
		ToolChoice:  "auto",
		Tools:       []interface{}{},
		TopLogprobs: 0,
		TopP:        1.0,
		Truncation:  "disabled",
		User:        nil,
		Metadata:    struct{}{},
	}

	// Set realistic token usage
	inputTokens := estimateTokens(req.Input)
	outputTokens := estimateTokens(responseText)
	
	response.Usage.InputTokens = inputTokens
	response.Usage.InputTokensDetails.CachedTokens = 0
	response.Usage.OutputTokens = outputTokens
	response.Usage.OutputTokensDetails.ReasoningTokens = 0
	response.Usage.TotalTokens = inputTokens + outputTokens

	// Add a small delay to simulate processing time
	time.Sleep(150 * time.Millisecond)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	log.Printf("Served fake response for model=%s, input_tokens=%d, output_tokens=%d",
		req.Model, inputTokens, outputTokens)
}

func generateResponseContent(input string) string {
	responses := map[string]string{
		"unicorn": "Once upon a time, a gentle unicorn named Luna discovered a glowing flower in the moonlit forest. When she touched it with her horn, stars began to sparkle all around her, lighting up the night. Luna smiled, knowing she had found a little magic to share sweet dreams with all her friends as they slept.",
		"story":   "In a land far away, there lived a kind dragon who loved to paint rainbows across the sky. Every morning, he would wake up early and use his colorful breath to create beautiful arcs of light. All the woodland creatures would gather to watch and cheer as their friend filled their world with wonder.",
		"bedtime": "A sleepy little owl named Oliver sat on his favorite branch, watching the moon rise slowly in the sky. As the stars twinkled above, he told himself a gentle story about floating on soft clouds. Soon his eyes grew heavy, and he drifted off to the most peaceful dreams.",
		"dragon":  "A friendly dragon named Ember lived in a cozy cave filled with books instead of treasure. Every evening, she would read stories to the village children who visited her home. When it was time for them to sleep, Ember would gently blow warm, sparkly air to keep them comfortable all night long.",
		"magic":   "In an enchanted garden, magical flowers bloomed only under the starlight. A young fairy named Willow tended to these special plants each night, singing soft lullabies to help them grow. As she worked, her gentle songs drifted through the air, bringing peaceful dreams to everyone nearby.",
	}

	input = strings.ToLower(input)
	for keyword, response := range responses {
		if strings.Contains(input, keyword) {
			return response
		}
	}

	return "I'm a fake OpenAI responses API. I received your input and I'm responding with this generic answer for testing purposes. This is a simulated response to help you test your integration."
}