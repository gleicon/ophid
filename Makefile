.PHONY: build run clean test install help release release-snapshot release-check

BINARY_NAME=ophid
BUILD_DIR=build
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Release variables
RELEASE_VERSION?=
RELEASE_MESSAGE?=

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ophid

run: build ## Build and run
	@./$(BUILD_DIR)/$(BINARY_NAME)

install: build ## Install to /usr/local/bin
	@echo "Installing to /usr/local/bin..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

release-check: ## Check if ready for release
	@echo "Checking release prerequisites..."
	@command -v goreleaser >/dev/null 2>&1 || { echo "Error: goreleaser is not installed. Install with: brew install goreleaser"; exit 1; }
	@test -n "$(RELEASE_VERSION)" || { echo "Error: RELEASE_VERSION is required. Usage: make release RELEASE_VERSION=v0.1.3 RELEASE_MESSAGE='Release notes'"; exit 1; }
	@test -n "$(RELEASE_MESSAGE)" || { echo "Error: RELEASE_MESSAGE is required. Usage: make release RELEASE_VERSION=v0.1.3 RELEASE_MESSAGE='Release notes'"; exit 1; }
	@echo "✓ Prerequisites OK"

release: release-check ## Create and push a new release (requires RELEASE_VERSION and RELEASE_MESSAGE)
	@echo "Creating release $(RELEASE_VERSION)..."
	@echo ""
	@echo "Checking working directory status..."
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "⚠ You have uncommitted changes:"; \
		git status --short; \
		echo ""; \
		read -p "Commit all changes before release? [y/N] " -n 1 -r; \
		echo; \
		if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
			echo "Committing all changes..."; \
			git add -A; \
			git commit -m "chore: prepare for release $(RELEASE_VERSION)"; \
		else \
			echo "Error: Please commit your changes before creating a release"; \
			exit 1; \
		fi \
	else \
		echo "✓ Working directory clean"; \
	fi
	@echo ""
	@echo "Pushing to main branch..."
	@git push origin main
	@echo ""
	@echo "Creating tag $(RELEASE_VERSION)..."
	@git tag -a $(RELEASE_VERSION) -m "$(RELEASE_MESSAGE)"
	@echo "Pushing tag $(RELEASE_VERSION)..."
	@git push origin $(RELEASE_VERSION)
	@echo ""
	@echo "Running goreleaser..."
	@goreleaser release --clean
	@echo ""
	@echo "✓ Release $(RELEASE_VERSION) complete!"
	@echo "✓ View release at: https://github.com/gleicon/ophid/releases/tag/$(RELEASE_VERSION)"

release-snapshot: ## Test release build locally without publishing
	@echo "Building snapshot release..."
	@command -v goreleaser >/dev/null 2>&1 || { echo "Error: goreleaser is not installed. Install with: brew install goreleaser"; exit 1; }
	@goreleaser release --snapshot --clean
	@echo "✓ Snapshot build complete - check dist/ directory"

.DEFAULT_GOAL := help
