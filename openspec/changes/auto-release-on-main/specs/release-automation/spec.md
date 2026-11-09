# Spec Delta

## Purpose

Releases `aws-sso-login` without manual steps: every green push to `main` that contains user-facing Conventional Commits is versioned, recorded in `CHANGELOG.md`, tagged, published with generated notes, and handed to the Homebrew tap.

## ADDED Requirements

### Requirement: Release evaluation after green CI on main
The release decision SHALL run only after the CI workflow for a push to `main` finishes successfully, and SHALL evaluate the commit that CI tested. When CI fails, is cancelled, or ran for a pull request, no tag or release SHALL be created. When `main` has moved past the tested commit, the run SHALL stop without releasing and leave the decision to the run for the newer commit.

#### Scenario: CI passed for a push to main
- **WHEN** a pull request with a `feat:` commit is merged and CI for the resulting push to `main` succeeds
- **THEN** the release evaluation runs for that commit

#### Scenario: CI failed
- **WHEN** CI for a push to `main` fails
- **THEN** no tag, changelog commit or release is created

#### Scenario: Main moved on
- **WHEN** the evaluation starts for commit A but `main` already points to a newer commit B
- **THEN** the run ends without a release and reports that the run for B decides

### Requirement: Version from Conventional Commits
The next version SHALL be derived from the non-merge commits between the latest `vX.Y.Z` tag reachable from the evaluated commit and that commit. A breaking change (`!` after the type or scope, or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer) SHALL bump major, `feat` minor, and `fix`, `perf` or `revert` patch; the highest bump wins. While the major version is `0`, a breaking change SHALL bump minor.

#### Scenario: Feature and fix
- **WHEN** the latest tag is `v1.4.2` and the new commits are `fix: handle empty cache` and `feat(cli): add --region`
- **THEN** the next version is `v1.5.0`

#### Scenario: Breaking change on a 1.x version
- **WHEN** the latest tag is `v1.4.2` and a new commit is `feat(cli)!: rename --profile`
- **THEN** the next version is `v2.0.0`

#### Scenario: Breaking change while the major version is 0
- **WHEN** the latest tag is `v0.3.0` and a new commit body has a `BREAKING CHANGE: exit codes changed` footer
- **THEN** the next version is `v0.4.0`

#### Scenario: Patch only
- **WHEN** the latest tag is `v0.3.0` and the only releasing commit is `perf: cache profile list`
- **THEN** the next version is `v0.3.1`

#### Scenario: No tag yet
- **WHEN** the repository has no `vX.Y.Z` tag and a commit is `feat: initial`
- **THEN** the next version is computed from `v0.0.0`, giving `v0.1.0`

### Requirement: No release without user-facing commits
No tag, changelog commit or release SHALL be created when none of the new commits is a breaking change, `feat`, `fix`, `perf` or `revert`. Commits of other types (`docs`, `chore`, `ci`, `test`, `build`, `style`, `refactor`), merge commits and subjects that are not Conventional Commits SHALL NOT trigger a release on their own.

#### Scenario: Documentation and dependency merges
- **WHEN** the only new commits are `docs(openspec): archive change` and `chore(deps): bump actions/checkout`
- **THEN** the run ends successfully without a tag and states that nothing is releasable

#### Scenario: Non-conventional subject
- **WHEN** the only new commit is `Update README`
- **THEN** no release is created

### Requirement: Changelog promotion
Before tagging `vX.Y.Z`, the content of the `## [Unreleased]` section of `CHANGELOG.md` SHALL move into a new `## [vX.Y.Z] - YYYY-MM-DD` section (UTC date) directly below an empty `## [Unreleased]` heading. The `[Unreleased]` compare link SHALL start at the new tag and a `[vX.Y.Z]` link SHALL compare the previous tag with the new one.

#### Scenario: Curated entries are kept
- **WHEN** `[Unreleased]` contains `### Fixed` entries and `v0.3.1` is released on 2026-11-10
- **THEN** `CHANGELOG.md` has an empty `## [Unreleased]`, followed by `## [v0.3.1] - 2026-11-10` with exactly those `### Fixed` entries, and the links `[Unreleased]: .../compare/v0.3.1...HEAD` and `[v0.3.1]: .../compare/v0.3.0...v0.3.1`

#### Scenario: Empty Unreleased section
- **WHEN** `[Unreleased]` has no entries and the new commits are `feat: add --region` and `fix: handle empty cache`
- **THEN** the new version section lists them under `### Added` and `### Fixed`, and breaking changes, if any, appear under `### Changed` prefixed with `**BREAKING**:`

### Requirement: Release commit and tag
The promoted changelog SHALL be committed to `main` as `chore(release): vX.Y.Z` on top of the evaluated commit, and an annotated tag `vX.Y.Z` SHALL point to that commit. The commit and the tag SHALL be pushed in one atomic push, so either both reach the repository or neither does. The release commit SHALL NOT start another release evaluation.

#### Scenario: Atomic push
- **WHEN** the push of the release commit is rejected because `main` changed
- **THEN** the tag is not pushed either, no release is published, and the run fails with the rejection

