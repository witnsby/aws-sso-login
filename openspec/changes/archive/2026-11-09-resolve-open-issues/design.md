# Design

## Context

See proposal.md for motivation and specs/ for the required behavior. Current state relevant to the approach:

- The AWS config is parsed twice with different rules. `profiles.ListSSOProfiles` handles the picker and understands `[default]`. `cli.retrieveProfile` handles every command and only looks up `[profile <name>]`. Neither understands `sso_session`.
- `cli/common.go` mixes profile loading, AWS CLI calls, cache parsing, expiry checks, the federation HTTP call and config-path resolution. It calls `exec.Command`, `user.Current()`, `time.Now()` and `http.Get` directly, which keeps package coverage at about 9%.
- Only `execCommand` (browser) and `defaultSelector` (picker) are swappable today.
- botocore (AWS CLI v2) builds the SSO cache key as `sha1(json.dumps(args, sort_keys=True, separators=(',', ':')))`. `args` holds `accountId` and `roleName`, plus `sessionName` when `sso_session` is set or `startUrl` otherwise. Its cached-credential fetcher refreshes credentials that expire within 15 minutes (`DEFAULT_EXPIRY_WINDOW_SECONDS = 60 * 15`). These details were checked against awscli 2.39 sources.

## Goals / Non-Goals

**Goals:**
- One place that resolves a profile name to a validated SSO configuration, used by every command and the picker.
- Every external effect (AWS CLI, filesystem root, clock, HTTP, terminal detection, browser launch) sits behind a package-level seam, so each spec scenario can be unit-tested with temp dirs and fakes.
- Each issue lands as its own commit, in an order that keeps the build green.

**Non-Goals:**
- No new packages beyond what the split needs, and no public library API (everything stays under `src/internal`).
- No talking to AWS SSO or STS APIs directly. The AWS CLI remains the only path to AWS.

## Decisions

### 1. Profile resolution lives in `profiles`
`profiles` gets a `Resolve(configPath, name) (Profile, error)` that returns a fully validated `Profile` (Name, AccountID, RoleName, StartURL, SSORegion, Region, SessionName). `ListSSOProfiles` is reimplemented on top of the same section-lookup and validation helpers. `cli` stops importing go-ini for config reads. Commands receive a `profiles.Profile` struct instead of an `*ini.Section`.
- *Alternative*: keep `retrieveProfile` in `cli` and teach both parsers about `default` and `sso_session`. Rejected because the duplication is the root cause of #22.
- `sso_session` precedence and conflict checks follow botocore. The session's values win. A profile-level `sso_start_url` or `sso_region` that differs from the session's is an error.

### 2. Cache key computed from the resolved profile
`cacheKey(p profiles.Profile)` marshals a `map[string]string` with `json.Marshal`. Go sorts map keys and emits compact JSON, which is byte-identical to botocore's `sort_keys=True, separators=(',', ':')` output for ASCII values. The map includes `sessionName` when `p.SessionName != ""` and `startUrl` otherwise. Golden-value tests pin the hash for both formats.

### 3. Seams
Package-level variables in `cli`, mirroring the existing `execCommand`:

| Seam | Production value | Used for |
| --- | --- | --- |
| `execCommand` | `exec.Command` | AWS CLI and browser |
| `lookPath` | `exec.LookPath` | Missing AWS CLI detection, browser availability |
| `now` | `time.Now` | Expiry margin |
| `httpClient` | `&http.Client{Timeout: 15 * time.Second}` | Federation request |
| `federationEndpoint` | `https://signin.aws.amazon.com/federation` | Federation request |
| `isInteractive` | `isatty(stdin) && isatty(stderr)` | Picker availability |
| `goos` | `runtime.GOOS` | Browser command choice |

