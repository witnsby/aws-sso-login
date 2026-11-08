# Tasks

Each group is one logical change. It ends with its own commit after `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate resolve-open-issues --strict` pass. Tests never touch the real `~/.aws` and never run the real AWS CLI: they use `t.TempDir()`, `HOME`, `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE` and the seams from design.md §3.

## 1. Test seams and path helpers

- [ ] 1.1 Replace `user.Current()` with `os.UserHomeDir()` in `helper`. Add `AwsConfigPath()` (honors `AWS_CONFIG_FILE`), `AwsCredentialsPath()` (honors `AWS_SHARED_CREDENTIALS_FILE`) and `AwsCliCachePath()`. Remove `cli.GetAwsConfigPath` and update callers. Verify with table tests in `helper/helper_test.go` covering env set and unset with `HOME` pointing at a temp dir.
- [ ] 1.2 Add package-level seams `lookPath`, `now`, `httpClient`, `federationEndpoint`, `isInteractive` and `goos` in `cli` next to `execCommand`, and route existing `exec.Command`, `time.Now` and `http.Get` calls through them. Verify that `go build ./...` succeeds and existing tests still pass.
- [ ] 1.3 Add a reusable fake command runner test helper (records argv, returns scripted stdout, stderr and exit code). Verify with a self-test in `cli/testhelpers_test.go`.
- [ ] 1.4 Commit `refactor: inject home dir, clock, HTTP and command seams` with `Refs #33` and `Refs #35`.

## 2. Single profile resolution with default and sso-session support

- [ ] 2.1 Add `profiles.Resolve(configPath, name)` returning a validated `Profile` (Name, AccountID, RoleName, StartURL, SSORegion, Region, SessionName). It handles `default` → `[default]` then `[profile default]`, `[profile <name>]`, legacy required keys, and `[sso-session <name>]` lookup with missing-section, missing-key and conflict errors. Verify with table tests for every scenario in specs/profile-resolution "Profile section lookup", "Legacy SSO profiles" and "sso-session profiles".
- [ ] 2.2 Reimplement `ListSSOProfiles` on the same helpers so it includes sso-session profiles and `default`, skips invalid and non-SSO sections, and sorts case-insensitively. Verify with a test using a mixed config (legacy, sso-session, non-SSO, default).
- [ ] 2.3 Replace `retrieveProfile` and all `*ini.Section` usage in `cli` with `profiles.Resolve` and `profiles.Profile`. Verify that `import --profile default` works in a test with a temp `AWS_CONFIG_FILE` and a fake runner.
- [ ] 2.4 Document sso-session support and the `default` profile in README.md and add `Fixed` entries to CHANGELOG.md `Unreleased`. Verify the README example config matches the test fixture.
- [ ] 2.5 Commit `fix: resolve default and sso-session profiles in one place` with `Closes #22` and `Refs #23`.

## 3. Cache key for sso-session profiles

- [ ] 3.1 Build the cache key from `profiles.Profile`: `sessionName` when set, otherwise `startUrl`, plus `accountId` and `roleName`. Verify with golden SHA-1 tests for both formats, computed independently with `printf '%s' '<json>' | shasum` and recorded in the test.
- [ ] 3.2 Add cache read tests: valid file, missing file, malformed JSON. Verify they run against a temp `HOME`.
- [ ] 3.3 Commit `fix: use sessionName in AWS CLI cache key for sso-session profiles` with `Closes #23`.

## 4. Expiry margin

- [ ] 4.1 Add `minCredentialValidity = 5 * time.Minute` and `time.RFC3339` parsing, and make `isExpired` use `now()`. Verify with table tests: unparsable, already expired, 2 minutes left, 30 minutes left, and the exact boundary.
- [ ] 4.2 Test the refresh path: near-expiry cache → runner called once → fresh cache returned; refresh still near expiry → "expired" error. Verify with the fake runner and a scripted cache update.
- [ ] 4.3 Add a CHANGELOG `Fixed` entry. Commit `fix: refresh credentials that expire within five minutes` with `Closes #27`.

## 5. Missing AWS CLI

- [ ] 5.1 Add `errAWSCLINotFound` and `requireAWSCLI()` using `lookPath("aws")`, and map `exec.ErrNotFound` from the runner to the same error. Skip the login retry for it. Verify with tests: `lookPath` fails → error text matches the spec and the runner is never called with `sso login`.
- [ ] 5.2 Add a CHANGELOG `Fixed` entry. Commit `fix: report a missing AWS CLI clearly` with `Closes #29`.

