# Tasks

Each group is one logical change. It ends with its own commit after `make tests`, `go vet ./...`, `gofmt -l ./src` (no output) and `openspec validate auto-release-on-main --strict` pass. Tests never touch the network, the real repository's tags, `~/.aws` or the AWS CLI. Implementation commits use non-releasing types (`build`, `ci`, `test`, `docs`) so that merging this change does not itself publish a release.

## 1. Proposal

- [x] 1.1 Write proposal, design, the `release-automation` and `build-release` spec deltas and tasks; verify with `openspec validate auto-release-on-main --strict`.
- [x] 1.2 Commit `docs(openspec): propose auto-release-on-main` with `Refs #43`.

## 2. Version from Conventional Commits

- [x] 2.1 Add `src/internal/release/commits.go`: parse `type(scope)!: description` subjects and `BREAKING CHANGE:` / `BREAKING-CHANGE:` footers; classify each commit as breaking, feat, fix, perf, revert, other or non-conventional; treat `chore(release):` as non-releasing. Verify with table tests in `commits_test.go` (scopes, `!`, footers, `Merge pull request` and `Update README` subjects).
- [x] 2.2 Add `src/internal/release/version.go`: semver parse/format, bump rules (highest bump wins, breaking → minor while major is `0`, `v0.0.0` base without tags, no bump without releasing commits). Verify with table tests covering every "Version from Conventional Commits" and "No release without user-facing commits" scenario.
- [x] 2.3 Add the `runner` seam and `source.go` (previous tag via `git tag --merged`, commits via `git log --no-merges` between the previous tag and `--rev`, default `HEAD`), and `src/cmd/release/main.go` with subcommand `next-version` printing `next=<tag>` and `previous=<tag>` lines (empty `next` when nothing is releasable) to stdout and diagnostics to stderr. Verify with tests that use a scripted fake runner, including a repository without tags.
- [x] 2.4 Commit `build(release): compute the next version from Conventional Commits` with `Refs #43`.

## 3. Changelog promotion

- [ ] 3.1 Add `src/internal/release/changelog.go` and the `changelog --version --date [--file]` subcommand: move `[Unreleased]` content into `## [vX.Y.Z] - YYYY-MM-DD`, leave an empty `## [Unreleased]`, rewrite the `[Unreleased]` link and insert the `[vX.Y.Z]` compare link; when `[Unreleased]` is empty, write generated `### Added` / `### Fixed` / `### Changed` (`**BREAKING**:` prefix) entries. Verify with golden-file tests for curated entries, an empty section, a first release without previous tag, and a copy of the current `CHANGELOG.md` promoted to `v0.3.1`.
- [ ] 3.2 Commit `build(release): promote the Unreleased changelog section` with `Refs #43`.

## 4. Generated release notes

- [ ] 4.1 Extend `source.go` to collect pull requests from merge-commit subjects (`Merge pull request #N`) and squash suffixes (`(#N)`) in the range, and to call `gh pr view N --json number,title,url,commits,closingIssuesReferences` once per pull request through the runner. Verify with fake-runner tests that map commits to pull requests and fall back to commit links.
- [ ] 4.2 Add `src/internal/release/notes.go` and the `notes --version [--rev] --out` subcommand: sections Breaking changes, Features, Fixes, Improvements, Other pull requests, Closed issues (deduplicated), compare link (omitted without previous tag), empty sections left out. Verify with golden tests for every "Generated release notes" and "Closed issues in release notes" scenario.
- [ ] 4.3 Commit `build(release): generate release notes with pull requests and closed issues` with `Refs #43`.

## 5. Workflows and documentation

