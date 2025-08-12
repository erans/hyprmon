.PHONY: build run install uninstall clean test profiles help

# Variables
BINARY_NAME := hyprmon
INSTALL_DIR := /usr/local/bin
BIN_DIR := ./bin
BUILD_DIR := ./build
GO := go

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS := -s -w
LDFLAGS += -X main.Version=$(VERSION)
LDFLAGS += -X main.GitCommit=$(GIT_COMMIT)
LDFLAGS += -X main.BuildDate=$(BUILD_DATE)
LDFLAGS += -X main.GoVersion=$(GO_VERSION)

GOFLAGS := -ldflags="$(LDFLAGS)"

# Default target
all: help

# Build the application
build: fmt-check
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	@$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Run the application
run: build
	@$(BIN_DIR)/$(BINARY_NAME)

# Run profile selection menu
profiles: build
	@$(BIN_DIR)/$(BINARY_NAME) profiles

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@sudo install -m 755 $(BIN_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installation complete: $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo "You can now run '$(BINARY_NAME)' from anywhere"

# Uninstall from system
uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstall complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BIN_DIR)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@$(GO) test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "Formatting complete"

# Check if code is formatted
fmt-check:
	@echo "Checking code formatting..."
	@fmt_output=$$($(GO) fmt ./... 2>&1); \
	if [ -n "$$fmt_output" ]; then \
		echo "❌ The following files need formatting:"; \
		echo "$$fmt_output"; \
		echo ""; \
		echo "Run 'make fmt' to fix formatting issues"; \
		exit 1; \
	else \
		echo "✓ Code formatting is correct"; \
	fi

# Check for issues
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "Dependencies updated"

# Development build with debug info
dev:
	@echo "Building $(BINARY_NAME) with debug info..."
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/$(BINARY_NAME) .
	@$(BIN_DIR)/$(BINARY_NAME)

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "Multi-platform build complete in $(BUILD_DIR)/"

# Show version information
version:
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Go Version: $(GO_VERSION)"

# Install git hooks
hooks:
	@echo "Installing git hooks..."
	@./scripts/install-hooks.sh

# Show help
help:
	@echo "HyprMon Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       - Build the application"
	@echo "  run         - Build and run the application"
	@echo "  profiles    - Run profile selection menu"
	@echo "  install     - Install to $(INSTALL_DIR)"
	@echo "  uninstall   - Remove from $(INSTALL_DIR)"
	@echo "  clean       - Remove build artifacts"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  deps        - Update dependencies"
	@echo "  dev         - Build and run with debug info"
	@echo "  build-all   - Build for multiple platforms"
	@echo "  version     - Show version information"
	@echo "  hooks       - Install git pre-commit hooks"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build            # Build the application"
	@echo "  make run              # Build and run"
	@echo "  make install          # Install to system"
	@echo "  make profiles         # Open profile menu"