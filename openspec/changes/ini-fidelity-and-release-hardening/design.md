# Design

## Context

Seven open issues (#39–#45) fall into two groups.

**INI fidelity and error classification (#39, #40, #44).** The AWS CLI (botocore) reads `~/.aws/config` and `~/.aws/credentials` with Python's `configparser.RawConfigParser` and default options: no inline comment prefixes, no quote stripping, full-line `#`/`;` comments only. `aws-sso-login` uses go-ini v1.67.0 with default options in two places:

- `src/internal/profiles/profiles.go` `load()`: `ini.LoadSources(ini.LoadOptions{}, configPath)`
- `src/internal/cli/credsfile.go` `loadCredentialsFile()`: `ini.Load(path)`

go-ini's defaults cut values at inline `#`/`;` and strip a pair of surrounding double quotes. `writeCredentialsFile` then calls `file.WriteTo` with go-ini's package-level `PrettyFormat = true`, which pads every key in every section into aligned columns. `isSSORoleNoAccessStderr` in `src/internal/cli/credentials.go` matches the substring `no access` in addition to the two exception names.

A throwaway test against go-ini v1.67.0 (not committed) confirmed:

- With `LoadOptions{IgnoreInlineComment: true}`, `abc#def;ghi`, `sh -c "a; b"`, `eu-west-1 ; trailing` and `https://example.awsapps.com/start/#` are read whole, and full-line `#`/`;` comments are still recognized and written back.
- With `PreserveSurroundedQuote: true`, `"x y"` and `"/p q/bin" --f "z"` keep their quotes.
- With `ini.PrettyFormat = false` and `ini.PrettyEqual = true`, keys are written as `key = value` without padding. When `IgnoreInlineComment` is set, go-ini writes values containing `#`/`;` verbatim instead of wrapping them in backticks.

**Delivery hygiene (#41, #42, #43, #45).** `go.mod` says `go 1.27.3`; both workflows use `actions/checkout@v4` and `actions/setup-go@v5` by tag; the publish workflow passes a fixed `--notes` string; `.github/dependabot.yml` has a `docker` entry for a non-existent `pipeline` directory and uses the `[dependabot]` prefix.

## Goals / Non-Goals

**Goals:**

- Read both AWS files with the same value semantics as AWS CLI v2, from one shared definition of the load options.
- `import` changes only the imported section's keys; other sections keep their values and lines.
- Legacy profiles with `#` in `sso_start_url` compute the same cache key as the AWS CLI.
- Classify "role not assigned" only from AWS exception names.
- Minimum Go `1.27`, pinned toolchain `go1.27.3` for CI and releases.
- SHA-pinned actions with an automated check; release notes from `CHANGELOG.md`; a Dependabot config without errors.

**Non-Goals:**

- Replacing go-ini or writing a parser. A new dependency would need its own justification, and go-ini's options cover the reported cases.
- Byte-for-byte preservation of blank lines, comment placement or key-delimiter style (`key=value` without spaces becomes `key = value` on rewrite). go-ini normalizes those; the AWS CLI reads both forms the same way.
- configparser features beyond the reported ones: indented continuation lines, `:` delimiters, `%` interpolation (botocore uses `RawConfigParser`, so interpolation is off anyway).
- Release signing, SBOMs, provenance.

## Decisions

### D1. One shared package for go-ini settings

Add `src/internal/awsini` with:

- `var LoadOptions = ini.LoadOptions{IgnoreInlineComment: true, PreserveSurroundedQuote: true}` (or an accessor returning a copy, so callers cannot mutate it).
- `Load(path string) (*ini.File, error)` calling `ini.LoadSources(LoadOptions, path)`, preserving the `fs.ErrNotExist` chain that `loadCredentialsFile` relies on.
- An `init()` that sets `ini.PrettyFormat = false` and `ini.PrettyEqual = true`.

`profiles.load` and `cli.loadCredentialsFile` both call `awsini.Load`. The credentials test that re-reads the file (`credsfile_test.go`) uses it too, so tests read files the same way production does.

*Alternatives considered:* putting the options in `helper` (would pull go-ini into a path/constants package used everywhere); exporting them from `profiles` (the credentials file is not a profile concern); setting the pretty-format globals inside `writeCredentialsFile` (mutating package globals on every write is racy in parallel tests and easy to forget in a second writer). A package `init()` is the single place go-ini's global write style is defined, and every importer of go-ini in this module goes through `awsini`.

### D2. Write format

`PrettyFormat = false` removes column alignment; `PrettyEqual = true` keeps ` = ` around the delimiter, matching what `aws configure` writes. `PrettySection` stays `true` (blank line between sections), which is go-ini's current behavior, so no new diff appears there. Empty values are written as `key = ` with a trailing space; this is accepted (AWS CLI reads it as empty) and noted in tests rather than special-cased.

### D3. Cache key and profile resolution

No code change is needed in `cacheKey` or `fromSection`: once the loader keeps `#`, `sso_start_url` flows through unchanged and the SHA-1 key matches botocore's. `value()` keeps `strings.TrimSpace`, which matches configparser's stripping of surrounding whitespace. Tests pin the full-URL cache key using a key computed from the documented algorithm.

### D4. Role-not-assigned detection

`isSSORoleNoAccessStderr` keeps only `forbiddenexception` and `accessdeniedexception` (case-insensitive). The real AWS CLI message always carries the exception name (`An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access`), so no true positive is lost. Anything else falls through to the existing paths: `import`/`console` run `aws sso login` once, `export`/`process` return the `aws sso login` advice. The `lowercase no access phrase` table case flips to `want: false`.

### D5. Go version

```
go 1.27
toolchain go1.27.3
```

With `GOTOOLCHAIN=auto` (the default) a Go 1.27.0–1.27.2 user building in this module is still switched to 1.27.3, because the `toolchain` line is honored for the main module; the gain is that `GOTOOLCHAIN=local` builds now work, and the `go` line stops claiming a language requirement the code does not have. `actions/setup-go` with `go-version-file: go.mod` is expected to honor the `toolchain` line. To avoid depending on that, both workflows add a step that compares `go env GOVERSION` with the `toolchain` line and fails on mismatch; if setup-go does not honor it, the implementer sets `go-version` from the `toolchain` line instead (still read from `go.mod`, e.g. via `go mod edit -json`). README's "Go 1.27.3 or newer" becomes "Go 1.27 or newer".

### D6. SHA-pinned actions and a pin check

Each `uses:` becomes `owner/repo@<40-hex-sha> # vX.Y.Z`, with SHAs resolved from the release tags via `gh api repos/<owner>/<repo>/git/ref/tags/<tag>` (dereferencing annotated tags). Dependabot's `github-actions` ecosystem updates SHA pins and their version comments. A `make check-action-pins` target greps `.github/workflows/*` for `uses:` lines whose ref is not 40 hex characters (local `./` actions excluded) and runs as a CI step, so a tag reference cannot slip back in.

### D7. Release notes from CHANGELOG.md

A `make release-notes VERSION=vX.Y.Z` target runs an `awk` script that prints the lines after `## [vX.Y.Z]` up to the next `## [` heading, trims leading/trailing blank lines, writes `tmp/release-notes.md`, and exits non-zero with `CHANGELOG.md has no entry for vX.Y.Z` when the result is empty. The publish workflow runs it right after checkout, before tests and builds, and passes `--notes-file tmp/release-notes.md` to `gh release create`. An existing release keeps its notes; assets are uploaded as today. `--generate-notes` was rejected because it duplicates the changelog and does not check it was updated.

### D8. Dependabot

Delete the `docker` entry. Use `commit-message: { prefix: "chore", include: "scope" }` for both remaining ecosystems, producing `chore(deps): ...`.

## Interaction with AWS CLI files and cache

- `~/.aws/config` is only read. Values now equal what botocore reads, so the resolved start URL, region, account and role match the AWS CLI for the reported cases.
- `~/.aws/credentials` is read and rewritten atomically, mode `0600`, as before. Other sections keep their values; their lines change only where go-ini normalizes delimiters or blank lines (Non-Goals).
- `~/.aws/cli/cache` key computation is unchanged in code; for legacy profiles with `#`/`;` in the start URL it now matches botocore. sso-session profiles are unaffected (`sessionName` is used).
- `AWS_CONFIG_FILE` and `AWS_SHARED_CREDENTIALS_FILE` are still resolved by the `helper` path functions.

## Platform differences

None. The change touches file parsing and formatting, not processes, browsers, paths or permissions; macOS and Linux behave the same. Workflow changes run on `ubuntu-latest` only; `awk` and `grep` usage stays POSIX so `make release-notes` and `make check-action-pins` also run on macOS.

## Testability

- INI behavior is tested with temp files and `AWS_CONFIG_FILE` / `AWS_SHARED_CREDENTIALS_FILE` pointing into `t.TempDir()`; no new seams are needed.
- `isSSORoleNoAccessStderr` is a pure function (table test). The login/no-login paths for unrelated "no access" stderr use the existing fake command runner in `testhelpers_test.go`.
- Because `awsini`'s `init()` sets go-ini globals, any test package that writes INI imports `awsini` (directly or via `cli`), so tests and production use the same write style.
- `make release-notes` and `make check-action-pins` are exercised locally against fixture input (a temp `CHANGELOG.md` and a temp workflow file) via `make -C` or by running the target with overridden file variables.

## Risks / Trade-offs

- [`PreserveSurroundedQuote` changes values that users quoted on purpose expecting go-ini to strip them] → the AWS CLI never stripped them, so the tool now agrees with the CLI; called out in CHANGELOG.
- [`PreserveSurroundedQuote` could make a quoted `sso_start_url` differ from an unquoted session value] → that matches botocore, which would also see different strings; the existing conflict error surfaces it.
- [go-ini globals are process-wide] → only `awsini` sets them, once, at init.
- [setup-go may ignore the `toolchain` line] → the version-assertion step catches it; fallback described in D5.
- [SHA pins look opaque] → version comments plus Dependabot updates.
- [A tag pushed without a changelog entry now fails the release] → intended; the error names the missing section, and re-running after adding the entry and re-tagging works.

## Migration Plan

No user migration. After merge, the next tag must have a `## [vX.Y.Z]` section in `CHANGELOG.md`. Rollback is a revert of the relevant commit; each issue lands in its own commit.

## Open Questions

None blocking. Whether `actions/setup-go` honors `toolchain` is resolved during implementation by the assertion step in D5.
