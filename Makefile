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

# Go variables
GOCMD=go
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

.PHONY: help build install clean test lint fmt vet deps tidy run sync fix release testdata demo

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
	@which golangci-lint > /dev/null || (echo "$(RED)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)" && exit 1)
	golangci-lint run
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

# Sync examples:
#   make sync                                    # Sync all providers (dry-run)
#   make sync PROVIDER=openai                    # Sync specific provider (dry-run)
#   make sync OUTPUT=./custom-dir                # Sync to custom directory (dry-run)
#   make sync PROVIDER=groq OUTPUT=./models      # Sync specific provider to custom dir (dry-run)
sync: ## Run sync command with dry-run (use PROVIDER=name OUTPUT=dir for options)
	@echo "$(BLUE)Running sync command (dry-run)...$(NC)"
	$(GOCMD) run $(MAIN_PATH) sync --dry-run $(if $(PROVIDER),--provider $(PROVIDER),) $(if $(OUTPUT),--output $(OUTPUT),)

list-models: ## List all models in catalog
	@echo "$(BLUE)Listing all models...$(NC)"
	$(GOCMD) run $(MAIN_PATH) list models

list-providers: ## List all providers
	@echo "$(BLUE)Listing all providers...$(NC)"
	$(GOCMD) run $(MAIN_PATH) providers

list-authors: ## List all authors
	@echo "$(BLUE)Listing all authors...$(NC)"
	$(GOCMD) run $(MAIN_PATH) authors

fix: ## Fix code formatting, imports, and dependencies
	@echo "$(BLUE)Fixing code...$(NC)"
	@echo "$(YELLOW)Formatting code...$(NC)"
	$(GOFMT) ./...
	@echo "$(YELLOW)Tidying modules...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)Code fixes complete$(NC)"

release: clean fix lint test build ## Prepare for release
	@echo "$(GREEN)Release build complete$(NC)"

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

# Validation targets
validate: ## Validate provider configurations
	@echo "$(BLUE)Validating provider configurations...$(NC)"
	$(GOCMD) run $(MAIN_PATH) validate

check-apis: ## Check API connectivity for all providers
	@echo "$(BLUE)Checking API connectivity...$(NC)"
	@echo "$(YELLOW)Testing OpenAI...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch --provider openai | head -5 || echo "$(RED)OpenAI: Failed$(NC)"
	@echo "$(YELLOW)Testing Anthropic...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch --provider anthropic | head -5 || echo "$(RED)Anthropic: Failed$(NC)"
	@echo "$(YELLOW)Testing Groq...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch --provider groq | head -5 || echo "$(RED)Groq: Failed$(NC)"
	@echo "$(YELLOW)Testing Google AI Studio...$(NC)"
	@$(GOCMD) run $(MAIN_PATH) fetch --provider google-ai-studio | head -5 || echo "$(RED)Google AI Studio: Failed$(NC)"

# Testdata management targets
# Examples:
#   make testdata                     # Show help and list all testdata
#   make testdata PROVIDER=groq       # List testdata for specific provider
#   make testdata-verify              # Verify all testdata files are valid
#   make testdata-update              # Update all provider testdata (requires API keys)
testdata: ## Manage testdata (use PROVIDER=name to specify provider)
	@echo "$(BLUE)Managing testdata...$(NC)"
	$(GOCMD) run $(MAIN_PATH) testdata $(if $(PROVIDER),--provider $(PROVIDER),) --verbose

testdata-verify: ## Verify all testdata files are valid
	@echo "$(BLUE)Verifying testdata files...$(NC)"
	$(GOCMD) run $(MAIN_PATH) testdata --verify --verbose

testdata-update: ## Update testdata for all providers (requires API keys)
	@echo "$(BLUE)Updating testdata for all providers...$(NC)"
	@echo "$(YELLOW)This will make actual API calls and update testdata files$(NC)"
	$(GOCMD) run $(MAIN_PATH) testdata --update --verbose

# Documentation
docs: ## Generate documentation
	@echo "$(BLUE)Generating documentation...$(NC)"
	@mkdir -p docs
	$(GOCMD) run $(MAIN_PATH) --help > docs/help.txt
	$(GOCMD) run $(MAIN_PATH) sync --help > docs/sync-help.txt
	@echo "$(GREEN)Documentation generated in docs/$(NC)"

# Demo
demo: ## Generate VHS demo video
	@echo "$(BLUE)Generating demo video...$(NC)"
	@which vhs > /dev/null || (echo "$(RED)VHS not found. Install with: go install github.com/charmbracelet/vhs@latest$(NC)" && exit 1)
	@echo "$(YELLOW)Recording terminal demo...$(NC)"
	vhs scripts/demo.tape
	@echo "$(GREEN)Demo video generated: scripts/demo.svg$(NC)"
	@echo "$(YELLOW)You can open scripts/demo.svg in your browser to view the demo$(NC)"
