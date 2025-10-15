# Starmap Makefile
# AI Model Catalog CLI

MAKEFLAGS += --no-print-directory

# Default target when running just 'make'
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=starmap
MAIN_PATH=./cmd/starmap/main.go
BUILD_DIR=./bin
VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_TIME) -X main.builtBy=makefile"

# Check for devbox and use it if available
HAS_DEVBOX := $(shell command -v devbox 2> /dev/null)
ifdef HAS_DEVBOX
	RUN_PREFIX := devbox run
else
	RUN_PREFIX :=
endif

# Go variables
ifdef HAS_DEVBOX
	GOCMD=$(RUN_PREFIX) go
else
	GOCMD=go
endif
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.PHONY: help build install uninstall clean test test-race test-integration test-all test-coverage lint fmt check fix vet deps tidy run update install-tools goreleaser-check release-snapshot-devbox ci-test release release-snapshot release-tag release-local testdata demo godoc version

# Default target  
all: clean fix check build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters

help: ## Display the list of targets and their descriptions
	@awk 'BEGIN {FS = ":.*##"; printf "\n\033[1mUsage:\033[0m\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } \
		/^###/ { printf "  \033[90m%s\033[0m\n", substr($$0, 4) }' $(MAKEFILE_LIST)

##@ Build & Install

build: ## Build the binary to current directory
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Built $(BINARY_NAME) in current directory$(NC)"

install: ## Install the binary to GOPATH/bin and shell completions
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	$(GOCMD) install $(LDFLAGS) ./cmd/starmap
	@echo "$(GREEN)Installed $(BINARY_NAME) to $(shell go env GOPATH)/bin$(NC)"
	@echo "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)"
	@echo ""
	@echo "$(BLUE)Installing shell completions...$(NC)"
	@if command -v starmap >/dev/null 2>&1; then \
		starmap install completion && echo "$(GREEN)✅ Completions installed for all shells$(NC)" || echo "$(YELLOW)⚠️  Some completions may have failed$(NC)"; \
	else \
		echo "$(YELLOW)starmap not found in PATH. You may need to restart your shell first.$(NC)"; \
		echo "$(YELLOW)Then run: starmap install completion$(NC)"; \
	fi

uninstall: ## Uninstall the binary from GOPATH/bin
	@echo "$(BLUE)Uninstalling $(BINARY_NAME)...$(NC)"
	@GOPATH_BIN=$$(go env GOPATH)/bin; \
	if [ -f "$$GOPATH_BIN/$(BINARY_NAME)" ]; then \
		rm -f "$$GOPATH_BIN/$(BINARY_NAME)"; \
		echo "$(GREEN)✅ Removed $(BINARY_NAME) from $$GOPATH_BIN$(NC)"; \
	else \
		echo "$(YELLOW)$(BINARY_NAME) not found in $$GOPATH_BIN$(NC)"; \
	fi
	@echo "$(BLUE)Also removing shell completions if installed...$(NC)"
	@$(MAKE) uninstall-completions

uninstall-completions: ## Remove all starmap shell completions
	@echo "$(BLUE)Removing starmap shell completions...$(NC)"
	@# Homebrew completions
	@if [ -f "/opt/homebrew/etc/bash_completion.d/starmap" ]; then \
		rm -f "/opt/homebrew/etc/bash_completion.d/starmap"; \
		echo "$(GREEN)✅ Removed Homebrew bash completion$(NC)"; \
	fi
	@if [ -f "/opt/homebrew/share/zsh/site-functions/_starmap" ]; then \
		rm -f "/opt/homebrew/share/zsh/site-functions/_starmap"; \
		echo "$(GREEN)✅ Removed Homebrew zsh completion$(NC)"; \
	fi
	@if [ -f "/opt/homebrew/share/fish/vendor_completions.d/starmap.fish" ]; then \
		rm -f "/opt/homebrew/share/fish/vendor_completions.d/starmap.fish"; \
		echo "$(GREEN)✅ Removed Homebrew fish completion$(NC)"; \
	fi
	@# Linux standard locations
	@if [ -f "/usr/local/etc/bash_completion.d/starmap" ]; then \
		rm -f "/usr/local/etc/bash_completion.d/starmap"; \
		echo "$(GREEN)✅ Removed Linux bash completion$(NC)"; \
	fi
	@if [ -f "/usr/local/share/zsh/site-functions/_starmap" ]; then \
		rm -f "/usr/local/share/zsh/site-functions/_starmap"; \
		echo "$(GREEN)✅ Removed Linux zsh completion$(NC)"; \
	fi
	@if [ -f "/usr/local/share/fish/vendor_completions.d/starmap.fish" ]; then \
		rm -f "/usr/local/share/fish/vendor_completions.d/starmap.fish"; \
		echo "$(GREEN)✅ Removed Linux fish completion$(NC)"; \
	fi
	@echo "$(GREEN)Shell completions cleanup complete$(NC)"
	@if command -v starmap >/dev/null 2>&1; then \
		starmap uninstall completion 2>/dev/null || echo "$(YELLOW)⚠️  Some completions may not have been removed$(NC)"; \
	else \
		echo "$(YELLOW)Note: starmap not found. Use 'starmap uninstall completion' to remove completions$(NC)"; \
	fi

