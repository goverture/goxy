# GoXY Makefile

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS = -s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	go build -ldflags="$(LDFLAGS)" -o goxy

# Build for release (multiple platforms)
.PHONY: build-all
build-all:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/goxy-linux-amd64
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/goxy-linux-arm64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/goxy-darwin-amd64
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/goxy-darwin-arm64
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/goxy-windows-amd64.exe

# Run tests
.PHONY: test
test:
	go test ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with race detector
.PHONY: test-race
test-race:
	go test -race ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linters
.PHONY: lint
lint:
	go vet ./...
	gofmt -s -l .

# Clean build artifacts
.PHONY: clean
clean:
	rm -f goxy
	rm -rf dist/
	rm -f coverage.out coverage.html

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod verify

# Run the application
.PHONY: run
run: build
	./goxy

# Development run (with hot reload using air if available)
.PHONY: dev
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running without hot reload..."; \
		go run main.go; \
	fi

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-all    - Build for all platforms"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  test-race    - Run tests with race detector"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linters"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run in development mode"
	@echo "  help         - Show this help message"

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"