`helper` replaces `user.Current()` with `os.UserHomeDir()`, which honors `$HOME` and lets tests point it at a temp dir. It exposes `AwsConfigPath()`, `AwsCredentialsPath()` and `AwsCliCachePath()`, honoring `AWS_CONFIG_FILE` and `AWS_SHARED_CREDENTIALS_FILE`. `cli.GetAwsConfigPath` is removed.
- *Alternative*: one big `deps` struct passed through every function. Rejected as a larger diff for no test benefit in a single-binary CLI. The package-level seams are already the project convention (see AGENTS.md, Testability).

### 4. Missing AWS CLI is detected before any AWS CLI call
`requireAWSCLI()` calls `lookPath("aws")` once per command that needs credentials and returns a sentinel `errAWSCLINotFound`. The command runner also maps `errors.Is(err, exec.ErrNotFound)` to the same sentinel, as a second line of defense. The login retry in `retrieveCredentials` skips the login for `errAWSCLINotFound` and `errSSORoleNoAccess`.

### 5. Expiry margin
A constant `minCredentialValidity = 5 * time.Minute` is checked as `now().Add(minCredentialValidity).Before(exp)`. Because botocore refreshes within 15 minutes, `aws sts get-caller-identity` always replaces credentials that our 5-minute margin rejects. A cache that is still within 5 minutes after the refresh therefore means the SSO session itself cannot issue new credentials, and the command fails. `time.RFC3339` is added to the accepted layouts.

### 6. Login output and stdin
`aws sso login` runs with `Stdin = os.Stdin`, `Stdout = os.Stderr` and `Stderr = os.Stderr`. Sending both streams to stderr keeps stdout clean for every command. Only `import` and `console` can trigger it (spec: no login for `export` and `process`).

### 7. Atomic credentials write
`writeCredentialsFile(path, *ini.File)` performs these steps:
1. Resolve symlinks with `filepath.EvalSymlinks`. A missing target is fine.
2. Create the parent directory with `MkdirAll(dir, 0o700)`.
3. Create the temp file with `os.CreateTemp(dir, ".credentials-*")`.
4. `Chmod(0o600)`, write with `WriteTo`, `Sync`, `Close`.
5. `os.Rename` the temp file over the target. On any error, remove the temp file.

The rename keeps the update atomic on the same filesystem on macOS and Linux. `0600` is applied explicitly, so the umask cannot loosen it.
- *Alternative*: `Chmod` the existing file and keep go-ini's `SaveTo`. Rejected because the write would still be non-atomic.

### 8. Federation request
The request is built as `federationEndpoint + "?" + params`. `httpClient.Do` errors are a `*url.Error` that embeds the full URL, which contains the session credentials, so the wrapper returns `fmt.Errorf("federation request failed: %w", uerr.Err)` and drops the URL. Non-200 responses return the status code only. A body that does not decode, or decodes to an empty `SigninToken`, returns a fixed message. The body is read through `io.LimitReader(1 MiB)`.

### 9. Browser command per OS
`openBrowser` picks `open` (darwin), `xdg-open` (linux) or `rundll32 url.dll,FileProtocolHandler` (windows, best effort) from `goos`. If `lookPath` fails or `Start` errors, it writes `Please open this URL in your browser: <url>` to stderr. The started process is released (`cmd.Process.Release()`) so it does not linger as a zombie. On macOS `open` returns quickly. On Linux `xdg-open` can stay running briefly; releasing it lets the CLI exit without waiting.
- The fallback prints the sign-in URL, which carries a short-lived sign-in token, to stderr. This happens only when no browser can be launched, and it matches the existing behavior (which printed to stdout). The URL is never logged at any level.

### 10. Persistent flags and logging
`--profile` and `--debug` move to `rootCmd.PersistentFlags()`. A `PersistentPreRunE` on the root does three things:
1. Sets the log level: debug if `--debug` or `AWS_SSO_LOGIN_DEBUG` is `1` or `true`; warn for `process` and `export`; info otherwise.
2. Resolves the profile name for commands that need one, using the order flag, `AWS_PROFILE`, then the picker when allowed, so precedence is implemented once.
3. Stores the resolved name for the command.

