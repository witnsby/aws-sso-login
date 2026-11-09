# Design

## Context

- All user-facing text other than stdout payloads goes through logrus with its default `TextFormatter`. On a terminal this prints `INFO[0014]`; when redirected it prints `time=… level=… msg=…`. `configureLogging` in `cli.go` sets Info for `console`/`import` and Warn for `process`/`export`.
- `reportError` in `cli.go` is the single place that renders a failure, and `Run` always calls `os.Exit(1)`. Only `errSSORoleNoAccess` gets a special message, and that message also appends the raw error.
- Errors are plain wrapped errors. `errSSORoleNoAccess` and `errAWSCLINotFound` are sentinels. A missing `--profile` is `errors.New(helper.ErrorProfileSpecification)`, and a cancelled picker is `errors.New("profile selection cancelled")`.
- `ssoLogin` runs `aws sso login` with stdin set to the terminal and both output streams set to `os.Stderr`. `stderrOut` is already a seam for non-log messages (the browser fallback). `isInteractive` already uses `go-isatty`.
- `profiles.Profile` already carries `AccountID` and `RoleName` for both legacy and sso-session profiles, so the unassigned-role message needs no new config parsing.

## Goals / Non-Goals

**Goals:**
- One small reporter in `src/internal/cli` that owns status line rendering, so call sites never format prefixes or colors themselves.
- Failure class, title, detail and next step attached to the error where the facts are known, and rendered in one place.
- Exit codes decided by the error alone, so tests can check them without running the binary.

**Non-Goals:**
- Replacing logrus. It stays for debug output.
- A general-purpose UI or TUI package, spinners, progress bars or a new dependency.

## Decisions

### 1. A `reporter` for status lines, separate from logrus

A `reporter` value (new file `src/internal/cli/status.go`) with `Step`, `Success` and `Warn` methods, plus an error block renderer used by `reportError`. It writes to the `stderrOut` seam. It is set up once in `PersistentPreRunE`, next to `configureLogging`, from three inputs:
- whether stderr is a terminal: a new seam `stderrIsTerminal` using `isatty.IsTerminal(os.Stderr.Fd())`;
- `NO_COLOR` and `TERM`, read with `os.Getenv`;
- the command name: `process` and `export` get a quiet reporter that drops `Step` and `Success`.

Its styles are two fixed prefix sets, symbols (`→ ✓ ! ✗`) and ASCII (`-> ok: warning: error:`), plus an on/off switch for ANSI colors.

*Alternative considered:* a custom `logrus.Formatter` that maps levels to prefixes. Rejected for two reasons. logrus has no "success" level. And mixing debug logs and status lines in one stream would make it impossible to keep timestamps on debug logs only.

### 2. A `userError` type carries the failure class and wording

```go
type userError struct {
    title, detail, hint string // detail may hold up to two lines
    code  int                   // exit status
    err   error                 // wrapped cause, returned by Unwrap
}
```

Each error is built with a constructor where its facts are known:
- `retrieveCredentials` wraps `errSSORoleNoAccess` into a code 4 error, because it already has the resolved `profiles.Profile` with the account ID and role name;
- `ssoLogin` builds "login failed" with code 3;
- the non-login refresh failure builds "not signed in" with code 3;
- `awsCommand` / `mapAWSExecError` build the missing AWS CLI error with code 5;
- `resolveProfileName` builds "missing `--profile`" with code 2;
- `HuhSelector.Pick` builds "cancelled" with code 130.

Because `Unwrap` returns the cause, existing `errors.Is(err, errSSORoleNoAccess)` checks keep working. Every other error has no `userError` in its chain and becomes a code 1 block whose title is `err.Error()`, with no hint.

*Alternative considered:* a mapping table in `reportError` from sentinels to messages. Rejected because `reportError` does not have the profile, so it could not name the account and role.

### 3. `Run` returns an exit status; `os.Exit` goes behind a seam

`Run` calls `newRootCmd().ExecuteC()`, so it knows which command failed, even when the failure happened during flag parsing before `PersistentPreRunE`. It calls `reportError(cmd, err)`, which renders the block and returns the code, and then calls a new `exit` seam (`os.Exit` in production). For `process`, the block is folded into one line (decision 1).

Usage errors (code 2) are created at three points:
- `rootCmd.SetFlagErrorFunc` wraps flag parse errors;
- the subcommands get `Args: cobra.NoArgs`, wrapped so the error becomes a usage `userError`;
- an unknown subcommand is detected in `Run` from the returned command being the root with leftover arguments.

Each usage error carries the hint `Run 'aws-sso-login <cmd> --help'`.

