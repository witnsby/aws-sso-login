# Design

## Context

- `build_release.yaml` ("Build Release") runs CI on pull requests and on pushes to `main`. `publish_release.yml` ("Publish Release") runs on `vX.Y.Z` tag pushes: it checks the `CHANGELOG.md` section (`make release-notes`), tests, builds four binaries plus checksums, creates the release and uploads assets.
- Pull requests are merged with merge commits; the commits inside them follow Conventional Commits (`feat(cli)!:`, `docs(openspec):`, Dependabot's `chore(deps):`).
- The `main` ruleset exists but is disabled, so `github-actions[bot]` can push to `main`.
- Events caused by the built-in `GITHUB_TOKEN` (pushing a tag or a commit) do not start other workflows, and that token cannot act on another repository.
- The tap (`witnsby/homebrew-aws-sso-login`) updates `Formula/aws-sso-login.rb` from `releases/latest` on push, daily schedule and `workflow_dispatch`, and commits the result to its `main`.
- This change touches neither AWS CLI config, credentials or cache formats nor any CLI runtime path; the helper runs only in CI (Linux runners) and on maintainers' machines, where it reads git history and the GitHub API.

## Goals / Non-Goals

**Goals:**
- One automated path from a green merge on `main` to a Homebrew formula update, with no long-lived token beyond one narrowly scoped secret for the tap.
- Release logic (bump, changelog, notes) testable with `go test`, without network or a real git repository.
- Manual tags keep working through the same publish workflow.

**Non-Goals:**
- Reworking CI checks or the build targets.
- Migrating the tap repository to OpenSpec; its part is specified here and delivered as a separate PR there.

## Decisions

### 1. Trigger on `workflow_run` of "Build Release", not on `push`
`auto_release.yml` runs on `workflow_run` (`workflows: [Build Release]`, `types: [completed]`, `branches: [main]`) and proceeds only when `conclusion == success` and `event == push`. It checks out `workflow_run.head_sha`, so it evaluates exactly the commit CI tested.

Before releasing, it compares `head_sha` with the current `origin/main`. If they differ, it stops: the newer push has its own CI run and evaluation, which will include these commits. This prevents tagging an untested commit and avoids a non-fast-forward release push.

A `concurrency: { group: auto-release, cancel-in-progress: false }` block serializes runs.

*Alternatives:* `on: push` to `main` with a `needs:` on a duplicated test job (duplicates CI, could release when the separate CI fails); waiting on checks with polling (fragile).

### 2. In-repo Go helper instead of a third-party release tool
`src/cmd/release` (entrypoint) and `src/internal/release` (logic) use only the standard library. Subcommands, all writing results to stdout or files and diagnostics to stderr:

- `next-version` prints the next tag (`v0.4.0`) or nothing when no commit is releasable; it also prints the previous tag for later steps.
- `changelog --version vX.Y.Z --date YYYY-MM-DD [--file CHANGELOG.md]` promotes `[Unreleased]` in place.
- `notes --version vX.Y.Z --out tmp/release-notes.md` renders the release description.

Internals:
- `commits.go`: parse `type(scope)!: description` subjects and `BREAKING CHANGE:` / `BREAKING-CHANGE:` footers. Anything else is "non-conventional".
- `version.go`: semver parse, bump rules (0.x: breaking → minor), highest bump wins.
- `changelog.go`: Keep a Changelog rewrite of the `[Unreleased]` section and the link references; if `[Unreleased]` is empty, write generated `### Added` / `### Fixed` / `### Changed` entries.
- `notes.go`: group entries into sections, attach pull request links, list closed issues, append the compare link.
- `source.go`: gathers data through a `runner` seam (`func(ctx, name string, args ...string) ([]byte, error)`) that calls `git` and `gh`. Tests swap the runner for a scripted fake, like `testhelpers_test.go` does for the AWS CLI.

Data gathering:
- Previous tag: the highest tag from `git tag --merged <rev> --list 'v*'` that parses as `vX.Y.Z`, skipping the version being released; no tag → base `v0.0.0`, all history. (`git describe` picks the nearest tag, not the highest, and exits non-zero without tags.)
- Commits: `git log --no-merges --format=%H%x1f%s%x1f%b%x1e <prev>..<rev>`.
- Pull requests: merge commits in the range (`git log --merges`) whose subject is `Merge pull request #N ...`, plus `(#N)` suffixes of squash merges. For each, one `gh pr view N --json number,title,url,commits,closingIssuesReferences` call maps commits to their pull request and gives the closed issues. Commits without a pull request link to the commit URL.
- Repository URL from `GITHUB_REPOSITORY` / `GITHUB_SERVER_URL`, defaulting to `witnsby/aws-sso-login` locally.

*Alternatives:* semantic-release (Node toolchain plus plugins, unpinned npm dependencies; conflicts with the small-dependency and SHA-pin rules); release-please (adds a release PR step that the user did not want); shell plus `awk` in YAML (hard to unit-test, the changelog rewrite is error-prone). `gh release create --generate-notes` lists pull requests but cannot group by commit type or list closed issues.

### 3. Release commit, annotated tag, atomic push
The `prepare` job configures `github-actions[bot]` as committer, runs `changelog`, commits `chore(release): vX.Y.Z`, creates `git tag -a vX.Y.Z -m "Release vX.Y.Z"`, and runs `git push --atomic origin HEAD:refs/heads/main refs/tags/vX.Y.Z`. It fails first if the tag already exists (local or `git ls-remote`).

Tagging the release commit, not the merge commit, means the tagged tree has the `## [vX.Y.Z]` section, so the existing changelog gate passes and `git describe` gives the binaries the right version.

The push uses `GITHUB_TOKEN`, so it starts neither CI nor `publish_release.yml`'s tag trigger. That prevents a loop and a double publish. As extra protection, `next-version` treats a `chore(release):` commit as non-releasing.

### 4. Publish via reusable workflow
`publish_release.yml` gets `workflow_call` with input `tag` next to its `push: tags` trigger. `auto_release.yml` calls it in a `publish` job (`needs: prepare`, `with: { tag }`, `secrets: inherit`). Inside, `TAG=${{ inputs.tag || github.ref_name }}`, and checkout uses `ref: refs/tags/$TAG` with `fetch-depth: 0`.

Steps after this change: changelog gate (`make release-notes VERSION=$TAG`, unchanged target, output now only a gate) → tests → build-all → checksums → `go run ./src/cmd/release notes` → create the release with `--notes-file` or, if it exists, upload with `--clobber` → `notify-tap` job.

Permissions: `contents: write` for the release; `pull-requests: read` and `issues: read` for `gh pr view`. A called workflow gets at most the caller's permissions, so `auto_release.yml` grants the same set.

### 5. Tap notification via `workflow_dispatch` with a fine-grained token
`notify-tap` (`needs: build-and-publish`) runs `gh workflow run publish_release.yml -R witnsby/homebrew-aws-sso-login --ref main -f tag=$TAG` with `GH_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}`. If the secret is empty, the job prints a `::warning::` and a step-summary line and exits 0.

Token: fine-grained personal access token (or GitHub App token later), resource owner `witnsby`, repository access only `homebrew-aws-sso-login`, permission `Actions: Read and write`. The existing `GH_TOKEN` secret is not reused: its scope is unknown and the name collides with the variable `gh` reads.

*Alternatives:* `repository_dispatch` (needs `Contents: write` on the tap, a wider permission); relying on the tap's daily schedule (up to 24 h delay, which is what the user wants to remove).

### 6. Tap side: optional `tag` input
In the tap repository, `publish_release.yml`'s `workflow_dispatch` gets `inputs.tag` (string, optional) and `run-name: Fetch Release ${{ inputs.tag || 'latest' }}`. The step passes `RELEASE_TAG: ${{ inputs.tag }}` to `make run`. `src/main.go` uses `GET /repos/witnsby/aws-sso-login/releases/tags/<tag>` when `RELEASE_TAG` is set, `releases/latest` otherwise, and fails when one of the four platform assets is missing. The URL builder and asset check are covered by unit tests with `httptest`. Pinning the tag avoids a race with "latest" and makes reruns reproducible.

### 7. Testability summary
- Pure functions (parse, bump, changelog rewrite, notes rendering) get table-driven tests with fixed input, including the scenarios in `specs/release-automation/spec.md`.
- `git` / `gh` access goes through the injected runner; tests feed recorded output. No test touches the network, the real repository's tags or `~/.aws`.
- Workflow YAML is checked by `make check-action-pins` in CI. `actionlint` is run locally during implementation if it is installed, but it is not added to CI.
- Platform: the helper only runs `git` and `gh`, with no paths or permissions that differ between macOS and Linux.

## Risks / Trade-offs

- [A non-conventional or mistyped subject (`feature:`) silently skips a release] → Documented in `AGENTS.md`/`README.md`; the run summary lists the commits it ignored. A commit-lint check is a possible follow-up.
- [Enabling the `main` ruleset later blocks the release push] → Noted in `README.md`; the ruleset needs a bypass for GitHub Actions, or the flow moves to a GitHub App token.
- [`workflow_run` workflows run from the default branch's definition] → Changes to `auto_release.yml` take effect only after merge; the first real run happens on the first releasable merge after this PR.
- [Merging this change could itself trigger a release] → The helper and workflows are not user-facing, so their commits use `ci:`, `build:`, `test:` and `docs:` types; merging this change releases nothing, and the first automated release comes from the next `feat`/`fix` merge. The PR description says so.
- [Personal token expires] → Dispatch fails loudly in `notify-tap` while the release stays; the tap's schedule still catches up within a day.
- [Dependency-only security bumps (`chore(deps)`) do not release] → Accepted; a maintainer can merge a `fix(deps):` commit or push a tag by hand.

## Migration Plan

1. Merge this change in `aws-sso-login` (CI green). No release is expected from it unless it contains releasing commit types.
2. Merge the tap PR (`tag` input, updater change).
3. A maintainer creates the fine-grained token and stores it as `HOMEBREW_TAP_TOKEN` in `witnsby/aws-sso-login` (Settings → Secrets and variables → Actions).
4. The next `feat`/`fix` merge produces the first automated release; verify the changelog commit, tag, release body and the tap's "Fetch Release" run.

Rollback: delete or disable `auto_release.yml`; `publish_release.yml` still works on manually pushed tags, and the tap still runs on schedule and manual dispatch without the input.
