# Goxy 🕸️

Lightweight **OpenAI API proxy** with **spending limits**. Drop it in front of your app to control usage & avoid surprise bills.

## Features

- [x] Hourly spending limit (once exceeded the proxy will return 429)
- [ ] Admin port (view/update limit and usage)
- [ ] Support for streaming requests (currently only synchronous requests are supported)

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
curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"What is the capital of France?"}]}'
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

## 📜 License

MIT
