# Spec Delta

## Purpose

Defines the checks every change must pass in CI and what a tagged release publishes, so binaries are built with a supported toolchain and users can verify them.

## ADDED Requirements

### Requirement: Supported Go toolchain
The module SHALL declare a Go release that is supported upstream. CI and release workflows SHALL take the Go version from `go.mod`. Binaries SHALL stay statically linked with `CGO_ENABLED=0`.

#### Scenario: Workflows follow go.mod
- **WHEN** the `go` directive in `go.mod` is changed
- **THEN** CI and release builds use that version without editing the workflows

### Requirement: CI checks
CI SHALL run on every pull request to `main` and on every push to `main`. It SHALL fail when `go vet ./...` reports problems, `gofmt -l ./src` prints any file, tests with the race detector fail, `govulncheck` reports a vulnerability in called code, or any release target fails to build.

#### Scenario: Unformatted file
- **WHEN** a pull request contains a Go file that `gofmt -l ./src` lists
- **THEN** the CI run fails

#### Scenario: Push to main
- **WHEN** a commit is pushed to `main`
- **THEN** the same CI checks run

### Requirement: Release artifacts
A `vX.Y.Z` tag SHALL publish binaries for darwin/amd64, darwin/arm64, linux/amd64 and linux/arm64, plus a SHA-256 checksums file covering every binary. The workflow SHALL authenticate with the workflow token scoped to `contents: write`. It SHALL fail when creating the release or uploading assets fails for any reason other than the release already existing.

#### Scenario: Checksums published
- **WHEN** tag `v1.2.3` is pushed
- **THEN** the release contains the four binaries and a checksums file with one SHA-256 line per binary

#### Scenario: Upload failure
- **WHEN** uploading an asset fails
- **THEN** the release workflow fails

### Requirement: Local build targets
`make help` SHALL list only targets that exist, including a `clean` target that removes build output. Each target SHALL stop at the first failing command. Version detection SHALL print no error when the repository has no tags.

#### Scenario: Clean target
- **WHEN** the user runs `make clean` after `make build`
- **THEN** the build output directory is removed

#### Scenario: Repository without tags
- **WHEN** `make build` runs in a clone without tags
- **THEN** the version defaults to `v0.0.0` and no git error is printed