## 6. Visible `aws sso login` output

- [ ] 6.1 Run `aws sso login` with stdin from the terminal and stdout and stderr on `os.Stderr`. Verify with a test that the fake runner receives `os.Stdin`, `os.Stderr` and `os.Stderr`, and that stdout stays empty.
- [ ] 6.2 Test the retry flow: expired → login once → success. Login fails → single error, no second login. Role not assigned → no login. Verify with fake runner scripts covering each specs/credential-retrieval login scenario.
- [ ] 6.3 Add a CHANGELOG `Fixed` entry. Commit `fix: show aws sso login instructions on stderr` with `Closes #25`.

## 7. Safe credentials file writes

- [ ] 7.1 Implement `writeCredentialsFile` (design.md §7): follow symlinks, `MkdirAll 0700`, temp file `0600`, write, sync, rename, cleanup on error. Verify with tests for a new file (mode `0600`), an existing `0644` file (becomes `0600`), a missing directory (created `0700`), a symlink target updated, other sections preserved, and a failed write leaving the original intact with no temp files.
- [ ] 7.2 Make `import` use `helper.AwsCredentialsPath()` (env override) and report the path on stderr without credential values. Verify with a test that sets `AWS_SHARED_CREDENTIALS_FILE`.
- [ ] 7.3 Document `AWS_SHARED_CREDENTIALS_FILE` and the `0600` mode in README.md, replacing the known-limitation note, and add a CHANGELOG `Security` entry. Commit `fix: write credentials file atomically with 0600 and honor AWS_SHARED_CREDENTIALS_FILE` with `Closes #26`.

## 8. Console sign-in fixes

- [ ] 8.1 Make logout happen only when `--force-logout` is true, with `--logout-wait` used only as the delay. Verify with a `handleLogout` test using a fake browser opener and a fake sleep for all three spec scenarios.
- [ ] 8.2 Harden the federation request: `httpClient` timeout, status check, `LimitReader`, empty-token check, and errors without the URL. Verify with `httptest.Server` tests for 200 with a token, 400 with HTML, 200 with an empty token, and a hanging handler with a short client timeout. Each asserts the error text contains no URL or credential values.
- [ ] 8.3 Choose the browser command by `goos`, with a stderr fallback when `lookPath` or `Start` fails, and release started processes. Verify with an updated `browser_test.go` covering linux with and without `xdg-open` and darwin with `open`. `open` is never tried on linux.
- [ ] 8.4 Use the profile `region` with fallback to `sso_region` for the console destination and logout pages. Verify with a sign-in URL test for both cases.
- [ ] 8.5 Update README console section and add CHANGELOG `Fixed` and `Changed` entries. Commit `fix: honor --force-logout=false, harden federation request, pick browser by OS` with `Closes #24`, `Closes #28`, `Closes #30` and `Refs #35`.

## 9. Consistent `--profile` handling

- [ ] 9.1 Move `--profile` to root persistent flags and resolve the name once in `PersistentPreRunE`: flag, then `AWS_PROFILE`, then the picker if `isInteractive()` and the command is not `process`, otherwise `must specify --profile`. Verify with table tests across all four commands for flag, env, interactive, non-interactive and cancelled cases.
- [ ] 9.2 Render the picker on stderr (`huh.NewForm(...).WithOutput(os.Stderr)`) and promote `mattn/go-isatty` to a direct dependency. Verify that `go mod tidy` adds no new module and that `selector_test.go` still passes.
- [ ] 9.3 Document `AWS_PROFILE`, the picker for `console` and `export`, and the recommendation to pass `--profile` in `credential_process` in README.md. Add a CHANGELOG `Added` entry. Commit `feat: persistent --profile with AWS_PROFILE fallback and picker for console and export` with `Closes #32`.

## 10. Logging

- [ ] 10.1 Add a persistent `--debug` flag and `AWS_SSO_LOGIN_DEBUG` (`1` or `true`), and set the log level in `PersistentPreRunE` (debug, warn for `process` and `export`, info otherwise). Verify with tests that capture logrus output for each case.
- [ ] 10.2 Remove log-and-return duplicates across `cli` so `reportAndExit` reports each error once, and move the "Updated credentials for" message to an info log on stderr. Verify with a test that a missing profile in `process` yields exactly one stderr line and empty stdout.
- [ ] 10.3 Add a test that runs `export`, `process` and `import` with debug on and fake credentials, and asserts no credential value appears in stderr.
- [ ] 10.4 Update the README logging section (`--debug`, env var, levels) and add a CHANGELOG `Added` entry. Commit `feat: add --debug and report each error once` with `Closes #31`.

