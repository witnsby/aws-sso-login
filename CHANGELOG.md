# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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
- `import` and `console` no longer enter an infinite `aws sso login` retry loop
  when the configured SSO role is not assigned to the user. The CLI now detects
  `ForbiddenException` / `AccessDeniedException` from `GetRoleCredentials` and
  exits immediately with a single, actionable error message.
- Removed duplicate error rendering on failure: cobra's `Error:` line and
  `Usage:` block are no longer printed for runtime errors, and the trailing
  `FATA[…]` line is gone. Failures now produce exactly one log line.

### Security

- `import` now writes the credentials file atomically (temp file plus rename)
  with mode `0600`, tightening existing files with looser permissions, and
  creates a missing directory with mode `0700`. It honors
  `AWS_SHARED_CREDENTIALS_FILE`. (#26)

## [v0.1.0] - 2026-05-06

### Added

- Interactive profile picker for `import` when `--profile` is omitted (charmbracelet/huh).

[Unreleased]: https://github.com/witnsby/aws-sso-login/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/witnsby/aws-sso-login/compare/v0.0.8...v0.1.0
