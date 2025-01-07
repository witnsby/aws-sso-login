SHELL=bash

# Default platform parameters if not specified
OS ?= $(GOOS)
ARCH ?= $(GOARCH)
OUT ?= aws-sso-login

VERSION=$(shell git describe --tags --abbrev=0 || echo "v0.0.0")
COMMIT_HASH=$(shell git rev-parse --short HEAD || echo "unknown")
LDFLAGS = -X github.com/witnsby/aws-sso-login/src/internal/helper.Version=$(VERSION) \
          -X github.com/witnsby/aws-sso-login/src/internal/helper.CommitHash=$(COMMIT_HASH)

# Install dependencies
.PHONY: install-deps
install-deps:
	go mod tidy

# Run tests
.PHONY: tests
tests:
	@set -e
	go test ./... -v || exit 1;

.PHONY: cover
cover:
	@set -e
	go test -short -count=1 -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	rm ./coverage.out

# Build single binary
.PHONY: build
build:
	@set -e
	@echo "Building $(OUT) for OS=$(OS) ARCH=$(ARCH)"
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o ./tmp/$(OUT) ./src/cmd/bin/main.go || exit 1;

# Build all variants
.PHONY: build-all
build-all:
	# macOS x86_64
	$(MAKE) build OS=darwin ARCH=amd64 OUT=$(OUT)_darwin_amd64 LDFLAGS="$(LDFLAGS)"
	# macOS arm64
	$(MAKE) build OS=darwin ARCH=arm64 OUT=$(OUT)_darwin_arm64 LDFLAGS="$(LDFLAGS)"
	# Linux x86_64
	$(MAKE) build OS=linux ARCH=amd64 OUT=$(OUT)_linux_amd64 LDFLAGS="$(LDFLAGS)"
	# Linux arm64
	$(MAKE) build OS=linux ARCH=arm64 OUT=$(OUT)_linux_arm64 LDFLAGS="$(LDFLAGS)"

# Help (optional)
help:
	@echo "Available targets:"
	@echo "  install-deps    Install dependencies"
	@echo "  tests           Run tests"
	@echo "  cover           Run coverage"
	@echo "  build           Build the application (single platform)"
	@echo "  build-all       Build the application for multiple platforms"
	@echo "  clean           Clean build artifacts"
