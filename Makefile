# Starmap Makefile
# AI Model Catalog CLI

# Variables
BINARY_NAME=starmap
MAIN_PATH=./cmd/starmap/main.go
BUILD_DIR=./bin
VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

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

.PHONY: help build install clean test lint fmt vet deps tidy run update fix release release-snapshot release-tag release-local testdata demo godoc

# Default target
all: clean fix lint test build

help: ## Display this help message
	@echo "$(BLUE)Starmap - AI Model Catalog CLI$(NC)"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary to current directory
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Built $(BINARY_NAME) in current directory$(NC)"

install: ## Install the binary to GOPATH/bin
	@echo "$(BLUE)Installing $(BINARY_NAME)...$(NC)"
	$(GOCMD) install $(LDFLAGS) ./cmd/starmap
	@echo "$(GREEN)Installed $(BINARY_NAME) to $(shell go env GOPATH)/bin$(NC)"
	@echo "$(YELLOW)Make sure $(shell go env GOPATH)/bin is in your PATH$(NC)"

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning...$(NC)"
	$(GOCLEAN)
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)Cleaned build artifacts$(NC)"

test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	$(GOTEST) -v ./...


test-coverage: ## Run tests with coverage
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

lint: ## Run linter and static analysis tools
	@echo "$(BLUE)Running static analysis...$(NC)"
	@echo "$(YELLOW)Running go vet...$(NC)"
	$(GOVET) ./...
	@echo "$(YELLOW)Running golangci-lint...$(NC)"
	@$(RUN_PREFIX) which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)" && exit 1)
	$(RUN_PREFIX) golangci-lint run
	@echo "$(GREEN)Static analysis complete$(NC)"

fmt: ## Format Go code
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GOVET) ./...

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

fix: ## Fix code formatting, imports, and dependencies
	@echo "$(BLUE)Fixing code...$(NC)"
	@echo "$(YELLOW)Formatting code...$(NC)"
	$(GOFMT) ./...
	@echo "$(YELLOW)Tidying modules...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)Code fixes complete$(NC)"

release: clean fix lint test ## Prepare for release
	@echo "$(GREEN)Ready for release. Run 'make release-tag VERSION=x.y.z' to create and push a release tag$(NC)"

release-snapshot: ## Create a snapshot release with goreleaser (no tag required)
	@echo "$(BLUE)Creating snapshot release with goreleaser...$(NC)"
	@which goreleaser > /dev/null || (echo "$(RED)goreleaser not found. Install from https://goreleaser.com$(NC)" && exit 1)
	goreleaser release --snapshot --clean
	@echo "$(GREEN)Snapshot release created in ./dist/$(NC)"

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

# Validation targets
validate: ## Validate provider configurations
	@echo "$(BLUE)Validating provider configurations...$(NC)"
	$(GOCMD) run $(MAIN_PATH) validate

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
#   make testdata                     # Show help and list all testdata
#   make testdata PROVIDER=groq       # List testdata for specific provider
#   make testdata-verify              # Verify all testdata files are valid
#   make testdata-update              # Update all provider testdata (requires API keys)
testdata: ## Manage testdata (use PROVIDER=name to specify provider)
	@echo "$(BLUE)Managing testdata...$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate testdata $(if $(PROVIDER),--provider $(PROVIDER),) --verbose

testdata-verify: ## Verify all testdata files are valid
	@echo "$(BLUE)Verifying testdata files...$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate testdata --verify --verbose

testdata-update: ## Update testdata for all providers (requires API keys)
	@echo "$(BLUE)Updating testdata for all providers...$(NC)"
	@echo "$(YELLOW)This will make actual API calls and update testdata files$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate testdata --verbose

# Documentation
generate: ## Generate all documentation (Go docs and catalog docs)
	@echo "$(BLUE)Generating Go documentation...$(NC)"
	@$(RUN_PREFIX) which gomarkdoc > /dev/null || (echo "$(RED)gomarkdoc not found. Install with: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest$(NC)" && exit 1)
	$(GOCMD) generate ./...
	@echo "$(GREEN)Go documentation generation complete$(NC)"
	@echo "$(BLUE)Generating catalog documentation...$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate
	@echo "$(GREEN)Catalog documentation generated in docs/$(NC)"

docs: generate ## Alias for generate command

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

