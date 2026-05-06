# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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

[Unreleased]: https://github.com/witnsby/aws-sso-login/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/witnsby/aws-sso-login/compare/v0.0.8...v0.1.0