##@ Development

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning...$(NC)"
	$(GOCLEAN)
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)Cleaned build artifacts$(NC)"

##@ Testing & Coverage

test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	$(GOTEST) -v ./...


test-coverage: ## Run tests with coverage
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

test-race: ## Run tests with race detector
	@echo "$(BLUE)Running tests with race detector...$(NC)"
	$(GOTEST) -race -v ./...

test-integration: ## Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	$(GOTEST) -tags=integration -v ./...

test-all: test test-race test-integration ## Run all tests
	@echo "$(GREEN)All tests completed!$(NC)"

lint: ## Run golangci-lint only
	@echo "$(BLUE)Running golangci-lint...$(NC)"
	@$(RUN_PREFIX) which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)" && exit 1)
	$(RUN_PREFIX) golangci-lint run
	@echo "$(GREEN)Linting complete$(NC)"

fmt: ## Format Go code with gofmt only
	@echo "$(BLUE)Formatting with gofmt...$(NC)"
	$(GOFMT) ./...
	@echo "$(GREEN)Formatting complete$(NC)"

check: ## Run all checks: vet + lint + test (no fixes)
	@echo "$(BLUE)Running checks: go vet & golangci-lint & tests...$(NC)"
	@$(RUN_PREFIX) which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)" && exit 1)
	$(GOVET) ./... && $(RUN_PREFIX) golangci-lint run && $(GOTEST) ./...
	@echo "$(GREEN)All checks passed$(NC)"

fix: ## Auto-fix everything: format, imports, lint issues, dependencies
	@echo "$(BLUE)Auto-fixing: format, imports, lints, dependencies...$(NC)"
	@$(RUN_PREFIX) which goimports > /dev/null || echo "$(YELLOW)Warning: goimports not found, skipping import fixes$(NC)"
	@$(RUN_PREFIX) which golangci-lint > /dev/null || echo "$(YELLOW)Warning: golangci-lint not found, skipping lint fixes$(NC)"
	$(GOFMT) ./... && ($(RUN_PREFIX) goimports -w -local github.com/agentstation/starmap . 2>/dev/null || true) && ($(RUN_PREFIX) golangci-lint run --fix 2>/dev/null || true) && $(GOMOD) tidy
	@echo "$(GREEN)Auto-fix complete$(NC)"

vet: ## Run go vet only
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GOVET) ./...
	@echo "$(GREEN)Vet complete$(NC)"

##@ Tooling

install-tools: ## Install development tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@$(RUN_PREFIX) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(RUN_PREFIX) go install golang.org/x/tools/cmd/goimports@latest
	@$(RUN_PREFIX) go install golang.org/x/vuln/cmd/govulncheck@latest
	@$(RUN_PREFIX) go install honnef.co/go/tools/cmd/staticcheck@latest
	@$(RUN_PREFIX) go install github.com/tetafro/godot/cmd/godot@latest
	@$(RUN_PREFIX) go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
	@echo "$(GREEN)Tools installed successfully$(NC)"

deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GOGET) -d ./...

tidy: ## Tidy go modules
	@echo "$(BLUE)Tidying modules...$(NC)"
	$(GOMOD) tidy

run: ## Run the application locally
	@echo "$(BLUE)Running $(BINARY_NAME)...$(NC)"
	$(GOCMD) run $(MAIN_PATH)

run-help: ## Show application help
	@echo "$(BLUE)Showing $(BINARY_NAME) help...$(NC)"
	$(GOCMD) run $(MAIN_PATH) --help