#### Scenario: Tag already exists
- **WHEN** the computed tag `vX.Y.Z` already exists in the repository
- **THEN** the run fails before committing, naming the existing tag, and nothing is pushed

#### Scenario: Release commit does not loop
- **WHEN** the `chore(release): v0.4.0` commit lands on `main`
- **THEN** no new release evaluation produces another tag for it

### Requirement: Publishing auto-created tags
A tag created by the release automation SHALL be built and published by the same release process as a manually pushed tag, within the same automation run, so the published release has the same binaries, checksums file and description format.

#### Scenario: Auto-created tag is published
- **WHEN** the automation pushes tag `v0.4.0`
- **THEN** a GitHub release `v0.4.0` is published with the four binaries and the checksums file, and no second publish run starts for the same tag

#### Scenario: Manually pushed tag
- **WHEN** a maintainer pushes tag `v0.4.1` by hand and `CHANGELOG.md` has its section
- **THEN** the release is published as before, with the generated description

### Requirement: Changelog section required for release
The publish process SHALL fail before creating the release or uploading assets when `CHANGELOG.md` at the tagged commit has no `## [vX.Y.Z]` section or that section has no content.

#### Scenario: Changelog not updated for a manual tag
- **WHEN** tag `v1.2.4` is pushed by hand and `CHANGELOG.md` has no `## [v1.2.4]` section
- **THEN** the publish run fails with a message naming the missing section, and no release or asset is published

### Requirement: Generated release notes
The release description for `vX.Y.Z` SHALL list the changes since the previous tag in this order, omitting empty sections: breaking changes, features, fixes, improvements (`perf`, `revert`), other pull requests (merged pull requests with no releasing commit), closed issues, and a compare link to the previous tag. Each change entry SHALL show the commit description with its scope and link the pull request that merged it.

#### Scenario: Entries link their pull requests
- **WHEN** commit `feat(cli): filter the profile picker as you type` was merged by pull request #50
- **THEN** the "Features" section contains `**cli:** filter the profile picker as you type` with a link to #50

#### Scenario: Documentation-only pull request
- **WHEN** pull request #52 merged only `docs(openspec): ...` commits in the range
- **THEN** it appears under "Other pull requests" with its title and link, and not under features or fixes

#### Scenario: Commit pushed without a pull request
- **WHEN** a `fix:` commit reached `main` without a pull request
- **THEN** it appears under "Fixes" with a link to the commit instead of a pull request

#### Scenario: First release has no previous tag
- **WHEN** the release has no previous tag
- **THEN** the description lists all releasing commits and omits the compare link

### Requirement: Closed issues in release notes
The "Closed issues" section SHALL list, once each, every issue closed by a pull request merged in the release range, with its number, title and link. When no issue was closed, the section SHALL be omitted.

#### Scenario: Pull request closes issues
- **WHEN** pull request #46 in the range closed issues #42 and #43
- **THEN** "Closed issues" lists #42 and #43 with their titles

#### Scenario: Issue closed by two pull requests
- **WHEN** issue #34 is linked as closed by two pull requests in the range
- **THEN** #34 appears once

### Requirement: Homebrew tap notification
After a release and all of its assets are published, the release process SHALL start the "Fetch Release" workflow of `witnsby/homebrew-aws-sso-login` on `main` with input `tag=vX.Y.Z`, authenticating with the `HOMEBREW_TAP_TOKEN` repository secret. It SHALL NOT start the tap workflow when publishing failed.

#### Scenario: Release published
- **WHEN** release `v0.4.0` and its assets are published and `HOMEBREW_TAP_TOKEN` is set
- **THEN** the tap's "Fetch Release" workflow starts with `tag=v0.4.0`

#### Scenario: Secret missing
- **WHEN** `HOMEBREW_TAP_TOKEN` is not set
- **THEN** the release stays published, the notification step reports a warning that the tap will pick up the release on its daily schedule, and the run does not fail

#### Scenario: Dispatch rejected
- **WHEN** the tap dispatch fails, for example because the token lacks access
- **THEN** the notification job fails with the API error while the published release is kept

#### Scenario: Asset upload failed
- **WHEN** uploading a binary to the release fails
- **THEN** the tap workflow is not started

### Requirement: Tap formula update for a given tag
The tap's "Fetch Release" workflow SHALL accept an optional `tag` input. With a tag, it SHALL update the formula to that release of `witnsby/aws-sso-login` and fail when the release does not exist or lacks any of the four platform binaries. Without a tag (push, schedule or manual run), it SHALL keep using the latest release.

#### Scenario: Dispatch with a tag
- **WHEN** "Fetch Release" runs with `tag=v0.4.0`
- **THEN** the formula version, URLs and SHA-256 sums point to the `v0.4.0` assets and the change is committed to the tap's `main`

#### Scenario: Unknown tag
- **WHEN** "Fetch Release" runs with `tag=v9.9.9` that has no release
- **THEN** the run fails with a message naming the tag and the formula is unchanged

#### Scenario: Scheduled run
- **WHEN** the daily schedule runs without a tag
- **THEN** the formula is updated to the latest release, or left unchanged when it is already current
