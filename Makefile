BINARY     := dfinstall
MCP_BINARY := dfinstall-mcp
BUILD_DIR  := bin
SRC_DIR    := cmd/dfinstall
MCP_SRC    := cmd/dfinstall-mcp
GO         := go
# Derived from the tag so the version a binary reports can't drift from the
# release it was built at. Falls back to the short SHA on an untagged tree, and
# to "dev" outside a git checkout.
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -X github.com/sresarehumantoo/dotfiles/v2/internal/core.DefaultDotfilesDir=$(CURDIR) \
              -X github.com/sresarehumantoo/dotfiles/v2/internal/core.Version=$(VERSION)
LOCAL_BIN  := $(HOME)/.local/bin

.DEFAULT_GOAL := help

.PHONY: help build build-mcp build-all test test-race lint fmt fmt-check ci check-decoupling e2e corplab \
        install uninstall install-bin install-mcp uninstall-mcp clean

help: ## Show this help
	@echo "dfinstall — make targets:"
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Compile the dfinstall CLI to bin/dfinstall
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./$(SRC_DIR)

build-mcp: ## Compile the MCP server to bin/dfinstall-mcp
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(MCP_BINARY) ./$(MCP_SRC)

build-all: build build-mcp ## Compile both binaries

test: ## Run the test suite
	$(GO) test ./cmd/... ./internal/... ./tests/...

test-race: ## Run the test suite under the race detector
	$(GO) test -race ./cmd/... ./internal/... ./tests/...

lint: ## Run go vet
	$(GO) vet ./cmd/... ./internal/... ./tests/...

fmt: ## Format sources in place (gofmt -s -w)
	gofmt -s -w cmd/ internal/ tests/

fmt-check: ## Check formatting without modifying (same as CI)
	@unformatted=$$(gofmt -s -l cmd internal tests) || exit 1; \
	if [ -n "$$unformatted" ]; then \
	  echo "Not gofmt -s formatted (run 'make fmt'):"; echo "$$unformatted"; exit 1; \
	fi

check-decoupling: ## Fail if site-specific content leaked into tracked files
	./scripts/check-decoupling.sh

ci: fmt-check lint check-decoupling build-all test test-race ## Run every check CI runs (fmt/vet/decoupling/build/test/race)

e2e: ## Run the end-to-end scenarios in Docker (slow; SCENARIO=sudo|bootstrap|rootless for one)
	@for s in $(or $(SCENARIO),$(basename $(notdir $(wildcard scripts/e2e/scenarios/*.sh)))); do \
	  echo "== e2e: $$s"; scripts/e2e/run.sh $$s || exit 1; done

corplab: ## Run the corporate-network scenarios (TLS inspection, Nexus) in Docker; SCENARIO=<name> for one
	scripts/corplab/run.sh $(SCENARIO)

install: build ## Build, then apply all dotfile modules (dfinstall install all)
	./$(BUILD_DIR)/$(BINARY) install all

uninstall: build ## Build, then remove all managed symlinks (dfinstall uninstall all)
	./$(BUILD_DIR)/$(BINARY) uninstall all

install-bin: build-all ## Install both binaries onto PATH (~/.local/bin)
	@mkdir -p $(LOCAL_BIN)
	install -m 0755 $(BUILD_DIR)/$(BINARY) $(LOCAL_BIN)/$(BINARY)
	install -m 0755 $(BUILD_DIR)/$(MCP_BINARY) $(LOCAL_BIN)/$(MCP_BINARY)
	@echo "Installed $(BINARY) + $(MCP_BINARY) to $(LOCAL_BIN)"

install-mcp: build-mcp ## Register the MCP server with Claude Code (user scope, works anywhere)
	@command -v claude >/dev/null 2>&1 || { echo "claude CLI not found — install Claude Code first"; exit 1; }
	@claude mcp remove -s user dfinstall >/dev/null 2>&1 || true
	claude mcp add -s user dfinstall -- $(CURDIR)/$(BUILD_DIR)/$(MCP_BINARY)
	@echo "Registered 'dfinstall' MCP server (user scope). Restart Claude Code to load it."

uninstall-mcp: ## Unregister the MCP server from Claude Code
	@command -v claude >/dev/null 2>&1 || { echo "claude CLI not found"; exit 1; }
	claude mcp remove dfinstall

clean: ## Remove build artifacts (bin/)
	rm -rf $(BUILD_DIR)