# Update examples:
#   make update                                   # Update all providers (dry-run)
#   make update PROVIDER=openai                  # Update specific provider (dry-run)
#   make update OUTPUT=./custom-dir              # Update to custom directory (dry-run)
#   make update PROVIDER=groq OUTPUT=./models    # Update specific provider to custom dir (dry-run)
update: ## Run update command with dry-run (use PROVIDER=name OUTPUT=dir for options)
	@echo "$(BLUE)Running update command (dry-run)...$(NC)"
	$(GOCMD) run $(MAIN_PATH) update --dry-run $(if $(PROVIDER),--provider $(PROVIDER),) $(if $(OUTPUT),--output $(OUTPUT),)

list-models: ## List all models in catalog
	@echo "$(BLUE)Listing all models...$(NC)"
	$(GOCMD) run $(MAIN_PATH) list models

list-providers: ## List all providers
	@echo "$(BLUE)Listing all providers...$(NC)"
	$(GOCMD) run $(MAIN_PATH) list providers

list-authors: ## List all authors
	@echo "$(BLUE)Listing all authors...$(NC)"
	$(GOCMD) run $(MAIN_PATH) list authors


##@ Release & CI Alignment

goreleaser-check: ## Validate GoReleaser config (CI-friendly)
	@echo "$(BLUE)Validating GoReleaser configuration...$(NC)"
	@$(RUN_PREFIX) which goreleaser > /dev/null || (echo "$(RED)goreleaser not found. Install from https://goreleaser.com$(NC)" && exit 1)
	@$(RUN_PREFIX) goreleaser check
	@echo "$(GREEN)✅ GoReleaser config is valid$(NC)"

release-snapshot-devbox: ## Create snapshot release using devbox tools
	@echo "$(BLUE)Creating snapshot release with devbox...$(NC)"
	@$(RUN_PREFIX) goreleaser release --snapshot --clean
	@echo "$(GREEN)Snapshot release created in ./dist/$(NC)"

ci-test: ## Run CI-equivalent tests locally
	@echo "$(BLUE)Running CI-equivalent test suite...$(NC)"
	@$(MAKE) clean fix check test-race
	@echo "$(GREEN)✅ All CI tests passed$(NC)"

release: clean fix check ## Prepare for release (use: make release VERSION=x.y.z)
	@if [ -z "$(VERSION)" ]; then \
		echo "$(GREEN)Ready for release. Run 'make release VERSION=x.y.z' to create and push a release tag$(NC)"; \
	else \
		$(MAKE) release-full VERSION=$(VERSION); \
	fi

release-full: ## Complete release workflow: prepare, tag, and trigger GitHub Actions
	@if [ -z "$(VERSION)" ]; then \
		echo "$(RED)VERSION is required. Usage: make release VERSION=0.1.0$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Starting full release workflow for v$(VERSION)...$(NC)"
	@echo "$(YELLOW)Step 1/5: Checking working directory...$(NC)"
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(RED)Error: Working directory is not clean. Please commit or stash changes.$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Step 2/5: Running pre-release checks...$(NC)"
	@$(MAKE) clean fix check > /dev/null
	@echo "$(YELLOW)Step 3/5: Testing CLI features...$(NC)"
	@echo "  Testing version command..."
	@$(GOCMD) run $(MAIN_PATH) version > /dev/null
	@echo "  Testing completion generation..."
	@$(GOCMD) run $(MAIN_PATH) completion bash > /dev/null
	@echo "  Testing man page generation..."
	@$(GOCMD) run $(MAIN_PATH) man > /dev/null
	@echo "$(YELLOW)Step 4/5: Creating and pushing tag...$(NC)"
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)
	@echo "$(YELLOW)Step 5/5: Release triggered!$(NC)"
	@echo "$(GREEN)✅ Release v$(VERSION) tagged and pushed!$(NC)"
	@echo "$(GREEN)🚀 GitHub Actions will now build and publish the release$(NC)"
	@echo "$(BLUE)Monitor progress at: https://github.com/agentstation/starmap/actions$(NC)"

release-v0.0.1: ## Quick release for v0.0.1 (first release)
	@$(MAKE) release-full VERSION=0.0.1

