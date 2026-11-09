# Proposal

## Why

User-facing output today is raw logrus text: lines start with `INFO[0014]` / `ERRO[0032]` (seconds since start), progress and results look the same, and errors repeat themselves ("No access: … (no access to the configured SSO role …)") without saying which account or role is affected or what to do next. Every failure also exits with status 1, so scripts cannot tell "not signed in" from "no access" from "AWS CLI missing". The output should read as a clear status report for people and give scripts a stable signal.

## What Changes

- Human-readable status on stderr in four line kinds: step (`→`), success (`✓`), warning (`!`), error (`✗`). Symbols and color are used only when stderr is a terminal; color is also disabled by `NO_COLOR` or `TERM=dumb`. Without a terminal the prefixes are plain ASCII (`->`, `ok:`, `warning:`, `error:`).
- The logrus `LEVEL[seconds]` format disappears from normal output. logrus is used only for debug output (`--debug` / `AWS_SSO_LOGIN_DEBUG`), with full timestamps.
- `import` and `console` end with one success line. For `import` it names the profile, the credentials file path (home shortened to `~`) and when the credentials expire (local time and remaining duration). For `console` it names the profile and the region the console opens in.
- Around `aws sso login`, a step line is printed before it starts and a success or error line after it ends. The AWS CLI keeps writing straight to the terminal, as it does now, so the verification code shows up immediately.
- The refresh notice no longer prints the caller ARN (it contains the account ID) at the default level; the ARN moves to debug output.
- Errors are rendered once as a block: a title line, up to two detail lines and a `Fix:` line with the next step. The "role not assigned" error names the account ID and role name from the profile. The raw wrapped error chain is shown only with debug enabled. For `process`, the block is folded into one line, because SDKs embed `credential_process` stderr in their own error messages.
- **BREAKING**: exit codes become specific: `0` success, `1` other error, `2` usage error (bad flags or arguments, missing `--profile` where no picker is allowed), `3` not signed in or `aws sso login` failed, `4` SSO role not assigned, `5` AWS CLI not found, `130` profile picker cancelled. Before this change, every failure exited with `1`.
- Affected commands:
  - `console`: new step and success lines; new error layout; new exit codes.
  - `import`: new step and success lines (including expiry); new error layout; new exit codes.
  - `export`: stdout unchanged and still `eval`-safe; stderr stays empty on success; new error layout and exit codes.
  - `process`: stdout unchanged (only the `credential_process` JSON); stderr stays empty on success; errors become one `error:`/`✗` line; new exit codes.
  - `version`: unchanged.

### Out of scope

- Machine-readable output for people-facing commands (`--output json`).
- A quiet mode for `import` and `console` (for example `--quiet`).
- Translating or localizing messages.
- Changing how `aws sso login` itself prints the device URL and code, or reformatting its output.
- Windows terminal color handling beyond the generic `NO_COLOR` / TTY rules.
- The manual-open fallback in `console` that prints the sign-in URL when no browser can be started.

## Capabilities

### New Capabilities

None. All behavior belongs to existing capabilities.

### Modified Capabilities

- `cli-output`: adds the status line format, the error block layout, the exit code table and the success summary; replaces the "exit with status 1" rule; switches default progress output from logrus to status lines and adds full timestamps and the raw error chain to debug output.
- `credential-retrieval`: login, expired SSO session, missing AWS CLI and unassigned-role failures get their own exit codes and next-step hints; the unassigned-role message names the account and role; the login is framed by status lines.
- `profile-resolution`: a missing `--profile` where no picker is allowed exits with `2`; a cancelled picker exits with `130`.
- `credentials-file`: the `import` result line also reports when the credentials expire.

## Impact

- Code: `src/internal/cli/cli.go` (`configureLogging`, `reportError`, `Run`), a new status reporter and user error type in `src/internal/cli/`, call sites in `credentials.go`, `import.go`, `console.go`, `profile.go`, `selector.go`; new seams for the stderr terminal check, the environment and `os.Exit`.
- Public API: exit codes (**BREAKING** for scripts that compare against `1`); stderr wording and layout (not a contract, but visible). Stdout contracts of `export`, `process` and `version` do not change.
- `openspec/config.yaml` project context: "Every failure exits with status 1" must be replaced by the new exit code table.
- Dependencies: none new. `github.com/mattn/go-isatty` is already a dependency; colors are plain ANSI escapes.
- Docs: `README.md` (output and exit codes section), `CHANGELOG.md` `Unreleased`.

## Related issues

None open. Searched `gh issue list --state all` for: "error message", "output format", "exit code", "stderr", "no access", "log". Closed predecessors #31 (duplicate error messages, debug switch) and #25 (hidden `aws sso login` output) are context only and are not referenced in commits.
