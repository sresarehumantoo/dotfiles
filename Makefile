BINARY    := dfinstall
BUILD_DIR := bin
SRC_DIR   := src/cmd/dfinstall
GO        := go

.PHONY: build test install clean fmt lint

build:
	$(GO) build -ldflags "-X github.com/sresarehumantoo/dotfiles/src/core.DefaultDotfilesDir=$(CURDIR)" \
	  -o $(BUILD_DIR)/$(BINARY) ./$(SRC_DIR)

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