release-check: ## Check if ready for release (CI-friendly)
	@echo "$(BLUE)Checking release readiness...$(NC)"
	@echo "$(YELLOW)Checking working directory...$(NC)"
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(RED)❌ Working directory is not clean$(NC)"; \
		exit 1; \
	else \
		echo "$(GREEN)✅ Working directory is clean$(NC)"; \
	fi
	@echo "$(YELLOW)Running tests...$(NC)"
	@$(GOTEST) ./... > /dev/null && echo "$(GREEN)✅ All tests pass$(NC)" || (echo "$(RED)❌ Tests failed$(NC)" && exit 1)
	@echo "$(YELLOW)Running linter...$(NC)"
	@$(GOVET) ./... > /dev/null && echo "$(GREEN)✅ No vet issues$(NC)" || (echo "$(RED)❌ Vet issues found$(NC)" && exit 1)
	@echo "$(YELLOW)Testing CLI features...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) version > /dev/null && echo "$(GREEN)✅ Version command works$(NC)" || (echo "$(RED)❌ Version command failed$(NC)" && exit 1)
	@$(GOCMD) run $(MAIN_PATH) completion bash > /dev/null && echo "$(GREEN)✅ Completion generation works$(NC)" || (echo "$(RED)❌ Completion generation failed$(NC)" && exit 1)
	@$(GOCMD) run $(MAIN_PATH) man > /dev/null && echo "$(GREEN)✅ Man page generation works$(NC)" || (echo "$(RED)❌ Man page generation failed$(NC)" && exit 1)
	@echo "$(YELLOW)Checking GoReleaser config...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)❌ goreleaser not found$(NC)" && exit 1)
	@goreleaser check > /dev/null && echo "$(GREEN)✅ GoReleaser config is valid$(NC)" || (echo "$(RED)❌ GoReleaser config invalid$(NC)" && exit 1)
	@echo "$(GREEN)🎉 Ready for release!$(NC)"

release-snapshot: ## Create a snapshot release with goreleaser (no tag required)
	@echo "$(BLUE)Creating snapshot release with goreleaser...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)goreleaser not found. Install from https://goreleaser.com$(NC)" && exit 1)
	goreleaser release --snapshot --clean
	@echo "$(GREEN)Snapshot release created in ./dist/$(NC)"
	@echo "$(YELLOW)Test the binaries in ./dist/ before creating a real release$(NC)"

release-tag: ## Create and push a release tag (use: make release-tag VERSION=0.1.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "$(RED)VERSION is required. Usage: make release-tag VERSION=0.1.0$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Creating release tag v$(VERSION)...$(NC)"
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)
	@echo "$(GREEN)Tag v$(VERSION) created and pushed. GitHub Actions will handle the release.$(NC)"

release-local: ## Build release locally with goreleaser (requires tag)
	@echo "$(BLUE)Building release locally with goreleaser...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)goreleaser not found. Install from https://goreleaser.com$(NC)" && exit 1)
	goreleaser release --skip=publish --clean
	@echo "$(GREEN)Local release created in ./dist/$(NC)"

# Cross-compilation targets
build-linux: ## Build for Linux
	@echo "$(BLUE)Building for Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "$(GREEN)Built $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

build-windows: ## Build for Windows
	@echo "$(BLUE)Building for Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)Built $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe$(NC)"

build-darwin: ## Build for macOS
	@echo "$(BLUE)Building for macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "$(GREEN)Built $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 and $(BINARY_NAME)-darwin-arm64$(NC)"

build-all: build-linux build-windows build-darwin ## Build for all platforms

# Docker targets (if needed in the future)
docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) .

# Environment setup
setup-env: ## Setup development environment
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@cp .env.example .env 2>/dev/null || echo "$(YELLOW)No .env.example found, using existing .env$(NC)"
	@echo "$(GREEN)Environment setup complete$(NC)"
	@echo "$(YELLOW)Don't forget to configure your API keys in .env$(NC)"

# Version info
version: ## Show version information
	@echo "$(BLUE)Version Information:$(NC)"
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Built:   $(BUILD_TIME)"

# Catalog update targets
update-catalog: ## Update embedded catalog with latest API data (requires API keys)
	@echo "$(BLUE)Updating embedded catalog...$(NC)"
	@echo "$(YELLOW)This will fetch latest models from all configured provider APIs$(NC)"
	$(GOCMD) run $(MAIN_PATH) update --output ./internal/embedded/catalog --force -y
	@echo "$(GREEN)Embedded catalog updated successfully!$(NC)"

update-catalog-provider: ## Update specific provider in embedded catalog (use PROVIDER=name)
	@if [ -z "$(PROVIDER)" ]; then \
		echo "$(RED)Error: PROVIDER not specified$(NC)"; \
		echo "$(YELLOW)Usage: make update-catalog-provider PROVIDER=openai$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Updating provider $(PROVIDER) in embedded catalog...$(NC)"
	$(GOCMD) run $(MAIN_PATH) update --provider $(PROVIDER) --output ./internal/embedded/catalog --force -y
	@echo "$(GREEN)Provider $(PROVIDER) updated successfully!$(NC)"

