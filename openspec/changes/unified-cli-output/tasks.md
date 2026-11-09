# Tasks

No GitHub issues are related (see proposal.md, Related issues), so commits carry no `Refs`/`Closes` references.

## 1. Status line reporter

- [x] 1.1 Add the `stderrIsTerminal` seam to `src/internal/cli/seams.go` (`isatty.IsTerminal(os.Stderr.Fd())`) and verify `go build ./...` succeeds
- [x] 1.2 Add `src/internal/cli/status.go` with the `reporter` (`Step`, `Success`, `Warn`), the symbol and ASCII prefix sets, ANSI color on/off and a quiet mode that drops `Step`/`Success` (design decisions 1 and 8); verify with `status_test.go` table tests for: terminal + color, terminal + `NO_COLOR`, terminal + `TERM=dumb`, non-terminal (no ANSI escapes, `-> ok: warning: error:`), and quiet mode
- [x] 1.3 Build the reporter in `PersistentPreRunE` from the command name, `stderrIsTerminal`, `NO_COLOR` and `TERM` (quiet for `process` and `export`); verify with a test that `process` and `export` get a quiet reporter and `import`/`console` do not
- [x] 1.4 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate unified-cli-output --strict`, tick 1.1–1.4 and commit `feat(cli): add status line reporter`

## 2. Error blocks and exit codes

- [x] 2.1 Add the `userError` type (title, up to two detail lines, hint, code, wrapped cause with `Unwrap`) and its constructors in `src/internal/cli/errors.go`; verify with tests that `errors.Is(err, errSSORoleNoAccess)` still holds through a wrapped `userError`
- [x] 2.2 Rewrite `reportError(cmd, err) int` to render the error block (title with error prefix, two-space-indented detail and `Fix:` lines, no repeated raw error), the one-line form for `process`, and a `logrus.Debugf` line with the full error chain; return `userError.code` or 1; verify with a table test over `reportError` for each error class, for both the block form and the `process` one-line form
- [x] 2.3 Add the `exit` seam, switch `Run` to `ExecuteC` and `exit(reportError(cmd, err))`; verify with a test that a failing command calls the `exit` seam with the returned code and that success never calls it
- [x] 2.4 Make usage errors exit with 2: wrap flag errors via `SetFlagErrorFunc`, set wrapped `cobra.NoArgs` on subcommands, detect unknown subcommands in `Run`, each with the hint `Run 'aws-sso-login <cmd> --help'`; verify with `newRootCmd()` tests for `import --no-such-flag`, `import extra-arg` and `no-such-command`, each giving exit 2 and one error block
- [x] 2.5 Replace "Every failure exits with status 1" in the `context` of `openspec/config.yaml` with the exit code table from the `cli-output` delta, and verify `openspec validate --all --strict` passes
- [x] 2.6 Document the exit codes in the `Error handling` section of `README.md`, and add a **BREAKING** `Changed` entry listing them under `## [Unreleased]` in `CHANGELOG.md`; verify the README table matches the `Exit codes` requirement
- [x] 2.7 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate unified-cli-output --strict`, tick 2.1–2.7 and commit `feat(cli)!: render errors as blocks with specific exit codes`

## 3. Classify credential and profile failures

- [x] 3.1 In `retrieveCredentials`, wrap `errSSORoleNoAccess` into a code 4 `userError` that names the profile, `AccountID` and `RoleName`, with the hint to ask an administrator or choose another profile; verify with tests for a legacy profile and an sso-session profile (`123456789012`, `ReadOnly`) on `console`, `import`, `export` and `process`, and that no `aws sso login` runs
- [x] 3.2 Turn the missing AWS CLI error into a code 5 `userError` whose hint points to the AWS CLI v2 install guide; verify with an `import --profile dev` test where `lookPath` fails: exit 5, no login attempted
- [x] 3.3 Turn the non-login refresh failure (`export`, `process`) into a code 3 "not signed in" `userError` with the hint `aws sso login --profile <name>`, including the unrelated "no access" proxy case; verify with the existing expired-token and proxy-error tests updated to expect exit 3
- [x] 3.4 Turn the `aws sso login` failure into a code 3 `userError` with the hint to check the AWS CLI output above and try again; verify with `login_test.go` that a failing login gives exit 3 and runs only once
- [x] 3.5 Turn the missing `--profile` error in `resolveProfileName` into a code 2 `userError` with the hint naming `--profile <name>` and `AWS_PROFILE`, and the picker cancellation in `HuhSelector.Pick` into a code 130 `userError`; update `profileflag_test.go` to check the codes for the non-interactive, `process` and cancelled-picker paths
- [x] 3.6 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate unified-cli-output --strict`, tick 3.1–3.6 and commit `feat(cli): classify credential and profile failures`

