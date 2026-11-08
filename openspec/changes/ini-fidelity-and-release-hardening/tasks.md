# Tasks

## 1. Read AWS files like the AWS CLI (#39)

- [x] 1.1 Add `src/internal/awsini` with the shared load options (`IgnoreInlineComment: true`, `PreserveSurroundedQuote: true`) and `Load(path)`; verify with a package test that `abc#def;ghi`, `sh -c "a; b"`, `"x y"` and `https://example.awsapps.com/start/#` are read whole, full-line `#`/`;` comments are skipped, and a missing file still matches `errors.Is(err, fs.ErrNotExist)`
- [x] 1.2 Switch `profiles.load` in `src/internal/profiles/profiles.go` to `awsini.Load`; verify with `profiles` tests that `Resolve` keeps `sso_start_url = https://example.awsapps.com/start/#` for a legacy profile and an sso-session profile, that `sso_role_name = Admin;Ops` is kept, and that `ListSSOProfiles` lists a profile whose start URL ends in `#`
- [x] 1.3 Switch `loadCredentialsFile` in `src/internal/cli/credsfile.go` (and the re-read in `credsfile_test.go`) to `awsini.Load`; verify with a test that `updateCredentialsFile` keeps `aws_secret_access_key = abc#def;ghi`, `credential_process = sh -c "a; b"` and `credential_process = "/opt/my tool/bin" --profile x` in `[other]` unchanged, adds no comment lines, and stays unchanged after a second import
- [x] 1.4 Add a `cli` test that a legacy profile with `sso_start_url = https://example.awsapps.com/start/#` hits the cache file keyed by the full URL (key computed independently in the test) and `process --profile dev` prints the credentials without invoking the fake AWS CLI
- [x] 1.5 Add a `CHANGELOG.md` Unreleased `### Fixed` entry: values with inline `#`/`;` or surrounding quotes are read like the AWS CLI, `import` no longer corrupts other profiles, and legacy start URLs ending in `/#` find cached credentials (#39); verify the entry renders under `## [Unreleased]`
- [x] 1.6 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 1.1–1.6, then commit `fix: read AWS config and credentials values like the AWS CLI` with `Closes #39`

## 2. Keep credentials file formatting on import (#40)

- [x] 2.1 Add an `init()` to `src/internal/awsini` that sets `ini.PrettyFormat = false` and `ini.PrettyEqual = true`, with a comment that this is the only place go-ini's write style is set; verify with an `awsini` test that writing a section produces `key = value` lines without padding
- [x] 2.2 Add a `credsfile_test.go` case: `[other]` with `aws_access_key_id = AKIDEXAMPLE` and `region = eu-west-1` is written back with exactly those lines after `import --profile dev`, and every key in `[dev]` is written as `key = value`; verify the test fails on the old formatting and passes now
- [x] 2.3 Add a `CHANGELOG.md` Unreleased `### Fixed` entry: `import` no longer re-aligns keys in profiles it does not update (#40); verify the entry is present
- [x] 2.4 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 2.1–2.4, then commit `fix: keep credentials file formatting when importing` with `Closes #40`

## 3. Narrow role-not-assigned detection (#44)

- [x] 3.1 Remove the `no access` substring match from `isSSORoleNoAccessStderr` in `src/internal/cli/credentials.go` and update its doc comment; verify the `lowercase no access phrase` case in `credentials_test.go` now expects `false` and the `ForbiddenException` / `AccessDeniedException` cases still expect `true`
- [x] 3.2 Add fake-runner tests: refresh stderr `proxy error: no access to upstream host` makes `import --profile dev` run `aws sso login --profile dev` once, and makes `export --profile dev` fail with the `aws sso login --profile dev` advice and empty stdout; verify both pass
- [x] 3.3 Add a `CHANGELOG.md` Unreleased `### Fixed` entry: only AWS `ForbiddenException`/`AccessDeniedException` errors are reported as an unassigned role (#44); verify the entry is present
- [x] 3.4 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 3.1–3.4, then commit `fix: detect unassigned SSO roles only from AWS exception names` with `Closes #44`

## 4. Minimum Go version with a pinned toolchain (#45)

- [x] 4.1 Change `go.mod` to `go 1.27` plus `toolchain go1.27.3`; verify `go mod tidy` leaves `go.mod`/`go.sum` unchanged and `GOTOOLCHAIN=local go build ./...` succeeds with the local 1.27.x toolchain
- [x] 4.2 Add a step to `build_release.yaml` and `publish_release.yml` after setup-go that fails unless `go env GOVERSION` equals the `toolchain` value from `go mod edit -json go.mod`; if setup-go installs a different version, switch `go-version-file` to a `go-version` read from the `toolchain` line; verify the step passes on the PR's CI run
- [x] 4.3 Update `README.md` ("Go 1.27 or newer" for building from source) and add a `CHANGELOG.md` Unreleased `### Changed` entry (#45); verify no remaining mention of "1.27.3 or newer" with `grep -n "1.27.3" README.md`
- [x] 4.4 Run `make tests`, `go vet ./...`, `gofmt -l ./src` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 4.1–4.4, then commit `build: require go 1.27 and pin toolchain go1.27.3` with `Closes #45`

## 5. Pin GitHub Actions to commit SHAs (#42)

- [ ] 5.1 Replace every `uses:` in `.github/workflows/*` with `owner/repo@<40-hex-sha> # vX.Y.Z`, resolving each SHA from its release tag with `gh api` (dereference annotated tags); verify each SHA with `gh api repos/<owner>/<repo>/commits/<sha>`
- [ ] 5.2 Add a `check-action-pins` Makefile target (listed in `make help`) that fails and prints the offending line for any `uses:` ref that is not 40 hex characters, ignoring local `./` actions, and run it as a step in `build_release.yaml`; verify it fails on a temp workflow containing `uses: actions/checkout@v4` and passes on the repository
- [ ] 5.3 Add a `CHANGELOG.md` Unreleased `### Changed` entry about SHA-pinned actions (#42) and mention `make check-action-pins` in the README development section; verify both are present
- [ ] 5.4 Run `make check-action-pins`, `make tests` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 5.1–5.4, then commit `ci: pin GitHub Actions to commit SHAs` with `Closes #42`

## 6. Release notes from CHANGELOG.md (#43)

- [ ] 6.1 Add a `release-notes` Makefile target (listed in `make help`) taking `VERSION=vX.Y.Z` that writes the `## [vX.Y.Z]` section body of `CHANGELOG.md` to `tmp/release-notes.md` and exits non-zero with `CHANGELOG.md has no entry for vX.Y.Z` when it is missing or empty; verify `make release-notes VERSION=v0.2.0` yields only the v0.2.0 entries and `VERSION=v9.9.9` fails
- [ ] 6.2 In `publish_release.yml`, run `make release-notes VERSION=${{ github.ref_name }}` right after checkout and pass `--notes-file tmp/release-notes.md` to `gh release create`; keep the existing-release path uploading assets only; verify with `actionlint` (or by review if unavailable) that the workflow is valid
- [ ] 6.3 Update the README release section to say release notes come from `CHANGELOG.md` and a tag needs a matching section; add a `CHANGELOG.md` Unreleased `### Changed` entry (#43); verify both are present
- [ ] 6.4 Run `make tests` and `openspec validate ini-fidelity-and-release-hardening --strict`, tick 6.1–6.4, then commit `ci: publish release notes from CHANGELOG.md` with `Closes #43`

## 7. Dependabot configuration (#41)

- [ ] 7.1 Remove the `docker` entry from `.github/dependabot.yml` and set `commit-message.prefix: "chore"` with `include: "scope"` for `github-actions` and `gomod`; verify the file parses (`python3 -c 'import yaml,sys; yaml.safe_load(open(".github/dependabot.yml"))'`) and lists only those two ecosystems
- [ ] 7.2 Run `openspec validate ini-fidelity-and-release-hardening --strict`, tick 7.1–7.2, then commit `chore: drop unused docker ecosystem from dependabot` with `Closes #41`

## 8. Verification

- [ ] 8.1 Run `make tests`, `go vet ./...`, `gofmt -l ./src` (no output), `make check-action-pins`, `openspec validate ini-fidelity-and-release-hardening --strict` and `openspec validate --all --strict`; verify all pass and `CHANGELOG.md` Unreleased lists #39–#45

## 9. Pull request

- [ ] 9.1 Open a PR from this branch to `main` whose description links the change id `ini-fidelity-and-release-hardening` and lists `Closes #39`, `Closes #40`, `Closes #41`, `Closes #42`, `Closes #43`, `Closes #44`, `Closes #45`; verify CI is green, including the toolchain assertion and pin check

## Workflow follow-up

- Archive the change after merge, once README.md and CHANGELOG.md reflect the behavior, running `openspec validate --all --strict` afterwards.
- Confirm GitHub closed #39–#45 on merge; report any that stayed open.
