# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project overview

`aws-sso-login` is a Go CLI that simplifies AWS IAM Identity Center (AWS SSO) authentication and credential management. It supports console login, exporting credentials as shell env vars, importing them into `~/.aws/credentials`, and emitting `credential_process`-compatible JSON. The `import` command accepts an optional `--profile` flag; when omitted on macOS/Linux, an interactive picker (powered by `charmbracelet/huh`) lists all SSO-enabled profiles from `~/.aws/config`.

The tool does not talk to the AWS SSO APIs directly. It shells out to AWS CLI v2 (`aws sso login`, `aws sts get-caller-identity`) and reads the role credentials the AWS CLI caches in `~/.aws/cli/cache`.

- **Language**: Go (version pinned in `go.mod`)
- **Module**: `github.com/witnsby/aws-sso-login`
- **CLI framework**: [`spf13/cobra`](https://github.com/spf13/cobra)
- **Interactive prompts**: [`charmbracelet/huh`](https://github.com/charmbracelet/huh)
- **Logging**: [`sirupsen/logrus`](https://github.com/sirupsen/logrus) (writes to stderr)
- **Config parsing**: [`go-ini/ini`](https://github.com/go-ini/ini)
- **Testing**: [`stretchr/testify`](https://github.com/stretchr/testify)
- **Specs**: [OpenSpec](https://github.com/Fission-AI/OpenSpec), configured in `openspec/config.yaml`

## Repository layout

```
.
├── src/
│   ├── cmd/bin/main.go          # Binary entrypoint, calls cli.Run()
│   └── internal/
│       ├── cli/                 # Cobra commands and command logic (console, export, import, process, version)
│       ├── profiles/            # Listing SSO-enabled profiles from ~/.aws/config
│       ├── helper/              # Path helpers, constants, build-time version variables
│       └── model/               # Shared data types (RoleCredential)
├── openspec/                    # OpenSpec config, specs and change proposals
├── .github/workflows/           # CI: build on PR, release on tag
├── .cursor/, .claude/, .agents/ # Agent rules and skills
├── Makefile                     # Build / test / coverage targets
├── CHANGELOG.md                 # Keep a Changelog format
└── README.md                    # User documentation
```

Keep new code under `src/internal/...`; this is a CLI, not a library.

## Common commands

| Task                        | Command                                       |
| --------------------------- | --------------------------------------------- |
| Install deps                | `make install-deps`                           |
| Run tests                   | `make tests`                                  |
| Coverage (HTML)             | `make cover`                                  |
| Vet                         | `go vet ./...`                                |
| Format check                | `gofmt -l ./src`                              |
| Build (host)                | `make build`                                  |
| Build all platforms         | `make build-all`                              |
| Run binary                  | `./tmp/aws-sso-login --help`                  |
| Import (direct profile)     | `./tmp/aws-sso-login import --profile <name>` |
| Import (interactive picker) | `./tmp/aws-sso-login import`                  |

The build output goes to `./tmp/`. Cross-compile targets cover `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`. Version and commit are injected via `-ldflags` into `helper.Version` and `helper.CommitHash`.

## Coding conventions

- **Formatting**: run `gofmt` (or `goimports`) before committing.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`; never silently discard. Either log an error or return it, not both: `reportAndExit` in `src/internal/cli/cli.go` renders the final error once.
- **Logging**: use `logrus` with structured fields (`logrus.WithField(...)`), not `fmt.Println`, for diagnostics.
- **Output contracts**: `export` stdout must stay `eval`-safe and `process` stdout must contain only the `credential_process` JSON. Anything else goes to stderr.
- **CLI**: add new commands as Cobra subcommands in `src/internal/cli/` and register them in `Run()` in `src/internal/cli/cli.go`. Declare flags in that file's `init()`.
- **Config**: list SSO profiles with `profiles.ListSSOProfiles`, load a single profile with `retrieveProfile`, and resolve the config path with `GetAwsConfigPath` (honors `AWS_CONFIG_FILE`). Do not re-parse `~/.aws/config` ad hoc.
- **Testability**: external commands, the filesystem and interactive prompts go behind package-level seams (see `execCommand` and `defaultSelector`) so tests can swap in fakes.
- **Tests**: colocate `_test.go` files with the code; use `testify/assert` and `testify/require`. Cover new branches and error paths.

## Spec-driven changes (OpenSpec)

For non-trivial behavior changes, create an OpenSpec change before writing code (`/opsx:propose` or the `openspec-propose` skill). Project constraints for artifacts live in `openspec/config.yaml`. Specs are in `openspec/specs/`, active changes in `openspec/changes/`.

## Pull request expectations

Before opening a PR:

1. `make tests` passes.
2. `gofmt -l ./src` produces no output.
3. `go vet ./...` is clean.
4. New user-facing behavior is reflected in `README.md` and in the `Unreleased` section of `CHANGELOG.md`.
5. No credentials, tokens, or `~/.aws/...` contents from your machine are committed.

## Safety rules for agents

- **Never** print, log, or commit real AWS credentials, SSO tokens, account IDs, or start URLs. Use placeholders like `123456789012` and `https://example.awsapps.com/start`.
- **Never** read or modify the user's real `~/.aws/config`, `~/.aws/credentials` or `~/.aws/cli/cache` during development; use temp dirs and `AWS_CONFIG_FILE` in tests.
- Do not run `aws sso login` or any AWS CLI command or AWS API call as part of automated edits or tests.
- Do not push to `main` or force-push without explicit instruction.
- Prefer minimal, focused diffs; do not reformat unrelated files.
- Write code, comments, commit messages and documentation in English.

## Useful references

- README: `README.md`
- Changelog: `CHANGELOG.md`
- License: `LICENSE` (Apache-2.0)
- CI: `.github/workflows/`
- OpenSpec config: `openspec/config.yaml`
- Cursor rules: `.cursor/rules/`
