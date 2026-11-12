# tui-workflow — Makefile
# Build & install helpers. `make install` installs into the user's home directory.

SHELL := /bin/bash

BINARY      ?= tui-workflow
BIN_DIR     ?= bin
INSTALL_DIR ?= $(HOME)/.local/bin

GO          ?= go
LDFLAGS     ?= -s -w
PKG         := ./cmd/tui-workflow

.PHONY: all build install uninstall run test vet clean help

all: build

build: ## Compile the binary into bin/
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "Built: $(BIN_DIR)/$(BINARY)"

install: build ## Install the binary into $(INSTALL_DIR) (user home)
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"
	@case ":$$PATH:" in *":$(INSTALL_DIR):"*) ;; \
		*) echo "Note: add $(INSTALL_DIR) to your PATH to run '$(BINARY)' anywhere." ;; \
	esac

uninstall: ## Remove the installed binary
	@rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed: $(INSTALL_DIR)/$(BINARY)"

run: ## Run from source
	$(GO) run $(PKG)

test: ## Run full test suite with race detector
	$(GO) test -v -race ./...

vet: ## Run go vet
	$(GO) vet ./...

clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR)
	@echo "Cleaned: $(BIN_DIR)"

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-12s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
