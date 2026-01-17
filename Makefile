# Makefile for ACMG-AMP MCP Server
# Provides targets for building, testing, and releasing

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Binary names
BINARY_NAME=mcp-server
BINARY_LITE=mcp-server-lite

# Build directories
BUILD_DIR=build
DIST_DIR=dist

# Version (can be overridden: make VERSION=1.0.0 release)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Linker flags
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Docker settings
DOCKER_IMAGE=acmg-amp-mcp-server
DOCKER_IMAGE_LITE=acmg-amp-mcp-server-lite
DOCKER_TAG ?= $(VERSION)

.PHONY: all build build-lite clean test test-coverage lint deps docker docker-lite help

# Default target
all: test build build-lite

# Build the full server (requires external databases)
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/mcp-server

# Build the lite server (no external databases)
build-lite:
	@echo "Building $(BINARY_LITE) (lightweight, no external dependencies)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_LITE) ./cmd/mcp-server-lite

# Cross-compile lite server for multiple platforms
build-lite-all: build-lite-linux build-lite-darwin build-lite-windows

build-lite-linux:
	@echo "Building $(BINARY_LITE) for Linux (amd64)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_LITE)-linux-amd64 ./cmd/mcp-server-lite
	@echo "Building $(BINARY_LITE) for Linux (arm64)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_LITE)-linux-arm64 ./cmd/mcp-server-lite

build-lite-darwin:
	@echo "Building $(BINARY_LITE) for macOS (amd64)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_LITE)-darwin-amd64 ./cmd/mcp-server-lite
	@echo "Building $(BINARY_LITE) for macOS (arm64)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_LITE)-darwin-arm64 ./cmd/mcp-server-lite

build-lite-windows:
	@echo "Building $(BINARY_LITE) for Windows (amd64)..."
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_LITE)-windows-amd64.exe ./cmd/mcp-server-lite

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests for lightweight components only
test-lite:
	@echo "Running lightweight component tests..."
	$(GOTEST) -v ./internal/feedback/... ./internal/cache/... ./internal/config/... -run "SQLite|Memory|Lite"

# Run linter
lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; }
	golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html

# Build Docker image (full version)
docker:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

# Build Docker image (lite version)
docker-lite:
	@echo "Building Docker image $(DOCKER_IMAGE_LITE):$(DOCKER_TAG)..."
	docker build -f Dockerfile.lite -t $(DOCKER_IMAGE_LITE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE_LITE):$(DOCKER_TAG) $(DOCKER_IMAGE_LITE):latest

# Run lite server locally
run-lite: build-lite
	@echo "Running $(BINARY_LITE)..."
	./$(BUILD_DIR)/$(BINARY_LITE)

# Install lite binary to GOPATH/bin
install-lite: build-lite
	@echo "Installing $(BINARY_LITE) to $(shell go env GOPATH)/bin..."
	@cp $(BUILD_DIR)/$(BINARY_LITE) $(shell go env GOPATH)/bin/

# Create release archives
release: build-lite-all
	@echo "Creating release archives..."
	@mkdir -p $(DIST_DIR)/release
	@for f in $(DIST_DIR)/$(BINARY_LITE)-*; do \
		base=$$(basename $$f); \
		if [[ $$f == *.exe ]]; then \
			zip -j $(DIST_DIR)/release/$$base.zip $$f; \
		else \
			tar -czf $(DIST_DIR)/release/$$base.tar.gz -C $(DIST_DIR) $$base; \
		fi \
	done
	@echo "Release archives created in $(DIST_DIR)/release/"

# Show help
help:
	@echo "ACMG-AMP MCP Server Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build           Build the full server (requires PostgreSQL/Redis)"
	@echo "  build-lite      Build the lightweight server (no external databases)"
	@echo "  build-lite-all  Cross-compile lite server for all platforms"
	@echo ""
	@echo "Docker Targets:"
	@echo "  docker          Build Docker image for full server"
	@echo "  docker-lite     Build Docker image for lite server"
	@echo ""
	@echo "Test Targets:"
	@echo "  test            Run all tests"
	@echo "  test-coverage   Run tests with coverage report"
	@echo "  test-lite       Run tests for lightweight components only"
	@echo ""
	@echo "Other Targets:"
	@echo "  deps            Download and tidy dependencies"
	@echo "  lint            Run golangci-lint"
	@echo "  clean           Remove build artifacts"
	@echo "  install-lite    Install lite binary to GOPATH/bin"
	@echo "  run-lite        Build and run lite server locally"
	@echo "  release         Create release archives for all platforms"
	@echo "  help            Show this help"
	@echo ""
	@echo "Environment Variables:"
	@echo "  VERSION         Version string (default: git describe)"
	@echo "  DOCKER_TAG      Docker image tag (default: VERSION)"
