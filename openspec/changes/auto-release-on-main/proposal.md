# Proposal

## Why

Releasing `aws-sso-login` takes three manual steps today: a release PR that adds a `## [vX.Y.Z]` section to `CHANGELOG.md`, a hand-pushed tag, and a manual run of the Homebrew tap's "Fetch Release" workflow (or a wait of up to a day for its schedule). Merged work reaches users late, version numbers depend on someone reading the commit history, and the release page says nothing about which pull requests and issues went into it.

## What Changes

- New "Auto Release" workflow: after the "Build Release" CI run for a push to `main` succeeds, it reads the Conventional Commits since the last `vX.Y.Z` tag and decides whether to release and which version:
  - `feat` → minor, `fix` / `perf` / `revert` → patch, `!` or a `BREAKING CHANGE:` footer → major (minor while the major version is `0`).
  - Only `docs`, `chore`, `ci`, `test`, `build`, `style`, `refactor` or non-conventional commits → no tag and no release.
- When it releases, the workflow moves the `[Unreleased]` content of `CHANGELOG.md` into a new `## [vX.Y.Z] - YYYY-MM-DD` section (or writes entries generated from the commits when `[Unreleased]` is empty), updates the compare links, commits `chore(release): vX.Y.Z` to `main`, tags that commit and pushes both in one atomic push.
- The same run builds and publishes the GitHub release by calling the existing "Publish Release" workflow, which becomes reusable (`workflow_call`) and keeps its tag-push trigger for manually pushed tags.
- The release description is generated instead of copied from `CHANGELOG.md`: sections for breaking changes, features, fixes, improvements and other pull requests, each entry linking its pull request, then the issues those pull requests closed, then a compare link to the previous tag. A `CHANGELOG.md` section for the tag is still required before anything is published.
- After the release and all its assets are published, the workflow starts the tap's "Fetch Release" workflow in `witnsby/homebrew-aws-sso-login` with the new tag. It authenticates with a new repository secret `HOMEBREW_TAP_TOKEN` (fine-grained token, `Actions: read and write` on the tap only). Without the secret, the release still succeeds and the run reports a warning; the tap's daily schedule remains the fallback.
- Homebrew tap (separate PR in `witnsby/homebrew-aws-sso-login`): "Fetch Release" accepts an optional `tag` input, and the updater pins the formula to that release instead of "latest" when the input is set.
- A small release helper written in Go, using only the standard library, under `src/internal/release` with entrypoint `src/cmd/release`, holds the version, changelog and notes logic so it is unit-tested. It is not part of the `aws-sso-login` binary.

User-visible impact per command:

- `console`, `export`, `import`, `process`, `version`: no behavior change. Released binaries report the auto-computed version through `version`.
- No flag, default, stdout format or exit code changes. Nothing in this change is **BREAKING** for CLI users.
- Contributors: commit subjects on `main` now decide releases, so they must follow Conventional Commits; a `feat` or `fix` merged to `main` is released to GitHub and Homebrew without further action.

Out of scope:

- Release PRs or manual approval gates before publishing (release-please style flows).
- Signing binaries, SBOMs or provenance attestations.
- Pre-release or build-metadata versions (`v1.2.3-rc.1`).
- Commenting on closed issues or pull requests after a release.
- Enforcing Conventional Commit subjects in CI (a commit-lint check on pull requests).
- Changes to how Dependabot labels its commits (`chore(deps)` keeps dependency-only merges from releasing).

## Capabilities

### New Capabilities

- `release-automation`: deciding from Conventional Commits whether and which version to release after a green push to `main`, promoting the changelog, tagging, generating release notes with pull requests and closed issues, and notifying the Homebrew tap.

### Modified Capabilities

- `build-release`: the "Release notes from the changelog" requirement is replaced. The release description is now generated (see `release-automation`); the existing gate that a `CHANGELOG.md` section exists before publishing moves to `release-automation`. The "Publish Release" workflow is also callable from another workflow.

## Impact

- Workflows: new `.github/workflows/auto_release.yml`; `.github/workflows/publish_release.yml` gains `workflow_call`, generated notes and the tap dispatch job.
- Code: new `src/internal/release` package and `src/cmd/release/main.go` (standard library only, no new module dependencies). The CLI binary is unchanged; `make tests`, `vet` and `fmt-check` cover the helper.
- Repository settings: new secret `HOMEBREW_TAP_TOKEN` (created by a maintainer). `main` must accept pushes from `github-actions[bot]`; the `main` ruleset is currently disabled, and enabling it later needs a bypass for the workflow.
- Docs: `README.md` (release process), `AGENTS.md` (Conventional Commits now drive releases), `CHANGELOG.md` `Unreleased`.
- Homebrew tap repository: `publish_release.yml` input, `src/main.go` tag selection with tests, `README.md` / `AGENTS.md`; delivered as its own PR in that repository.

## Related issues

- Refs #43 (closed): it introduced release notes from `CHANGELOG.md`; this change generates them from commits, pull requests and closed issues while keeping the changelog gate.
- Refs #34 (closed): release pipeline hardening; the new workflow keeps SHA-pinned actions and least-privilege tokens.
- No open issue covers automatic tagging or tap notification (searched: "automatic release", "homebrew", "semver OR version bump", "release OR tag OR changelog").