Errors are returned wrapped and rendered once by `reportAndExit`. Intermediate `logrus.Error` and `Info` calls that duplicate the returned error are removed.

### 11. Picker on stderr
`HuhSelector.Pick` wraps the select in `huh.NewForm(huh.NewGroup(sel)).WithOutput(os.Stderr)`, so `eval "$(aws-sso-login export)"` never captures the UI. TTY detection uses `github.com/mattn/go-isatty`, already in the module graph through huh and promoted from indirect to direct. This adds no new module. The stdlib `ModeCharDevice` check is not used because `/dev/null` also passes it.

### 12. Splitting `awsCredentialsManager`
The struct is replaced by focused files in `cli`:
- `credentials.go`: retrieval, cache, expiry, login retry, AWS CLI runner.
- `credsfile.go`: credentials-file load, update and atomic write.
- `console.go`: sign-in token, URL building, logout, region choice.
- `browser.go`: opening the browser.

`common.go` is deleted once it is empty. Each command function receives a `profiles.Profile` and the injected seams.

### 13. Commit order
The order is chosen so each step builds and passes tests on its own:
1. Seams and helpers (#33 groundwork, #35 path-helper move).
2. Profile resolution (#22, #23 parsing).
3. Cache key (#23).
4. Expiry (#27).
5. Missing CLI (#29).
6. Login output (#25).
7. Credentials file (#26).
8. Console fixes (#24, #28, #30, region from #35).
9. Persistent flags and picker (#32).
10. Logging (#31).
11. Remaining cleanups (#35).
12. Coverage top-up (#33).
13. Build, CI and release (#34).
14. Docs.

### Interaction with AWS CLI files
- `~/.aws/config` and `AWS_CONFIG_FILE`: read-only, both section syntaxes and `[sso-session]` are supported.
- `~/.aws/cli/cache`: read-only, key algorithm identical to botocore. Refresh happens only through `aws sts get-caller-identity`.
- `~/.aws/credentials` and `AWS_SHARED_CREDENTIALS_FILE`: rewritten atomically with `0600`. Other sections and comments are preserved by go-ini. Key order inside the imported section is unchanged.

### Platform differences (macOS vs Linux)
- Browser: `open` vs `xdg-open`. Headless Linux typically has neither, which triggers the stderr fallback.
- `os.Rename` is atomic within a directory on both platforms, and the temp file is created in the target directory to guarantee that.
- File modes `0600` and `0700` are enforced explicitly on both platforms. On Windows (best effort) the mode bits are mostly ignored, and nothing in the specs promises otherwise.

## Risks / Trade-offs

- [Picker is now offered for `console` and `export`] → It appears only when both stdin and stderr are TTYs, and `process` never prompts. Scripts that relied on the "must specify --profile" error in a terminal now see a prompt instead. This is an acceptable, additive behavior change.
- [`AWS_PROFILE` fallback for `process` can pick the profile whose `credential_process` invokes us] → The resolved profile is then not an SSO profile and fails with a clear error. The README recommends always passing `--profile` in `credential_process`.
- [Stricter expiry margin causes more AWS CLI refreshes] → At most one `sts get-caller-identity` call per command, and only in the last 5 minutes of a session.
- [Atomic rename replaces the file inode] → Hard links to `~/.aws/credentials` stop sharing content, while symlinks are preserved by resolving them first. This is documented in the README.
- [Go toolchain bump to 1.27] → Contributors need Go 1.27+. `go.mod`'s `go` line and the README state it.
- [One large change] → Mitigated by one commit per task group and strict validation before each commit, so the PR can be reviewed commit by commit.

## Migration Plan

No user migration is required. All changes are backward compatible: existing legacy profiles, flags and output formats keep working. On first `import`, an existing credentials file is tightened to `0600`. For rollback, revert the PR. No persisted state changes format.
