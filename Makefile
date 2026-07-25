BINARY     := dfinstall
MCP_BINARY := dfinstall-mcp
BUILD_DIR  := bin
SRC_DIR    := src/cmd/dfinstall
MCP_SRC    := src/cmd/mcp
GO         := go
LDFLAGS    := -X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(CURDIR)
LOCAL_BIN  := $(HOME)/.local/bin

.DEFAULT_GOAL := help

.PHONY: help build build-mcp build-all test test-race lint fmt fmt-check ci \
        install uninstall install-bin install-mcp uninstall-mcp clean

help: ## Show this help
	@echo "dfinstall — make targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Compile the dfinstall CLI to bin/dfinstall
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./$(SRC_DIR)

build-mcp: ## Compile the MCP server to bin/dfinstall-mcp
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(MCP_BINARY) ./$(MCP_SRC)

build-all: build build-mcp ## Compile both binaries

test: ## Run the test suite
	$(GO) test ./src/... ./tests/...

test-race: ## Run the test suite under the race detector
	$(GO) test -race ./src/... ./tests/...

lint: ## Run go vet
	$(GO) vet ./src/... ./tests/...

fmt: ## Format sources in place (gofmt -s -w)
	gofmt -s -w src/ tests/

fmt-check: ## Check formatting without modifying (same as CI)
	@unformatted=$$(gofmt -s -l src tests); \
	if [ -n "$$unformatted" ]; then \
	  echo "Not gofmt -s formatted (run 'make fmt'):"; echo "$$unformatted"; exit 1; \
	fi

ci: fmt-check lint build-all test test-race ## Run every check CI runs (fmt/vet/build/test/race)

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
