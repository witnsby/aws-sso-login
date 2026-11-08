# Proposal

## Why

A review of the codebase filed 14 open issues (#22–#35). Together they block a large share of users: profiles in the `sso-session` format that `aws configure sso` writes by default do not work at all, and neither does the `[default]` profile. Credentials are also written world-readable, and several failure modes hang or mislead the user. The fixes touch the same few files (`common.go`, `cli.go`, `profiles.go`), so they are planned together as one change and landed as one commit per logical fix.

## What Changes

**Profile resolution (#22, #23, #32)**
- One resolver maps profile names to config sections for every command and for the picker. `default` resolves to `[default]` (falling back to `[profile default]`).
- Profiles that reference `[sso-session <name>]` are supported: `sso_start_url` and `sso_region` are read from the session section, and the profiles appear in the picker.
- `--profile` becomes a persistent flag on the root command. When it is empty, `AWS_PROFILE` is used. After that, `console`, `export` and `import` show the interactive picker when stdin and stderr are terminals. `process` never prompts.

**Credential retrieval (#23, #25, #27, #29)**
- The AWS CLI cache key uses `sessionName` instead of `startUrl` for sso-session profiles, matching the AWS CLI v2 algorithm.
- Cached credentials that expire within 5 minutes count as expired and are refreshed.
- A missing `aws` executable produces `AWS CLI v2 is required but "aws" was not found in PATH` instead of advice to run `aws sso login`.
- While `aws sso login` runs, its output (authorization URL and code) is shown on stderr.

**Credentials file (#26)**
- `import` honors `AWS_SHARED_CREDENTIALS_FILE` and creates a missing parent directory with mode `0700`. It writes the file atomically (temp file plus rename) with mode `0600`, which also tightens existing files that had looser permissions.

**Console sign-in (#24, #28, #30, #35)**
- `--force-logout=false` no longer logs out. `--logout-wait` is used only as the delay after a logout.
- The federation token request has a timeout, rejects non-200 responses and empty tokens, and never includes the request URL (which contains credentials) in errors.
- The browser command is chosen by OS (`open` on macOS, `xdg-open` on Linux). When no command is available, the URL is printed to stderr.
- The console opens in the profile's `region` when it is set, otherwise in `sso_region`.

**Logging and output (#31, #35)**
- Each error is reported once, by `reportAndExit`.
- A persistent `--debug` flag and the `AWS_SSO_LOGIN_DEBUG` environment variable enable debug logs.
- `process` and `export` log at warn level by default. `import` and `console` keep info level.
- `version` no longer prints a trailing blank line. The "Updated credentials for" message moves from stdout to stderr.

**Testability and code quality (#33, #35)**
- The AWS CLI runner, home directory, clock, HTTP client and federation endpoint move behind package-level seams. Table-driven tests cover cache keys, expiry, the login retry flow, credentials-file writing, `export` and `process` output, and console sign-in.
- Doc comments are fixed, `ErrorPofileSpecification` becomes `ErrorProfileSpecification`, the config-path helper moves next to the other path helpers, flag-read errors are handled, and the `silent` parameter is removed. `awsCredentialsManager` is split by responsibility.

**Build, CI and release (#34)**
- Go is upgraded to a supported release (1.27) and workflows read the version from `go.mod`. Direct dependencies are updated.
- CI runs on pull requests and on pushes to `main`. It runs `go vet`, a `gofmt -l` check, tests with `-race`, and `govulncheck`.
- Releases publish SHA-256 checksums, use `GITHUB_TOKEN` with `contents: write`, and fail on real `gh` errors.
- The Makefile gains a working `clean` target, real error handling, and quiet `git describe`. `CHANGELOG.md` gets the missing `v0.1.1` entry.

No change is **BREAKING**: flag names, defaults, `export` and `process` stdout formats, and exit codes stay the same. The new flags (`--debug`, persistent `--profile`) and the `AWS_PROFILE` fallback are additive. Commands that previously failed without `--profile` now fall back to `AWS_PROFILE` or the picker.

### User-visible impact per command

- **console**: works with `[default]` and sso-session profiles and accepts `AWS_PROFILE`. Offers the picker on a TTY. Honors `--force-logout=false`. Shows `aws sso login` instructions. Opens the profile's `region`. Fails fast on network problems. On Linux, opens the browser with `xdg-open`, or prints the URL to stderr when that is unavailable.
- **export**: works with `[default]` and sso-session profiles and accepts `AWS_PROFILE`. Offers the picker on a TTY (rendered on stderr, so `eval "$(aws-sso-login export)"` stays safe). Never returns credentials with less than 5 minutes left. Stdout format is unchanged and stderr is quieter by default.
- **import**: works with `[default]` and sso-session profiles and accepts `AWS_PROFILE`. Writes the credentials file with mode `0600`, atomically, to `AWS_SHARED_CREDENTIALS_FILE` when set. Shows `aws sso login` instructions. Requires `--profile` or `AWS_PROFILE` when not on a TTY.
- **process**: works with `[default]` and sso-session profiles and accepts `AWS_PROFILE`. Never prompts. Never returns credentials with less than 5 minutes left. Stdout JSON is unchanged and stderr is silent unless there is a warning or `--debug` is set.
- **version**: no trailing blank line.

### Out of scope

- A user-configurable expiry margin (the margin is a code constant, which tests can override).
- `golangci-lint` and GoReleaser (both optional in #34). They can follow in their own change.
- Windows support beyond keeping `rundll32` as the Windows browser command.
- Running `aws sso login` automatically from `export` or `process`.
- Changing the federation endpoint, session duration, or the `credential_process` JSON version.

## Related issues

- Closes #22 — `import` picker lists the [default] profile, but importing it fails
- Closes #23 — Profiles using `sso_session` are not supported
- Closes #24 — `console --force-logout=false` still logs out
- Closes #25 — Output of `aws sso login` is hidden from the user
- Closes #26 — `import` writes ~/.aws/credentials world-readable and non-atomically
- Closes #27 — Credentials are returned right up to their expiration time
- Closes #28 — Federation sign-in request has no timeout and no response validation
- Closes #29 — Misleading error when the AWS CLI is not installed
- Closes #30 — `openBrowser` picks the wrong command on Linux
- Closes #31 — Logging: duplicate error messages and no way to enable debug output
- Closes #32 — Consistent `--profile` handling across commands
- Closes #33 — Add unit tests for credential retrieval, caching and output
- Closes #34 — Harden build, CI and release pipeline
- Closes #35 — Code-quality cleanups

Dependabot PR #18 (cobra 1.8.1 → 1.9.1) is superseded by the dependency update in #34.

## Capabilities

### New Capabilities

- `profile-resolution`: resolving a profile name (flag, `AWS_PROFILE`, picker) to a validated SSO configuration from the AWS config file, for legacy and sso-session profiles.
- `credential-retrieval`: obtaining role credentials from the AWS CLI cache, refreshing them through the AWS CLI, the expiry margin, and login and failure handling.
- `credentials-file`: how `import` writes credentials to the shared credentials file.
- `console-sign-in`: federation sign-in, logout handling, region choice and opening the browser for `console`.
- `cli-output`: stdout and stderr contracts, logging levels, debug switch and error reporting shared by all commands.
- `build-release`: CI checks, toolchain, and release artifacts.

### Modified Capabilities

None (no specs exist yet).

## Impact

- **Code**: `src/internal/cli/*` (all commands, `common.go` split into smaller files), `src/internal/profiles` (section resolution, sso-session), `src/internal/helper` (path helpers, constants), `src/internal/model`.
- **Dependencies**: no new third-party modules. Existing direct modules (cobra, logrus, huh, go-ini, testify) are bumped. `govulncheck` runs in CI through `go run` at a pinned version and is not added to `go.mod`.
- **Files read and written**: `~/.aws/config` (or `AWS_CONFIG_FILE`), `~/.aws/credentials` (or `AWS_SHARED_CREDENTIALS_FILE`), `~/.aws/cli/cache`. All formats stay compatible with AWS CLI v2.
- **CI and release**: `.github/workflows/build_release.yaml`, `.github/workflows/publish_release.yml`, `Makefile`, `go.mod`, `go.sum`.
- **Docs**: `README.md`, `CHANGELOG.md` (`Unreleased` plus the missing `v0.1.1`), `AGENTS.md` (helper names).