# Enhanced embed command with automatic authentication
embed: ## Update embedded catalog with automatic Google Cloud auth
	@echo "$(BLUE)Checking Google Cloud authentication...$(NC)"
	@if $(GOCMD) run $(MAIN_PATH) auth gcloud --check 2>/dev/null; then \
		echo "$(GREEN)✅ Google Cloud authenticated$(NC)"; \
	else \
		echo "$(YELLOW)Google Cloud authentication required$(NC)"; \
		$(GOCMD) run $(MAIN_PATH) auth gcloud || exit 1; \
	fi
	@echo "$(BLUE)Validating catalog structure...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) validate || exit 1
	@echo "$(BLUE)Checking provider authentication...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) auth status
	@echo "$(BLUE)Updating embedded sources...$(NC)"
	@curl -s https://models.dev/api.json -o internal/embedded/sources/models.dev/api.json || echo "$(YELLOW)Warning: Could not update models.dev api.json$(NC)"
	@echo "$(BLUE)Updating embedded catalog...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) update --output ./internal/embedded/catalog --force -y
	@echo "$(GREEN)✅ Embedded catalog and sources updated successfully!$(NC)"

embed-provider: ## Update specific provider with auth check (use PROVIDER=name)
	@if [ -z "$(PROVIDER)" ]; then \
		echo "$(RED)Error: PROVIDER not specified$(NC)"; \
		echo "$(YELLOW)Usage: make embed-provider PROVIDER=google-vertex$(NC)"; \
		exit 1; \
	fi
	@if [ "$(PROVIDER)" = "google-vertex" ] || [ "$(PROVIDER)" = "google-ai-studio" ]; then \
		echo "$(BLUE)Checking Google Cloud authentication...$(NC)"; \
		if $(GOCMD) run $(MAIN_PATH) auth gcloud --check 2>/dev/null; then \
			echo "$(GREEN)✅ Google Cloud authenticated$(NC)"; \
		else \
			echo "$(YELLOW)Google Cloud authentication required$(NC)"; \
			$(GOCMD) run $(MAIN_PATH) auth gcloud || exit 1; \
		fi; \
	fi
	@echo "$(BLUE)Updating provider $(PROVIDER) in embedded catalog...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) update --provider $(PROVIDER) --output ./internal/embedded/catalog --force -y
	@echo "$(GREEN)✅ Provider $(PROVIDER) updated successfully!$(NC)"

# Validation targets
validate: ## Validate entire embedded catalog structure
	@echo "$(BLUE)Validating catalog structure...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) validate

validate-providers: ## Validate providers.yaml only
	@$(GOCMD) run $(MAIN_PATH) validate providers

validate-authors: ## Validate authors.yaml only
	@$(GOCMD) run $(MAIN_PATH) validate authors

validate-models: ## Validate model definitions
	@$(GOCMD) run $(MAIN_PATH) validate models

# Authentication targets
auth: ## Check authentication status for all providers
	@$(GOCMD) run $(MAIN_PATH) auth

auth-status: ## Show authentication status (same as auth)
	@$(GOCMD) run $(MAIN_PATH) auth status

auth-verify: ## Verify credentials work with test API calls
	@$(GOCMD) run $(MAIN_PATH) auth verify

auth-gcloud: ## Authenticate with Google Cloud
	@$(GOCMD) run $(MAIN_PATH) auth gcloud

check-apis: ## Check API connectivity for all providers
	@echo "$(BLUE)Checking API connectivity...$(NC)"
	@echo "$(YELLOW)Testing OpenAI...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch models --provider openai | head -5 || echo "$(RED)OpenAI: Failed$(NC)"
	@echo "$(YELLOW)Testing Anthropic...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch models --provider anthropic | head -5 || echo "$(RED)Anthropic: Failed$(NC)"
	@echo "$(YELLOW)Testing Groq...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch models --provider groq | head -5 || echo "$(RED)Groq: Failed$(NC)"
	@echo "$(YELLOW)Testing Google AI Studio...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch models --provider google-ai-studio | head -5 || echo "$(RED)Google AI Studio: Failed$(NC)"