- [ ] 5.1 Make `.github/workflows/publish_release.yml` reusable: add `workflow_call` with input `tag`, resolve `TAG` from the input or `github.ref_name`, check out `refs/tags/$TAG` with full history, keep `make release-notes` as the changelog gate, generate the description with `go run ./src/cmd/release notes`, create the release or upload with `--clobber`, and grant `contents: write`, `pull-requests: read`, `issues: read`. Verify with `make check-action-pins` and `actionlint` if installed.
- [ ] 5.2 Add the `notify-tap` job to `publish_release.yml` (`needs` the publish job): run `gh workflow run publish_release.yml -R witnsby/homebrew-aws-sso-login --ref main -f tag=$TAG` with `GH_TOKEN` from `secrets.HOMEBREW_TAP_TOKEN`; with an empty secret print a `::warning::` and a step-summary line and exit 0. Verify by reading the job against the "Homebrew tap notification" scenarios and with `actionlint` if installed.
- [ ] 5.3 Add `.github/workflows/auto_release.yml`: `workflow_run` on "Build Release" completed for `main`, proceed only on `success` + `push`, `concurrency: auto-release`; `prepare` job checks out `head_sha`, stops when `origin/main` moved, runs `next-version`, fails if the tag exists, runs `changelog`, commits `chore(release): vX.Y.Z` as `github-actions[bot]`, creates the annotated tag and pushes with `git push --atomic`; `publish` job calls `./.github/workflows/publish_release.yml` with the tag and `secrets: inherit`. Actions pinned by SHA. Verify with `make check-action-pins` and `actionlint` if installed.
- [ ] 5.4 Update `README.md` (release section: automatic releases from Conventional Commits, the `HOMEBREW_TAP_TOKEN` secret and its permissions, manual tags still supported, `main` ruleset bypass note), `AGENTS.md` (commit types decide releases; `src/cmd/release` in the layout) and add a `### Changed` entry to `[Unreleased]` in `CHANGELOG.md`. Verify the documented commands match the workflows.
- [ ] 5.5 Commit `ci: release automatically after green merges to main` with `Refs #43`.

## 6. Homebrew tap (`witnsby/homebrew-aws-sso-login`, separate PR)

- [ ] 6.1 In the tap worktree, add the optional `tag` input and `run-name` to `workflow_dispatch` in `.github/workflows/publish_release.yml` and pass it as `RELEASE_TAG` to `make run`. Verify by reading the workflow and with `actionlint` if installed.
- [ ] 6.2 In the tap's `src/main.go`, fetch `releases/tags/<RELEASE_TAG>` when set and `releases/latest` otherwise, and fail when any of the four platform assets is missing. Add `httptest`-based unit tests for the URL choice, an unknown tag (404) and a missing asset; verify with `make tests` in the tap.
- [ ] 6.3 Update the tap's `README.md` and `AGENTS.md`: releases trigger the workflow automatically with a tag; schedule and manual runs stay. Verify the documented `gh workflow run ... -f tag=` command.
- [ ] 6.4 Commit in the tap repository `feat: update the formula for a given release tag on dispatch`, push the branch and open its PR, linking this change and the `aws-sso-login` PR.

## 7. Verification

- [ ] 7.1 Run `make tests`, `go vet ./...`, `gofmt -l ./src` (no output), `make check-action-pins`, `openspec validate auto-release-on-main --strict` and `openspec validate --all --strict`.
- [ ] 7.2 Dry-run the helper read-only against this repository: `go run ./src/cmd/release next-version` reports no release for `v0.3.0..HEAD` (docs-only), and `go run ./src/cmd/release notes --version v0.3.0 --rev v0.3.0 --out <scratch>` lists the v0.3.0 features, the breaking exit-code change and its pull requests; `changelog` run on a scratch copy of `CHANGELOG.md` produces the expected section. Nothing is committed or pushed by the dry run.

## 8. Pull request

- [ ] 8.1 Push the branch and open the `aws-sso-login` PR. The description links the change id `auto-release-on-main`, lists `Refs #43` and `Refs #34`, links the tap PR, states that merging it creates no release, and lists the maintainer step to create `HOMEBREW_TAP_TOKEN`.

## Workflow follow-up

- A maintainer creates the fine-grained token (owner `witnsby`, repository `homebrew-aws-sso-login`, `Actions: Read and write`) and saves it as the `HOMEBREW_TAP_TOKEN` secret in `witnsby/aws-sso-login`.
- After both PRs merge, check the first automated release: changelog commit, tag, release description, assets and the tap's "Fetch Release" run for that tag.
- Then run `openspec-verify-change` and archive the change.
