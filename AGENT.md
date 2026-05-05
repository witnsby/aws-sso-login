# AGENT.md

Guidance for AI coding agents working in this repository.

## Project overview

`aws-sso-login` is a Go CLI that simplifies AWS SSO authentication and credential management. It supports console login, exporting credentials as shell env vars, importing them into `~/.aws/credentials`, and emitting `credential_process`-compatible JSON.

- **Language**: Go 1.23+
- **Module**: `github.com/witnsby/aws-sso-login`
- **CLI framework**: [`spf13/cobra`](https://github.com/spf13/cobra)
- **Logging**: [`sirupsen/logrus`](https://github.com/sirupsen/logrus)
- **Config parsing**: [`go-ini/ini`](https://github.com/go-ini/ini)
- **Testing**: [`stretchr/testify`](https://github.com/stretchr/testify)

## Repository layout

```
.
├── src/
│   ├── cmd/bin/main.go      # Binary entrypoint
│   └── internal/            # Internal packages (not importable externally)
├── .github/workflows/       # CI workflows
├── Makefile                 # Build / test / coverage targets
├── go.mod / go.sum
└── README.md
```

Keep new code under `src/` and prefer `src/internal/...` for packages that should not leak as a public API.

## Common commands

| Task                 | Command                       |
| -------------------- | ----------------------------- |
| Install deps         | `make install-deps`           |
| Run tests            | `make tests`                  |
| Coverage (HTML)      | `make cover`                  |
| Build (host)         | `make build`                  |
| Build all platforms  | `make build-all`              |
| Run binary           | `./tmp/aws-sso-login --help`  |

The build output goes to `./tmp/`. Cross-compile targets cover `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`.

## Coding conventions

- **Formatting**: always run `gofmt` (or `goimports`) before committing.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`; never silently discard.
- **Logging**: use `logrus` with structured fields (`logrus.WithField(...)`), not `fmt.Println` for diagnostics.
- **CLI**: add new commands as Cobra subcommands under `src/internal/...`; wire them up in `src/cmd/bin/main.go`.
- **Config**: read AWS profiles via the existing helpers; do not re-parse `~/.aws/config` ad hoc.
- **Tests**: colocate `_test.go` files with the code; use `testify/assert` and `testify/require`. Aim to cover new branches and error paths.
- **Public API**: this is a CLI, not a library — keep packages in `internal/` unless there is a clear reason to expose them.

## Pull request expectations

Before opening a PR:

1. `make tests` passes.
2. `gofmt -l ./src` produces no output.
3. `go vet ./...` is clean.
4. New user-facing behavior is reflected in `README.md` and (if relevant) `CHANGELOG.md`.
5. No credentials, tokens, or `~/.aws/...` paths from your machine are committed.

## Safety rules for agents

- **Never** print, log, or commit real AWS credentials, SSO tokens, account IDs, or start URLs. Use placeholders like `123456789012` and `https://example.awsapps.com/start`.
- **Never** modify the user's real `~/.aws/config` or `~/.aws/credentials` during development; use temp dirs in tests.
- Do not run `aws sso login` or any AWS API calls as part of automated edits.
- Do not push to `main` or force-push without explicit instruction.
- Prefer minimal, focused diffs; do not reformat unrelated files.

## Useful references

- README: `README.md`
- License: `LICENSE` (Apache-2.0)
- CI: `.github/workflows/`
- Cursor rules: `.cursor/rules/`
