# Makefile for Deft - AI Server Manager

.PHONY: all agent-cli daemon installer panel docker-panel clean test tidy check-root

# Go settings
GO := go
GOTOOLCHAIN := auto
PATH := /home/lucasi/sdk/go1.26.1/bin:$(PATH)

# Output directory
BIN_DIR := ./bin

# Binary names
CLI_BIN := $(BIN_DIR)/deft
DAEMON_BIN := $(BIN_DIR)/deftd
INSTALLER_BIN := $(BIN_DIR)/deft-install
PANEL_BIN := $(BIN_DIR)/deft-panel

# Helper to check for root/sudo
check-root:
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: This target requires sudo / root privileges."; \
		exit 1; \
	fi

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

all: agent-cli daemon installer panel

agent-cli: $(BIN_DIR)
	@echo "Building CLI (deft)..."
	$(GO) build -o $(CLI_BIN) ./cmd/deft

daemon: $(BIN_DIR)
	@echo "Building Daemon (deftd)..."
	$(GO) build -o $(DAEMON_BIN) ./cmd/deftd

installer: $(BIN_DIR)
	@echo "Building Installer (deft-install)..."
	$(GO) build -o $(INSTALLER_BIN) ./cmd/deft-install

panel: $(BIN_DIR)
	@echo "Building Panel Frontend..."
	cd internal/panel/ui/web && pnpm install && pnpm run build
	@echo "Building Panel Backend (deft-panel)..."
	$(GO) build -o $(PANEL_BIN) ./cmd/deft-panel

docker-panel: check-root
	@echo "Building Panel Docker Image (requires sudo)..."
	docker build -t ghcr.io/lucasile/deft-panel:latest -f panel/Dockerfile .

clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf internal/panel/ui/web/build
	rm -rf internal/panel/ui/web/.svelte-kit

test:
	@echo "Running tests..."
	$(GO) test ./...

tidy:
	@echo "Tidying module..."
	$(GO) mod tidy
