# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Changed

- Releases are published automatically after a green merge to `main`: the
  version follows the Conventional Commits since the last tag, the release
  description lists breaking changes, features, fixes, improvements, merged
  pull requests and closed issues, and the Homebrew tap is updated right away.

## [v0.3.0] - 2026-11-09

### Added

- The interactive profile picker filters as you type: it opens with the
  filter input focused and keeps the profiles whose name, account ID or role
  name contains the typed text, ignoring case. `Esc` clears the filter. The
  picker is at most 15 lines tall and scrolls through longer lists.

### Changed

- **BREAKING**: failures now exit with a status that names the failure
  class instead of always `1`: `2` usage error, `3` not signed in to AWS SSO
  or `aws sso login` failed, `4` SSO role not assigned, `5` AWS CLI not
  found, `130` profile picker cancelled; `1` remains for everything else.
  Scripts that only check for a non-zero status are not affected.
- Errors are reported as one block with a title, details and a `Fix:` line
  with the next step instead of a raw log line; `process` prints the same
  information on one line. The underlying error chain is shown with
  `--debug`.
- Positional arguments are rejected with a usage error; they were silently
  ignored before.
- Progress and results are shown as status lines on stderr (`→` step, `✓`
  success, `!` warning, `✗` error; ASCII prefixes when stderr is not a
  terminal, no colors with `NO_COLOR` or `TERM=dumb`) instead of logrus lines
  such as `INFO[0014]`. Logrus output is limited to `--debug`, now with full
  timestamps.
- `import` ends with a success line that also says when the credentials
  expire; `console` ends with a success line naming the profile and region.
  `aws sso login` is announced before it starts and confirmed after it
  succeeds.
- The refresh notice no longer prints the caller ARN; it is shown with
  `--debug`.

## [v0.2.1] - 2026-11-09

### Fixed

- Values in `~/.aws/config` and `~/.aws/credentials` are now read the way the
  AWS CLI reads them: `#` and `;` inside a value and surrounding double
  quotes are kept. `import` no longer truncates or unquotes values of other
  profiles in the credentials file, and legacy profiles whose
  `sso_start_url` ends in `/#` find their cached credentials again. (#39)
- `import` no longer re-aligns the keys of profiles it does not update; the
  credentials file is written with plain `key = value` lines. (#40)
- Only AWS `ForbiddenException` and `AccessDeniedException` errors are
  reported as an unassigned SSO role. Other errors that merely mention "no
  access" now lead to `aws sso login` (`console`, `import`) or to the
  `aws sso login` advice (`export`, `process`). (#44)

### Changed

- `go.mod` declares `go 1.27` as the minimum and pins `toolchain go1.27.3`
  for CI and release builds; CI fails when the active toolchain differs.
  (#45)
- GitHub Actions in the CI and release workflows are pinned to full commit
  SHAs, and CI fails on tag or branch references (`make check-action-pins`).
  (#42)
- GitHub release descriptions are taken from the tag's section in
  `CHANGELOG.md` (`make release-notes`); the release workflow fails before
  publishing when that section is missing. (#43)
- Dependabot no longer configures the non-existent `docker` ecosystem and
  writes Conventional Commit messages (`chore(deps): ...`). (#41)

## [v0.2.0] - 2026-11-09

### Added

- `--profile` is accepted by every command and defaults to `AWS_PROFILE`.
  `console` and `export` now offer the interactive picker too, drawn on
  stderr, when stdin and stderr are terminals; `process` never prompts.
  (#32)
- A persistent `--debug` flag and the `AWS_SSO_LOGIN_DEBUG` environment
  variable enable debug logs, including raw AWS CLI stderr. (#31)

### Fixed

- Profiles that reference an `[sso-session <name>]` section (the default
  format of `aws configure sso`) are now supported by every command and
  listed in the interactive picker. Their cached credentials are found
  with the same cache key the AWS CLI uses (`sessionName` instead of
  `startUrl`). (#23)
- The `default` profile now works with `--profile default` and from the
  picker; it is read from `[default]`, falling back to `[profile default]`.
  (#22)
- Cached credentials that expire within 5 minutes are now refreshed instead
  of being returned by `export`, `import`, `process` and `console`. (#27)
- A missing AWS CLI is now reported as `AWS CLI v2 is required but "aws" was
  not found in PATH` instead of advice to run `aws sso login`. (#29)
- `import` and `console` now show the output of `aws sso login` (the
  authorization URL and verification code) on stderr, so login works when no
  browser opens automatically. (#25)
- `console --force-logout=false` no longer opens the logout page;
  `--logout-wait` is only the delay after a logout. (#24)
- The federation sign-in request used by `console` now times out, reports
  non-200 responses and empty tokens clearly, and never includes the request
  URL (which carries credentials) in errors. (#28)
- `console` picks the browser launcher by OS (`open` on macOS, `xdg-open` on
  Linux) and prints the URL to stderr when none is available, instead of
  trying `open` on Linux. (#30)
- `export` now escapes `$`, `` ` ``, `"` and `\` inside its double-quoted
  values, so `eval` assigns them verbatim. Output for ordinary credential
  values is unchanged. (#33)

### Changed

- `process` and `export` log only warnings and errors by default, every
  error is reported once, and the "Updated credentials" notice moved from
  stdout to stderr. (#31)
- `console` opens the console in the profile's `region` when it is set,
  falling back to `sso_region`. (#35)
- Go 1.27.3 (the first 1.27 release without known standard-library
  vulnerabilities reachable from this tool) is required to build from source; cobra, logrus, testify and
  go-isatty are updated. (#34)
- CI runs on pull requests and pushes to `main` with `go vet`, a `gofmt`
  check, tests with `-race` and `govulncheck`. Releases publish
  `aws-sso-login_checksums.txt` (SHA-256), use the workflow token and fail on
  real upload errors. The Makefile gains `clean`, `vet`, `fmt-check` and
  `checksums` targets and stops at the first failing command. (#34)

### Security

- `import` now writes the credentials file atomically (temp file plus rename)
  with mode `0600`, tightening existing files with looser permissions, and
  creates a missing directory with mode `0700`. It honors
  `AWS_SHARED_CREDENTIALS_FILE`. (#26)

## [v0.1.1] - 2026-05-06

### Fixed

- `import` and `console` no longer enter an infinite `aws sso login` retry loop
  when the configured SSO role is not assigned to the user. The CLI now detects
  `ForbiddenException` / `AccessDeniedException` from `GetRoleCredentials` and
  exits immediately with a single, actionable error message.
- Removed duplicate error rendering on failure: cobra's `Error:` line and
  `Usage:` block are no longer printed for runtime errors, and the trailing
  `FATA[…]` line is gone. Failures now produce exactly one log line.

## [v0.1.0] - 2026-05-06

### Added

- Interactive profile picker for `import` when `--profile` is omitted (charmbracelet/huh).

[Unreleased]: https://github.com/witnsby/aws-sso-login/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/witnsby/aws-sso-login/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/witnsby/aws-sso-login/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/witnsby/aws-sso-login/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/witnsby/aws-sso-login/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/witnsby/aws-sso-login/compare/v0.0.8...v0.1.0
