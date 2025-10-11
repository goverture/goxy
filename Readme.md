# GoXY - AI API Proxy Server

[![Go Version](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://github.com/goverture/goxy/workflows/CI/badge.svg)](https://github.com/goverture/goxy/actions)

A high-performance, configurable proxy server for AI API endpoints with built-in rate limiting, usage tracking, and cost calculation.

## ✨ Features

- 🚀 **High Performance**: Efficient proxy with streaming support
- 📊 **Usage Tracking**: Track token usage and calculate costs
- 🛡️ **Rate Limiting**: Configurable rate limits with multiple algorithms
- 💰 **Cost Calculation**: YAML-configurable pricing for different models
- 🔄 **Flexible Routing**: Support for multiple AI API providers
- 📈 **Monitoring**: Comprehensive logging and metrics
- ⚡ **Streaming**: Full support for SSE streaming responses

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/goverture/goxy.git
cd goxy

# Build the binary
go build -o goxy

# Run with default settings
./goxy
```

### Basic Usage

```bash
# Default (OpenAI)
./goxy

# Custom base URL (like myproxy.local)
./goxy -openai-base-url=http://myproxy.local:8080/v1

# Or any other API endpoint
./goxy -openai-base-url=https://api.anthropic.com/v1
```

### Testing the Proxy

#### Non-streaming Request

```bash
curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","messages":[{"role":"user","content":"What is the capital of France?"}]}'
```

#### Streaming Request

```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"Stream a response"}]}'
```

## 📋 Example Response

```json
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 11,
    "completion_tokens": 8,
    "total_tokens": 19
  }
}
```

## ⚙️ Configuration

### Pricing Configuration

GoXY uses YAML-based pricing configuration. Create a `pricing/pricing.yaml` file:

```yaml
models:
  gpt-4:
    prompt: 0.03
    completion: 0.06
    aliases:
      - "gpt-4-0613"
  
  gpt-4o:
    prompt: 0.005
    completion: 0.015
    aliases:
      - "gpt-4o-2024-08-06"

# Default pricing for unknown models
default:
  prompt: 0.01
  completion: 0.02
  
# Cached token discount (90% discount = 10% cost)
cached_token_discount: 0.1
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-openai-base-url` | Base URL for the AI API | `https://api.openai.com/v1` |
| `-port` | Port to listen on | `8080` |
| `-rate-limit` | Requests per minute limit | `60` |

## 🏗️ Architecture

- **`main.go`**: Server setup and routing
- **`handlers/`**: HTTP request handlers and proxy logic
- **`limit/`**: Rate limiting algorithms and middleware
- **`pricing/`**: Cost calculation and pricing configuration
- **`config/`**: Configuration management

## 🛠️ Development

### Prerequisites

- Go 1.25 or later
- Git

### Building from Source

```bash
git clone https://github.com/goverture/goxy.git
cd goxy
go mod download
go build -o goxy
```

### Running Tests

```bash
go test ./...
```

### Running with Development Mode

```bash
go run main.go -openai-base-url=http://localhost:8080/v1
```

## 📚 API Documentation

GoXY is compatible with the OpenAI API specification. All endpoints that work with OpenAI's API will work with GoXY.

### Supported Endpoints

- `POST /v1/chat/completions` - Chat completions (streaming and non-streaming)
- `GET /health` - Health check endpoint

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- OpenAI for the API specification
- The Go community for excellent libraries
- All contributors to this project

## 📞 Support

- 📧 Create an issue on GitHub
- 💬 Join our discussions
- 📖 Check the documentation

---

**GoXY** - Making AI API proxying simple and efficient.