## 11. Code-quality cleanups

- [ ] 11.1 Fix the doc comments on the former `awsCredentialsManager` and on `versionCmd`, rename `ErrorPofileSpecification` to `ErrorProfileSpecification`, handle errors from `cmd.Flags().Get*`, and remove the `silent` parameter. Verify that `go vet ./...` is clean and tests pass.
- [ ] 11.2 Remove the trailing newline from `getVersion`. Verify with a test that `version` prints exactly one line.
- [ ] 11.3 Split `common.go` into `credentials.go`, `credsfile.go` and `console.go` (design.md §12) and delete `common.go`. Verify that `go build ./...` succeeds and all tests pass unchanged.
- [ ] 11.4 Update AGENTS.md (helper names, file layout). Commit `refactor: code-quality cleanups and split credentials manager` with `Closes #35`.

## 12. Test coverage

- [ ] 12.1 Add table tests for the `export` output format (with and without region, eval-safe quoting) and the `process` JSON (Version 1, exact fields, nothing else on stdout). Verify with `go test ./src/internal/cli -run 'Export|Process'`.
- [ ] 12.2 Fill any remaining gaps from #33 (cache, expiry, login retry, import write, console URL and logout). Verify with `go test -cover ./src/internal/cli`, which reports at least 70% coverage.
- [ ] 12.3 Commit `test: cover credential retrieval, caching and output` with `Closes #33`.

## 13. Build, CI and release

- [ ] 13.1 Bump the `go` directive to 1.27, update cobra, logrus, huh, go-ini and testify, and run `go mod tidy`. Verify with `make tests` and `make build-all`.
- [ ] 13.2 CI: trigger on `pull_request` and `push` to `main`, use `go-version-file: go.mod`, and add steps for `go vet ./...`, a `gofmt -l ./src` check that fails on output, `go test -race ./...`, `govulncheck` at a pinned version via `go run`, and `make build-all`. Verify with `actionlint` if available, otherwise by reviewing the YAML.
- [ ] 13.3 Release: use `go-version-file`, `permissions: contents: write`, `GITHUB_TOKEN`, create the release only if `gh release view` fails, fail on any other error, generate `aws-sso-login_checksums.txt` with `sha256sum` and upload it, and fix the asset-name comment. Verify by reviewing the YAML and by running the checksum step locally against `./tmp`.
- [ ] 13.4 Makefile: add `clean`, set `.SHELLFLAGS := -eu -o pipefail -c`, drop the ineffective `@set -e` lines, silence `git describe` with `2>/dev/null`, and add `vet` and `fmt-check` targets. Verify that `make clean`, `make help` and `make build` work in a tag-less clone without a git error.
- [ ] 13.5 Add the missing `v0.1.1` section to CHANGELOG.md (move the role-access fix from `Unreleased`, dated 2026-05-06), fix the compare links, and update the README Go version. Commit `ci: upgrade Go, harden CI and release, fix Makefile` with `Closes #34`.

## 14. Verification

- [ ] 14.1 Run `make tests`, `go vet ./...` and `gofmt -l ./src` (no output), and `openspec validate resolve-open-issues --strict` and `openspec validate --all --strict`. All must pass.
- [ ] 14.2 Build with `make build` and smoke-test against a temp `AWS_CONFIG_FILE` and `HOME` without the AWS CLI on `PATH`: `version` prints one line, `process` without a profile exits 1 with one stderr line, and `import --profile default` reports the missing AWS CLI. Verify that each exit status and stream matches the specs.

## 15. Pull request

- [ ] 15.1 Open a PR against `main` titled `fix: resolve open issues #22–#35`. In the description, link `openspec/changes/resolve-open-issues`, list `Closes #22` through `Closes #35` one per line, and note that it supersedes Dependabot PR #18. Verify on GitHub that all 14 issues show as linked to the PR.

## Workflow follow-up

- After merge, archive the change with `openspec archive resolve-open-issues`, run `openspec validate --all --strict`, and commit the archive separately.
- Confirm that GitHub closed issues #22–#35. Report any that stayed open instead of closing them by hand.
