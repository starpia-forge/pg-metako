# Purpose    : Comprehensive build automation for pg-metako PostgreSQL replication manager
# Context    : Development workflow automation following Go and Docker best practices
# Constraints: Must support build, test, Docker operations, and cross-platform compatibility

# Project information
PROJECT_NAME := pg-metako
VERSION := 1.0.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build parameters
BINARY_NAME := pg-metako
BINARY_PATH := ./bin/$(BINARY_NAME)
MAIN_PATH := ./cmd/pg-metako
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -X main.gitBranch=$(GIT_BRANCH) -w -s"

# Docker parameters
DOCKER_IMAGE := $(PROJECT_NAME)
DOCKER_TAG := $(VERSION)
DOCKER_LATEST := $(PROJECT_NAME):latest
DOCKER_VERSIONED := $(PROJECT_NAME):$(DOCKER_TAG)

# Test parameters
TEST_PACKAGES := ./...
TEST_TIMEOUT := 300s
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

# Directories
BIN_DIR := bin
BUILD_DIR := build
DIST_DIR := dist

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
PURPLE := \033[0;35m
CYAN := \033[0;36m
NC := \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

# Phony targets
.PHONY: help build build-linux build-windows build-darwin build-all clean test test-verbose test-coverage test-race lint fmt vet deps deps-update deps-verify docker-build docker-run docker-push docker-clean docker-compose-up docker-compose-down docker-compose-logs setup install uninstall release dev-setup check pre-commit ci

##@ General

help: ## Display this help message
	@echo "$(CYAN)$(PROJECT_NAME) Makefile$(NC)"
	@echo "$(YELLOW)Version: $(VERSION)$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build

build: ## Build the application
	@echo "$(BLUE)Building $(PROJECT_NAME)...$(NC)"
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "$(GREEN)Build completed: $(BINARY_PATH)$(NC)"

build-linux: ## Build for Linux
	@echo "$(BLUE)Building $(PROJECT_NAME) for Linux...$(NC)"
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "$(GREEN)Linux build completed$(NC)"

build-windows: ## Build for Windows
	@echo "$(BLUE)Building $(PROJECT_NAME) for Windows...$(NC)"
	@mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)
	@echo "$(GREEN)Windows build completed$(NC)"

build-darwin: ## Build for macOS
	@echo "$(BLUE)Building $(PROJECT_NAME) for macOS...$(NC)"
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "$(GREEN)macOS builds completed$(NC)"

build-all: build-linux build-windows build-darwin ## Build for all platforms
	@echo "$(GREEN)All platform builds completed$(NC)"

##@ Testing

test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	$(GOTEST) -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)
	@echo "$(GREEN)Tests completed$(NC)"

test-verbose: ## Run tests with verbose output
	@echo "$(BLUE)Running tests with verbose output...$(NC)"
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)

test-coverage: ## Run tests with coverage
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GOTEST) -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_FILE) $(TEST_PACKAGES)
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "$(GREEN)Coverage report generated: $(COVERAGE_HTML)$(NC)"

test-race: ## Run tests with race detection
	@echo "$(BLUE)Running tests with race detection...$(NC)"
	$(GOTEST) -race -timeout $(TEST_TIMEOUT) $(TEST_PACKAGES)

##@ Code Quality

lint: ## Run linter (requires golangci-lint)
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found, skipping...$(NC)"; \
	fi

fmt: ## Format Go code
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GOFMT) ./...
	@echo "$(GREEN)Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GOVET) $(TEST_PACKAGES)
	@echo "$(GREEN)Vet completed$(NC)"

##@ Dependencies

deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GOMOD) download
	@echo "$(GREEN)Dependencies downloaded$(NC)"

deps-update: ## Update dependencies
	@echo "$(BLUE)Updating dependencies...$(NC)"
	$(GOMOD) tidy
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

deps-verify: ## Verify dependencies
	@echo "$(BLUE)Verifying dependencies...$(NC)"
	$(GOMOD) verify
	@echo "$(GREEN)Dependencies verified$(NC)"