catalog-docs: ## Generate only catalog documentation
	@echo "$(BLUE)Generating catalog documentation...$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate docs
	@echo "$(GREEN)Catalog documentation generated in docs/$(NC)"

# Demo
demo: ## Generate VHS demo video
	@echo "$(BLUE)Generating demo video...$(NC)"
	@$(RUN_PREFIX) which vhs > /dev/null || (echo "$(RED)VHS not found. Install with: go install github.com/agentstation/vhs@latest$(NC)" && exit 1)
	@echo "$(YELLOW)Recording terminal demo...$(NC)"
	$(RUN_PREFIX) vhs scripts/demo.tape
	@echo "$(GREEN)Demo video generated: scripts/demo.svg$(NC)"
	@echo "$(YELLOW)You can open scripts/demo.svg in your browser to view the demo$(NC)"

# Site generation targets
site-generate: ## Generate static documentation site
	@echo "$(BLUE)Generating documentation site...$(NC)"
	$(GOCMD) run $(MAIN_PATH) generate site
	@echo "$(GREEN)Site generated in site/public$(NC)"

site-serve: ## Run Hugo development server
	@echo "$(BLUE)Starting Hugo development server...$(NC)"
	$(GOCMD) run $(MAIN_PATH) serve site

site-build: ## Build production site with Hugo
	@echo "$(BLUE)Building production site...$(NC)"
	@$(RUN_PREFIX) which hugo > /dev/null || (echo "$(RED)Hugo not found. Run 'devbox shell' or install with: brew install hugo$(NC)" && exit 1)
	$(RUN_PREFIX) hugo --source site --minify --gc
	@echo "$(GREEN)Production build in site/public$(NC)"

site-theme: ## Download Hugo theme
	@echo "$(BLUE)Downloading Hugo Book theme...$(NC)"
	@mkdir -p site
	cd site && git submodule add https://github.com/alex-shpak/hugo-book themes/hugo-book 2>/dev/null || true
	cd site && git submodule update --init --recursive
	@echo "$(GREEN)Theme installed$(NC)"

site-preview: site-theme site-generate site-serve ## Full site preview workflow

site-setup: ## Set up Hugo site structure
	@echo "$(BLUE)Setting up Hugo site structure...$(NC)"
	@mkdir -p site/{content,themes,static,layouts,assets}
	@mkdir -p site/static/{css,js,img}
	@mkdir -p site/layouts/{_default,partials,shortcodes}
	@if [ ! -L site/content ]; then cd site && ln -sf ../docs content; fi

site-test-pages: ## Test site with GitHub Pages URL locally
	@echo "$(BLUE)Testing site with GitHub Pages URL...$(NC)"
	@cd site && $(RUN_PREFIX) hugo serve --baseURL "https://agentstation.github.io/starmap/" --buildDrafts
	@echo "$(GREEN)Preview available at http://localhost:1313$(NC)"

site-test-custom: ## Test site with custom domain locally
	@echo "$(BLUE)Testing site with custom domain...$(NC)"
	@cd site && $(RUN_PREFIX) hugo serve --baseURL "https://starmap.agentstation.ai/" --buildDrafts
	@echo "$(GREEN)Preview available at http://localhost:1313$(NC)"

deploy-check: ## Check if site is ready for deployment
	@echo "$(BLUE)Checking deployment readiness...$(NC)"
	@test -L site/content || (echo "$(RED)Error: content symlink missing$(NC)" && exit 1)
	@test -d site/themes/hugo-book || (echo "$(RED)Error: theme missing. Run 'make site-theme'$(NC)" && exit 1)
	@test -f site/hugo.yaml || (echo "$(RED)Error: hugo.yaml missing$(NC)" && exit 1)
	@echo "$(GREEN)✓ Content symlink exists$(NC)"
	@echo "$(GREEN)✓ Theme installed$(NC)"
	@echo "$(GREEN)✓ Hugo config present$(NC)"
	@echo "$(GREEN)Site is ready for deployment!$(NC)"
	@echo ""
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Enable GitHub Pages: Settings → Pages → Source: GitHub Actions"
	@echo "  2. Push to master branch to trigger deployment"
	@echo "  3. Optional: Configure custom domain in Settings → Pages"
	@echo "$(GREEN)Site structure created$(NC)"