# Testdata management targets
# Examples:
#   make testdata              # Update all provider testdata (requires API keys)
#   make testdata PROVIDER=groq  # Update specific provider testdata
testdata: ## Update testdata for all providers (use PROVIDER=name for specific provider)
	@echo "$(BLUE)Updating testdata for $(if $(PROVIDER),$(PROVIDER),all providers)...$(NC)"
	@echo "$(YELLOW)This will make actual API calls and update testdata files$(NC)"
	@if [ -n "$(PROVIDER)" ]; then \
		$(GOTEST) ./internal/sources/providers/$(PROVIDER) -update -v; \
	else \
		for dir in internal/sources/providers/*/; do \
			provider=$$(basename $$dir); \
			if [ -f "$$dir/client_test.go" ]; then \
				echo "$(BLUE)Updating $$provider testdata...$(NC)"; \
				$(GOTEST) ./internal/sources/providers/$$provider -update || true; \
			fi; \
		done; \
	fi

# Documentation
openapi: ## Generate OpenAPI 3.1 documentation (embedded in binary)
	@echo "$(BLUE)Generating OpenAPI 3.1 documentation...$(NC)"
	@$(RUN_PREFIX) which swag > /dev/null || (echo "$(RED)swag not found. Run 'devbox shell' to enter the development environment$(NC)" && exit 1)
	@echo "$(YELLOW)Step 1/3: Generating OpenAPI 3.1 with swag v2...$(NC)"
	@$(RUN_PREFIX) swag init -g internal/server/docs.go -o internal/embedded/openapi --parseDependency --parseInternal --v3.1
	@echo "$(YELLOW)Step 2/3: Renaming generated files...$(NC)"
	@mv internal/embedded/openapi/swagger.json internal/embedded/openapi/openapi.json
	@mv internal/embedded/openapi/swagger.yaml internal/embedded/openapi/openapi.yaml
	@rm -f internal/embedded/openapi/docs.go
	@echo "$(YELLOW)Step 3/3: Verifying embedded specs...$(NC)"
	@test -f internal/embedded/openapi/openapi.json || (echo "$(RED)Error: openapi.json not found$(NC)" && exit 1)
	@test -f internal/embedded/openapi/openapi.yaml || (echo "$(RED)Error: openapi.yaml not found$(NC)" && exit 1)
	@echo "$(GREEN)OpenAPI 3.1 specs generated and ready for embedding$(NC)"
	@echo "$(GREEN)  - internal/embedded/openapi/openapi.json$(NC)"
	@echo "$(GREEN)  - internal/embedded/openapi/openapi.yaml$(NC)"
	@echo "$(BLUE)Specs will be embedded in binary via //go:embed$(NC)"

generate: openapi ## Generate all documentation (Go docs and OpenAPI)
	@echo "$(BLUE)Generating Go documentation...$(NC)"
	@$(RUN_PREFIX) which gomarkdoc > /dev/null || (echo "$(RED)gomarkdoc not found. Install with: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest$(NC)" && exit 1)
	$(GOCMD) generate ./...
	@echo "$(GREEN)All documentation generation complete$(NC)"

godoc: ## Generate only Go documentation using go generate
	@echo "$(BLUE)Generating Go documentation...$(NC)"
	@$(RUN_PREFIX) which gomarkdoc > /dev/null || (echo "$(RED)gomarkdoc not found. Install with: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest$(NC)" && exit 1)
	$(GOCMD) generate ./...
	@echo "$(GREEN)Go documentation generation complete$(NC)"

docs-check: ## Check if documentation is up to date (for CI)
	@echo "$(BLUE)Checking if documentation is up to date...$(NC)"
	@$(RUN_PREFIX) which gomarkdoc > /dev/null || (echo "$(RED)gomarkdoc not found. Install with: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest$(NC)" && exit 1)
	@for pkg in $$(find ./pkg ./internal -name "generate.go" -exec dirname {} \;); do \
		echo "Checking $$pkg..."; \
		cd $$pkg && gomarkdoc -c -e -o README.md . || exit 1; \
		cd - > /dev/null; \
	done
	@echo "$(GREEN)All documentation is up to date$(NC)"

# Demo
demo: ## Generate VHS demo video
	@echo "$(BLUE)Generating demo video...$(NC)"
	@$(RUN_PREFIX) which vhs > /dev/null || (echo "$(RED)VHS not found. Install with: go install github.com/agentstation/vhs@latest$(NC)" && exit 1)
	@echo "$(YELLOW)Recording terminal demo...$(NC)"
	$(RUN_PREFIX) vhs scripts/demo.tape
	@echo "$(GREEN)Demo video generated: scripts/demo.svg$(NC)"
	@echo "$(YELLOW)You can open scripts/demo.svg in your browser to view the demo$(NC)"
