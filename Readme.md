# Goxy 🕸️

Lightweight **OpenAI API proxy** with **spending limits**. Drop it in front of your app to control usage & avoid surprise bills.

**PSA: I'm vibing it at the moment, please don't rely on it too much until v1.0 !**

## Features

- [x] Hourly spending limit (once exceeded the proxy will return 429)
- [x] Admin port (view/update limit and usage)
- [x] Support for flex/priority service level pricing
- [x] Persistence across restart
- [ ] Support for streaming requests (currently only synchronous requests are supported)

## Supported endpoints

- [x] v1/chat/completions
- [x] v1/responses

## Supported models

- [x] "current" text models from https://platform.openai.com/docs/pricing#text-tokens
- [ ] "legacy" text models from https://platform.openai.com/docs/pricing#legacy-models

## TODOs

- [x] Don't use float64 for pricing computation (not precise)

## 🚀 Install

```bash
go install github.com/goverture/goxy@latest
```

## ⚙️ Example

Run the proxy

```bash
# 1.5$ per hour spending limit
goxy -l 1.5
```

Then point your app to `http://localhost:8080`.

```
# Completion API
curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5-nano","messages":[{"role":"user","content":"What is the capital of France?"}]}'

# Responses API
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4.1",
    "input": "Tell me a three sentence bedtime story about a unicorn."
  }'
```

Or in Python

```
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ.get("OPENAI_API_KEY"),
    base_url="http://localhost:8080/v1"
)

response = client.responses.create(
    model="gpt-4o",
    instructions="You are a coding assistant that talks like a pirate.",
    input="How do I check if a Python object is an instance of a class?",
)

print(response.output_text)
```

## Admin API

Admin interface runs on port 8081 (configurable with `-a`).

```bash
# View usage
curl http://localhost:8081/usage

# Update spending limit
curl -X PUT http://localhost:8081/limit \
  -H "Content-Type: application/json" \
  -d '{"limit_usd": 5.0}'

# Health check
curl http://localhost:8081/health
```

## 📜 License

MIT
