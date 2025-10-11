# Contributing to GoXY

Thank you for your interest in contributing to GoXY! This document provides guidelines and information for contributors.

## 🤝 How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include:

- **Clear description** of the issue
- **Steps to reproduce** the behavior
- **Expected behavior** vs actual behavior
- **Environment details** (Go version, OS, etc.)
- **Logs or error messages** if applicable

### Suggesting Features

Feature requests are welcome! Please:

- **Check existing issues** for similar requests
- **Provide clear description** of the feature
- **Explain the use case** and why it would be valuable
- **Consider implementation complexity** and maintenance burden

### Pull Requests

1. **Fork** the repository
2. **Create a feature branch** from `master`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make your changes** following our coding standards
4. **Add tests** for new functionality
5. **Update documentation** if needed
6. **Commit with clear messages**:
   ```bash
   git commit -m "Add feature: brief description"
   ```
7. **Push to your fork** and submit a pull request

## 🛠️ Development Setup

### Prerequisites

- Go 1.25 or later
- Git
- Make (optional, for convenience commands)

### Local Development

```bash
# Clone your fork
git clone https://github.com/yourusername/goxy.git
cd goxy

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the project
go build -o goxy

# Run locally
./goxy
```

### Testing

We maintain high test coverage. Please ensure:

- **New code has tests**: All new functions should have corresponding tests
- **Tests pass**: Run `go test ./...` before submitting
- **Test edge cases**: Consider error conditions and boundary cases

### Code Style

- **Follow Go conventions**: Use `gofmt`, `go vet`, and `golint`
- **Write clear code**: Prefer readability over cleverness
- **Add comments**: Document public functions and complex logic
- **Use meaningful names**: Variables and functions should be self-documenting

## 📋 Code Guidelines

### Go Standards

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Handle errors explicitly and appropriately
- Use context.Context for cancellation and timeouts

### Project Structure

```
goxy/
├── main.go           # Application entry point
├── config/           # Configuration management
├── handlers/         # HTTP handlers and proxy logic
├── limit/           # Rate limiting algorithms
├── pricing/         # Cost calculation and pricing
└── docs/            # Documentation
```

### Testing Standards

- **Unit tests**: Test individual functions in isolation
- **Integration tests**: Test component interactions
- **Table-driven tests**: Use for testing multiple scenarios
- **Test helpers**: Create reusable test utilities
- **Mocking**: Mock external dependencies appropriately

### Documentation

- **README updates**: Keep the README current with new features
- **Code comments**: Document public APIs and complex logic
- **Examples**: Provide usage examples for new features
- **CHANGELOG**: Update for all user-facing changes

## 🔄 Release Process

### Version Numbers

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes

### Release Checklist

- [ ] Update version in relevant files
- [ ] Update CHANGELOG.md
- [ ] Run full test suite
- [ ] Update documentation
- [ ] Create git tag
- [ ] Publish release

## 📞 Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Code Review**: We provide constructive feedback on PRs

## 🎯 Areas for Contribution

We especially welcome contributions in these areas:

- **Performance optimizations**: Improve proxy efficiency
- **New rate limiting algorithms**: Additional limiting strategies
- **Monitoring and metrics**: Enhanced observability
- **Documentation**: Tutorials, examples, and guides
- **Testing**: Improve test coverage and reliability
- **CI/CD**: Enhance automation and deployment

## 📜 Code of Conduct

This project follows the [Go Community Code of Conduct](https://golang.org/conduct). Please be respectful and inclusive in all interactions.

## 🙏 Recognition

Contributors will be recognized in:

- Release notes for significant contributions
- README acknowledgments
- GitHub contributor stats

Thank you for contributing to GoXY! 🚀