### 4. logrus is configured for debug output only

`configureLogging` sets:
- with `--debug` / `AWS_SSO_LOGIN_DEBUG`: `DebugLevel` and `TextFormatter{FullTimestamp: true}`;
- otherwise: `WarnLevel`. No code logs at Warn or above any more after this change, so logrus stays silent in practice.

The current `logrus.Infof` calls change as follows:
- the login notice becomes `report.Step`;
- the refresh notice becomes `report.Step("Refreshed credentials for [dev]")`, and the caller ARN goes to `logrus.Debug`;
- the import result becomes `report.Success`.

After an error block, `reportError` writes `logrus.Debugf("error chain: %+v", err)`. That is visible only with debug enabled.

### 5. `aws sso login` stays attached directly to the terminal

`ssoLogin` keeps `cmd.Stdout = os.Stderr` and `cmd.Stderr = os.Stderr`. The reporter prints `→` before `cmd.Run()` and `✓` after it succeeds; a failure becomes the code 3 `userError`. Piping the AWS CLI's output through Go to add prefixes was rejected. The AWS CLI is a Python program, and Python block-buffers stdout when it is not a TTY, so the device URL and verification code could appear only after the user had already needed them.

### 6. The import success line

`importCreds` builds the line from:
- the profile name;
- `displayPath(path)`: replaces a leading `$HOME/` with `~/`, and leaves other paths untouched;
- the expiry, from the existing `parseExpirationTime`, converted with `.In(now().Location())`.

The remaining time is `exp.Sub(now()).Truncate(time.Minute)`, formatted with the trailing `0s` removed (`8h0m`, `45m`). The date (`2006-01-02`) is added when the expiry falls on a different local day than `now()`. Taking the location from the `now` seam lets tests pin both the clock and the time zone.

### 7. Interaction with AWS CLI files

None of the AWS CLI's files change: no new reads or writes of `~/.aws/config`, `~/.aws/credentials` or `~/.aws/cli/cache`, and no change to the cache key. Account and role names come from the already resolved profile, which honors `AWS_CONFIG_FILE`. The displayed credentials path comes from `helper.AwsCredentialsPath`, so `AWS_SHARED_CREDENTIALS_FILE` is shown as configured.

### 8. Platform notes (macOS, Linux)

- `go-isatty` behaves the same on both platforms.
- Both macOS Terminal/iTerm2 and common Linux terminals render UTF-8 symbols and basic ANSI SGR colors (`\x1b[2m`, `32m`, `33m`, `31m`, reset `0m`).
- No process, path or permission behavior changes. Builds stay `CGO_ENABLED=0`.

### 9. Testability

Tests replace these seams:
- `stderrOut` with a `bytes.Buffer`;
- `stderrIsTerminal` with a function returning true or false;
- `exit` with a recorder;
- `now` with a fixed time in a chosen location.

The environment is set with `t.Setenv`. AWS CLI behavior (refresh failures with `ForbiddenException`, expired sessions, `aws sso login` failure, `aws` missing from `PATH`) is scripted with the existing fake command runner in `testhelpers_test.go`. A table test over `reportError` checks the exit code and rendered block for each error class. Command-level tests run `newRootCmd()` with arguments to cover usage errors.

## Risks / Trade-offs

- [Scripts that check `$? -eq 1`] → Marked **BREAKING** in `CHANGELOG.md` and the README. Every failure still exits non-zero, so `if !` / `|| …` checks keep working.
- [A terminal with a non-UTF-8 locale shows garbled symbols] → Rare on supported platforms. Redirected output uses ASCII. Locale detection is listed as an open question, not built now.
- [The cobra unknown-command detection depends on cobra internals] → Covered by a command-level test, so a cobra upgrade that changes it fails CI.
- [Hiding the raw error chain makes bug reports less detailed] → The chain is one `--debug` away, and the README tells users to include `--debug` output in issues.
- [Exit code 130 for a cancelled picker can be mistaken for SIGINT] → Intended. It matches shell conventions for user cancellation.

## Migration Plan

1. Ship in the next minor release with a **BREAKING** entry under `Changed` in `CHANGELOG.md` that lists the exit codes.
2. Update the project context in `openspec/config.yaml` ("Every failure exits with status 1") to the new exit code table, in the same change.
3. Rollback: revert the change. No state or files are migrated.

## Open Questions

- Should the tool also fall back to ASCII prefixes on a terminal whose locale is not UTF-8 (`LC_ALL` / `LC_CTYPE` / `LANG` without `UTF-8`)? This can be added to the style detection later without changing specs for UTF-8 users.
