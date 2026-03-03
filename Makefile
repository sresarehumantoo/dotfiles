BINARY     := dfinstall
MCP_BINARY := dfinstall-mcp
BUILD_DIR  := bin
SRC_DIR    := src/cmd/dfinstall
MCP_SRC    := src/cmd/mcp
GO         := go

.PHONY: build build-mcp test install clean fmt lint

build:
	$(GO) build -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(CURDIR)" \
	  -o $(BUILD_DIR)/$(BINARY) ./$(SRC_DIR)

build-mcp:
	$(GO) build -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(CURDIR)" \
	  -o $(BUILD_DIR)/$(MCP_BINARY) ./$(MCP_SRC)

test:
	$(GO) test ./src/... ./tests/...

install: build
	./$(BUILD_DIR)/$(BINARY) install all

clean:
	rm -rf $(BUILD_DIR)

fmt:
	gofmt -s -w src/ tests/

lint:
	$(GO) vet ./src/... ./tests/...
