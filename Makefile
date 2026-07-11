# Build and release commands for the Python stdio MCP package.
UV ?= uv
DOCKER_IMAGE ?= acmg-classifier
DOCKER_TAG ?= latest

.PHONY: all build clean docker help lint package run test test-coverage

all: test build

# Build source and wheel distributions.
build:
	$(UV) build

# Verify the package using its focused test suite.
test:
	$(UV) run pytest tests/packaging

test-coverage:
	$(UV) run pytest --cov=acmg_classifier tests/packaging

lint:
	$(UV) run ruff check src tests/packaging

# Build the stdio-only MCP container. Run it with `docker run -i`.
docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Start the local MCP server over standard input/output.
run:
	$(UV) run acmg-mcp

clean:
	rm -rf dist build .pytest_cache

help:
	@echo "ACMG classifier Python package"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "  build          Build source and wheel distributions"
	@echo "  test           Run package contract tests"
	@echo "  test-coverage  Run package contract tests with coverage"
	@echo "  lint           Run Ruff on package and package tests"
	@echo "  docker         Build the stdio MCP container"
	@echo "  run            Start acmg-mcp over standard input/output"
	@echo "  clean          Remove local build artifacts"
