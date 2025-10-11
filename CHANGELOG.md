# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-10-11

### Added
- Initial release of GoXY AI API Proxy Server
- HTTP proxy server with streaming support for AI API endpoints
- Configurable YAML-based pricing system for different AI models
- Rate limiting with multiple algorithms (token bucket, fixed window)
- Usage tracking and cost calculation with cached token discount support
- Comprehensive logging and request/response capture
- Model alias support for pricing configuration
- Health check endpoint
- CI/CD pipeline with automated testing
- Complete test coverage for all components

### Features
- **Proxy Handler**: Efficient HTTP proxy with streaming SSE support
- **Pricing System**: YAML-configurable pricing with model aliases and default fallback
- **Rate Limiting**: Configurable per-minute request limits with 429 status handling
- **Usage Tracking**: Token usage monitoring with hourly spending limits
- **Monitoring**: Request/response logging with detailed metrics
- **Configuration**: Environment-based and file-based configuration options

### Technical Details
- Built with Go 1.25
- Zero external runtime dependencies for core functionality
- Thread-safe configuration loading
- Graceful error handling and recovery
- Support for OpenAI API specification compliance

[Unreleased]: https://github.com/goverture/goxy/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/goverture/goxy/releases/tag/v1.0.0