##@ Docker

docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(DOCKER_LATEST) -t $(DOCKER_VERSIONED) .
	@echo "$(GREEN)Docker image built: $(DOCKER_LATEST), $(DOCKER_VERSIONED)$(NC)"

docker-run: ## Run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	docker run --rm -it -p 8080:8080 $(DOCKER_LATEST)

docker-push: ## Push Docker image to registry
	@echo "$(BLUE)Pushing Docker image...$(NC)"
	docker push $(DOCKER_VERSIONED)
	docker push $(DOCKER_LATEST)
	@echo "$(GREEN)Docker image pushed$(NC)"

docker-clean: ## Clean Docker images and containers
	@echo "$(BLUE)Cleaning Docker resources...$(NC)"
	docker rmi $(DOCKER_LATEST) $(DOCKER_VERSIONED) 2>/dev/null || true
	docker system prune -f
	@echo "$(GREEN)Docker cleanup completed$(NC)"

##@ Docker Compose

docker-compose-up: ## Start services with Docker Compose
	@echo "$(BLUE)Starting services with Docker Compose...$(NC)"
	./scripts/deploy.sh start

docker-compose-down: ## Stop services with Docker Compose
	@echo "$(BLUE)Stopping services with Docker Compose...$(NC)"
	./scripts/deploy.sh stop

docker-compose-logs: ## Show Docker Compose logs
	@echo "$(BLUE)Showing Docker Compose logs...$(NC)"
	./scripts/deploy.sh logs

##@ Development

setup: deps ## Setup development environment
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@mkdir -p $(BIN_DIR) $(BUILD_DIR) $(DIST_DIR)
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@echo "$(GREEN)Development environment setup completed$(NC)"

dev-setup: setup ## Setup development environment with additional tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2; \
	fi
	@echo "$(GREEN)Development tools installed$(NC)"

check: fmt vet test ## Run all checks (format, vet, test)
	@echo "$(GREEN)All checks completed$(NC)"

pre-commit: fmt vet test-race lint ## Run pre-commit checks
	@echo "$(GREEN)Pre-commit checks completed$(NC)"

##@ Installation

install: build ## Install the binary to GOPATH/bin
	@echo "$(BLUE)Installing $(PROJECT_NAME)...$(NC)"
	@cp $(BINARY_PATH) $$(go env GOPATH)/bin/
	@echo "$(GREEN)$(PROJECT_NAME) installed to $$(go env GOPATH)/bin/$(NC)"

uninstall: ## Uninstall the binary from GOPATH/bin
	@echo "$(BLUE)Uninstalling $(PROJECT_NAME)...$(NC)"
	@rm -f $$(go env GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)$(PROJECT_NAME) uninstalled$(NC)"

##@ Release

release: clean build-all test docker-build ## Create a release build
	@echo "$(BLUE)Creating release $(VERSION)...$(NC)"
	@mkdir -p $(DIST_DIR)
	@cp $(BIN_DIR)/* $(DIST_DIR)/ 2>/dev/null || true
	@echo "$(GREEN)Release $(VERSION) created in $(DIST_DIR)$(NC)"

##@ Cleanup

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	$(GOCLEAN)
	@rm -rf $(BIN_DIR) $(BUILD_DIR) $(DIST_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "$(GREEN)Cleanup completed$(NC)"

##@ CI/CD

ci: deps check test-coverage ## Run CI pipeline
	@echo "$(GREEN)CI pipeline completed$(NC)"

##@ Information

version: ## Show version information
	@echo "$(CYAN)$(PROJECT_NAME) $(VERSION)$(NC)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"

status: ## Show project status
	@echo "$(CYAN)Project Status$(NC)"
	@echo "Name: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $$(go version)"
	@echo "Docker: $$(docker --version 2>/dev/null || echo 'Not available')"
	@echo "Git Status: $$(git status --porcelain | wc -l) modified files"
	@if [ -f $(BINARY_PATH) ]; then echo "Binary: Built"; else echo "Binary: Not built"; fi