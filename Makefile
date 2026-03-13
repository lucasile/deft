# Makefile for Deft - AI Server Manager

.PHONY: all agent installer panel clean test tidy

# Go binary settings
GO := go
GOTOOLCHAIN := auto
PATH := /home/lucasi/sdk/go1.26.1/bin:$(PATH)

# Build outputs
BIN_DIR := ./bin
AGENT_BIN := $(BIN_DIR)/deft
INSTALLER_BIN := $(BIN_DIR)/deft-install
PANEL_BIN := $(BIN_DIR)/deft-panel

# Ensure bin directory exists
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Default target: build all components
all: agent installer panel

# Build the agent module
agent: $(BIN_DIR)
	@echo "Building Agent..."
	cd agent && $(GO) build -o ../$(AGENT_BIN) .

# Build the installer module
installer: $(BIN_DIR)
	@echo "Building Installer..."
	cd installer && $(GO) build -o ../$(INSTALLER_BIN) .

# Build the panel module
panel: $(BIN_DIR)
	@echo "Building Panel Frontend..."
	cd panel/web && pnpm install && pnpm run build
	@echo "Building Panel Backend..."
	cd panel && $(GO) build -o ../$(PANEL_BIN) .

# Clean up built binaries
clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf panel/web/build
	rm -rf panel/web/.svelte-kit

# Run tests for all modules
test:
	@echo "Running tests..."
	$(GO) test ./...

# Tidy up Go modules
tidy:
	@echo "Tidying up modules..."
	$(GO) work sync
	cd agent && $(GO) mod tidy
	cd installer && $(GO) mod tidy
	cd panel && $(GO) mod tidy
	cd proto && $(GO) mod tidy
	cd internal/i18n && $(GO) mod tidy