## 4. Progress and results as status lines

- [x] 4.1 Change `configureLogging` to debug-only logrus: `DebugLevel` with `TextFormatter{FullTimestamp: true}` when debug is on, `WarnLevel` otherwise; verify with `logging_test.go` that debug lines carry a full date and time and that no `LEVEL[` marker appears without debug
- [x] 4.2 Replace the `logrus.Infof` refresh notice with `report.Step("Refreshed credentials for [<profile>]")` and move the caller ARN to `logrus.Debug`; verify that `import` without debug shows the step line and neither the ARN nor the account ID, and that `--debug` shows the ARN
- [x] 4.3 Frame `aws sso login` with a step line before `cmd.Run()` and a success line after it, keeping `cmd.Stdout`/`cmd.Stderr` set to `os.Stderr`; verify with `login_test.go` that the step line comes before the fake AWS CLI output and the success line after it
- [x] 4.4 Add `displayPath` (leading `$HOME/` becomes `~/`) and expiry formatting (local time from `now().Location()`, remaining time truncated to minutes without `0s`, date added on another day), and end `import` with the success line; verify with tests for `18:42` + `8h0m`, the next-day date, `~/.aws/credentials`, and `AWS_SHARED_CREDENTIALS_FILE=/tmp/creds` shown in full, and that no credential value appears in the output
- [x] 4.5 End `console` with a success line naming the profile and the console region; verify with `console_test.go`, including the picker path, that it is the last stderr line and stdout stays empty
- [x] 4.6 Verify `export` and `process` print nothing to stderr on success, with a test for each
- [x] 4.7 Rewrite the `Logging` section of `README.md` with sample `import` output (success and unassigned role) using placeholders `123456789012` and `https://example.awsapps.com/start`, and note the terminal/ASCII/`NO_COLOR` rules; add an `Unreleased` `Changed` entry to `CHANGELOG.md` for the new status lines and the import expiry; verify the README samples match the test fixtures' wording
- [x] 4.8 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate unified-cli-output --strict`, tick 4.1–4.8 and commit `feat(cli): report progress and results as status lines`

## 5. Verification

- [ ] 5.1 Run `make tests`, `go vet ./...`, `gofmt -l ./src` (no output) and `openspec validate unified-cli-output --strict`, and confirm all pass
- [ ] 5.2 Run `make build` and smoke-test `./tmp/aws-sso-login` against a temp `AWS_CONFIG_FILE` and `HOME`, with no AWS calls: `process --profile missing` exits 1; `process` with no profile exits 2 with a single `error:` line; `import --no-such-flag` exits 2; `import --profile dev` with `PATH` lacking `aws` exits 5; and with `2>&1 | cat` there are no ANSI escapes
- [ ] 5.3 Run `openspec validate --all --strict` and confirm it passes before opening the PR

## 6. Pull request

- [ ] 6.1 Open a PR from this branch to `main` whose description links the OpenSpec change `unified-cli-output`, calls out the **BREAKING** exit codes, and states that no related issues exist (no `Closes`/`Refs` lines); verify the PR description renders the exit code table

## Workflow follow-up

- After merge, archive the change with `openspec-archive-change`, confirming that `README.md` and `CHANGELOG.md` match the archived behavior and running `openspec validate --all --strict` afterwards. Commit the archive on its own.
