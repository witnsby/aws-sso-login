# build-release Specification

## Purpose
Defines the checks every change must pass in CI and what a tagged release publishes, so binaries are built with a supported toolchain and users can verify them.

## Requirements

### Requirement: Supported Go toolchain
The module SHALL declare a Go release that is supported upstream. The `go` directive in `go.mod` SHALL name only the minimum language version (`major.minor`), and a `toolchain` directive SHALL pin the patch release used by CI and release builds. CI and release workflows SHALL take the Go version from `go.mod`. Binaries SHALL stay statically linked with `CGO_ENABLED=0`.

#### Scenario: Workflows follow go.mod
- **WHEN** the `toolchain` directive in `go.mod` is changed
- **THEN** CI and release builds use that toolchain version without editing the workflows

#### Scenario: Older patch release builds from source
- **WHEN** a user with Go 1.27.0 and `GOTOOLCHAIN=local` runs `make build`
- **THEN** the build succeeds without downloading another toolchain

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

### Requirement: Pinned workflow actions
Every `uses:` reference to an action in the repository workflows SHALL name a full 40-character commit SHA, followed by a comment with the release version it corresponds to. CI SHALL fail when a workflow references an action by tag or branch.

#### Scenario: Action referenced by tag
- **WHEN** a pull request changes a workflow to `uses: actions/checkout@v4`
- **THEN** the CI run fails and names the unpinned reference

#### Scenario: Pinned action accepted
- **WHEN** every workflow references actions as `owner/repo@<40-hex-sha> # vX.Y.Z`
- **THEN** the pin check passes

#### Scenario: Dependabot update keeps the pin
- **WHEN** Dependabot proposes a newer version of a pinned action
- **THEN** the proposed reference is again a full commit SHA with the new version in the comment

### Requirement: Release notes from the changelog
The release for tag `vX.Y.Z` SHALL use as its description the body of the `## [vX.Y.Z]` section of `CHANGELOG.md`, up to the next `## [` heading. When that section is missing or has no content, the release workflow SHALL fail before creating the release or uploading assets.

#### Scenario: Notes taken from the changelog
- **WHEN** tag `v1.2.3` is pushed and `CHANGELOG.md` has a `## [v1.2.3] - 2026-11-09` section with `### Fixed` entries
- **THEN** the published release description contains those `### Fixed` entries and nothing from other versions

#### Scenario: Changelog not updated
- **WHEN** tag `v1.2.4` is pushed and `CHANGELOG.md` has no `## [v1.2.4]` section
- **THEN** the release workflow fails with a message naming the missing section, and no release or asset is published

#### Scenario: Release already exists
- **WHEN** a release for the tag already exists
- **THEN** its assets are uploaded as before and the workflow does not fail because the release exists

### Requirement: Dependency update configuration
Dependabot SHALL be configured only for package ecosystems present in the repository (`github-actions` and `gomod`). Its commit messages SHALL follow Conventional Commits with the `chore` type and the `deps` scope.

#### Scenario: No configuration for missing directories
- **WHEN** Dependabot runs on the repository
- **THEN** it reports no error for an ecosystem or directory that does not exist

#### Scenario: Conventional commit message
- **WHEN** Dependabot opens a pull request that updates a Go module
- **THEN** its commit message starts with `chore(deps):`
