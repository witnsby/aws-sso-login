# Proposal

## Why

`aws-sso-login` parses `~/.aws/config` and `~/.aws/credentials` differently from the AWS CLI: go-ini cuts values at inline `#`/`;` and strips surrounding quotes, so `import` silently corrupts unrelated profiles and legacy profiles whose start URL ends in `/#` never hit the credential cache (#39). Each `import` also re-aligns every key in the credentials file (#40), and a loose "no access" match can skip a login that would have fixed the problem (#44). On the delivery side, the release pipeline runs third-party actions by moving tag with `contents: write` (#42), publishes a fixed release text (#43), raises the minimum Go version only to get a patched CI toolchain (#45), and Dependabot errors on every run because of a non-existent `docker` directory (#41).

## What Changes

- Read the AWS config file and the shared credentials file the way botocore's `configparser` does: inline `#` and `;` and surrounding double quotes are part of the value; only full-line `#`/`;` lines are comments. Both readers share one set of load options.
- `import` rewrites the credentials file without re-formatting it: sections and keys it does not touch keep their values and their `key = value` lines; only the imported profile's keys change.
- Legacy profiles whose `sso_start_url` contains `#` (for example `https://example.awsapps.com/start/#`) build the AWS CLI cache key from the full URL, so cached credentials are found again.
- "Role not assigned" is detected only from the AWS CLI exception names (`ForbiddenException`, `AccessDeniedException`), not from any stderr that contains "no access". Other failures fall back to the existing login / `aws sso login` advice.
- `go.mod` declares `go 1.27` as the minimum and `toolchain go1.27.3` for CI and release builds.
- Every GitHub Action in the workflows is pinned to a full commit SHA with the version in a comment.
- Release notes are the `CHANGELOG.md` section of the tag; the release workflow fails before publishing when that section is missing or empty.
- Dependabot configures only the ecosystems that exist (`github-actions`, `gomod`) and uses Conventional Commit messages (`chore(deps): ...`).

User-visible impact per command:

- `import`: other profiles in the credentials file are no longer truncated, unquoted or re-aligned. The written file contains `key = value` lines without column alignment.
- `console`, `export`, `process`, `import`: legacy profiles with `#` in `sso_start_url` find their cached credentials. Values with inline `#`, `;` or surrounding quotes in `~/.aws/config` are read exactly as the AWS CLI reads them. A failure whose stderr only says "no access" without an AWS exception name now leads to `aws sso login` (`console`, `import`) or to the `aws sso login` advice (`export`, `process`) instead of the "role not assigned" message.
- No flag, default, stdout format or exit code changes. Nothing in this change is **BREAKING**.

Out of scope:

- Replacing go-ini or writing a custom INI parser.
- Preserving blank lines and comment placement byte-for-byte; go-ini still normalizes those.
- Other configparser features go-ini handles differently (indented continuation lines, `%` interpolation, `:` as a delimiter).
- Signing release binaries, SBOMs or provenance attestations.
- Changes to the Homebrew tap.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `profile-resolution`: new requirement that config values are read like the AWS CLI reads them (inline `#`/`;` and quotes kept).
- `credentials-file`: "Atomic update preserving other profiles" now requires untouched sections to keep their values and lines, and requires AWS-CLI-compatible value parsing.
- `credential-retrieval`: the cache key uses the start URL exactly as written; "Role not assigned" is detected only from AWS exception names.
- `build-release`: minimum Go version vs. pinned toolchain, SHA-pinned actions, release notes from `CHANGELOG.md`, Dependabot configuration.

## Impact

- Code: `src/internal/profiles/profiles.go` (load options), `src/internal/cli/credsfile.go` (load options, write format), `src/internal/cli/credentials.go` (`isSSORoleNoAccessStderr`), a small shared package for go-ini options, and their tests.
- Build and CI: `go.mod`, `.github/workflows/build_release.yaml`, `.github/workflows/publish_release.yml`, `.github/dependabot.yml`, a changelog-extraction script or Makefile target.
- Docs: `README.md` (Go requirement, release notes), `CHANGELOG.md` (Unreleased).
- Dependencies: none added.

## Related issues

Searched with `gh issue list --state all --search` for "go-ini inline comment", "toolchain", "release notes", "pin actions SHA", "dependabot", "no access". #34 and #33 (closed) touched the same areas earlier and are not reopened.

- Closes #39 — go-ini truncates config and credentials values at `#` and `;`
- Closes #40 — `import` re-aligns every key in the credentials file
- Closes #41 — Remove the unused docker ecosystem from dependabot.yml
- Closes #42 — Pin GitHub Actions to commit SHAs
- Closes #43 — Generate release notes instead of a fixed text
- Closes #44 — Narrow the "no access" match in isSSORoleNoAccessStderr
- Closes #45 — Use `go 1.27` with a `toolchain` directive in go.mod
