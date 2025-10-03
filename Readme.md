# Default (OpenAI)
go run main.go

# Custom base URL (like myproxy.local)
go run main.go -openai-base-url=http://myproxy.local:8080/v1

# Or any other API endpoint
go run main.go -openai-base-url=https://api.anthropic.com/v1

# Non-stream

curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5-mini","messages":[{"role":"user","content":"What model are you ?"}]}'

# Streaming

curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"Stream please"}]}'


# Response

{
  "id": "chatcmpl-CKPj015cnmsX6s6MREPP0nG79e66x",
  "object": "chat.completion",
  "created": 1758981482,
  "model": "gpt-5-mini-2025-08-07",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "I'm ChatGPT, an AI language model created by OpenAI and based on the GPT-4 family. I don't have a specific API model tag to show inside this chat, but my capabilities and behavior follow the GPT-4-era architecture.\n\nKnowledge cutoff: June 2024. If you want details about exact model names to use with the API (for example gpt-4, gpt-4o, gpt-4o-mini, etc.), tell me what you need and I can help guide you.",
        "refusal": null,
        "annotations": []
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 11,
    "completion_tokens": 369,
    "total_tokens": 380,
    "prompt_tokens_details": {
      "cached_tokens": 0,
      "audio_tokens": 0
    },
    "completion_tokens_details": {
      "reasoning_tokens": 256,
      "audio_tokens": 0,
      "accepted_prediction_tokens": 0,
      "rejected_prediction_tokens": 0
    }
  },
  "service_tier": "default",
  "system_fingerprint": null
}