SHELL=bash
# Stop each recipe at the first failing command, including inside pipes.
.SHELLFLAGS := -eu -o pipefail -c

# Default platform parameters if not specified
OS ?= $(GOOS)
ARCH ?= $(GOARCH)
OUT ?= aws-sso-login
BUILD_DIR := ./tmp

VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS = -X github.com/witnsby/aws-sso-login/src/internal/helper.Version=$(VERSION) \
          -X github.com/witnsby/aws-sso-login/src/internal/helper.CommitHash=$(COMMIT_HASH)

# Install dependencies
.PHONY: install-deps
install-deps:
	go mod tidy

# Run tests
.PHONY: tests
tests:
	go test ./... -v

.PHONY: cover
cover:
	go test -short -count=1 -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	rm ./coverage.out

# Static checks
.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt-check
fmt-check:
	@out="$$(gofmt -l ./src)"; \
	if [ -n "$$out" ]; then echo "Files need gofmt:"; echo "$$out"; exit 1; fi

# Build single binary
.PHONY: build
build:
	@echo "Building $(OUT) for OS=$(OS) ARCH=$(ARCH)"
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(OUT) ./src/cmd/bin/main.go

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

# SHA-256 checksums for the release binaries
.PHONY: checksums
checksums:
	cd $(BUILD_DIR) && \
	if command -v sha256sum >/dev/null; then sha256sum $(OUT)_*_* > $(OUT)_checksums.txt; \
	else shasum -a 256 $(OUT)_*_* > $(OUT)_checksums.txt; fi

# Remove build output
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  install-deps    Install dependencies"
	@echo "  tests           Run tests"
	@echo "  cover           Run coverage"
	@echo "  vet             Run go vet"
	@echo "  fmt-check       Fail if any Go file needs gofmt"
	@echo "  build           Build the application (single platform)"
	@echo "  build-all       Build the application for multiple platforms"
	@echo "  checksums       Write SHA-256 checksums for the built binaries"
	@echo "  clean           Remove build artifacts"
