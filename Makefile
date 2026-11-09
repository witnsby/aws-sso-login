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

# Fail unless the active Go toolchain is the one pinned in go.mod
.PHONY: check-toolchain
check-toolchain:
	@want="$$(awk '$$1 == "toolchain" {print $$2}' go.mod)"; \
	got="$$(go env GOVERSION)"; \
	if [ -z "$$want" ]; then echo "go.mod has no toolchain directive"; exit 1; fi; \
	if [ "$$got" != "$$want" ]; then echo "Go toolchain is $$got, go.mod pins $$want"; exit 1; fi; \
	echo "Go toolchain $$got matches go.mod"

# Fail when a workflow references an action by tag or branch instead of a
# full commit SHA. Local actions (uses: ./path) are ignored.
WORKFLOWS ?= .github/workflows
.PHONY: check-action-pins
check-action-pins:
	@bad="$$(grep -HnE '^[[:space:]]*(-[[:space:]]*)?uses:' $(WORKFLOWS)/*.y*ml \
		| grep -vE 'uses:[[:space:]]*\./' \
		| grep -vE 'uses:[[:space:]]*[^[:space:]@]+@[0-9a-f]{40}([[:space:]]|$$)' || true)"; \
	if [ -n "$$bad" ]; then echo "Actions must be pinned to a full commit SHA:"; echo "$$bad"; exit 1; fi; \
	echo "All actions are pinned to commit SHAs"

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

# Release notes for VERSION: the body of its "## [VERSION]" section in
# CHANGELOG.md, up to the next "## [" heading or the link references.
# Fails when the section is missing or empty.
CHANGELOG ?= CHANGELOG.md
.PHONY: release-notes
release-notes:
	@mkdir -p $(BUILD_DIR)
	@awk -v v="$(VERSION)" ' \
		index($$0, "## [" v "]") == 1 { found = 1; next } \
		found && (/^## \[/ || /^\[[^]]+\]: /) { exit } \
		found { lines[++n] = $$0 } \
		END { \
			s = 1; while (s <= n && lines[s] ~ /^[[:space:]]*$$/) s++; \
			e = n; while (e >= s && lines[e] ~ /^[[:space:]]*$$/) e--; \
			for (i = s; i <= e; i++) print lines[i] \
		}' $(CHANGELOG) > $(BUILD_DIR)/release-notes.md
	@if [ ! -s $(BUILD_DIR)/release-notes.md ]; then \
		echo "$(CHANGELOG) has no entry for $(VERSION)"; exit 1; fi
	@echo "Release notes for $(VERSION) written to $(BUILD_DIR)/release-notes.md"

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
	@echo "  check-toolchain Fail unless the Go toolchain matches go.mod"
	@echo "  check-action-pins Fail if a workflow action is not pinned to a commit SHA"
	@echo "  build           Build the application (single platform)"
	@echo "  build-all       Build the application for multiple platforms"
	@echo "  checksums       Write SHA-256 checksums for the built binaries"
	@echo "  release-notes   Write the CHANGELOG.md section for VERSION=vX.Y.Z to tmp/release-notes.md"
	@echo "  clean           Remove build artifacts"
