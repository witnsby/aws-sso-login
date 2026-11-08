# **AWS SSO Login Utility (aws-sso-login)**

A Go command-line tool that simplifies AWS IAM Identity Center (AWS SSO) authentication and credential management. It lets developers, engineers and DevOps professionals log into the AWS Management Console, export or import temporary credentials, and feed credentials to AWS SDKs through `credential_process`.

## **Features**

### 1. Console Login
- Logs into the AWS Web Console using SSO credentials and opens the session in the default web browser.
- Optionally logs out of an existing console session first.
- Builds the federation sign-in URL automatically.

### 2. Export Credentials
- Prints AWS credentials for a profile as shell `export` statements.
- Sets `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_SECURITY_TOKEN` and, when the profile has a `region`, `AWS_DEFAULT_REGION`.

### 3. Import Credentials
- Fetches credentials for a profile and saves them into the AWS credentials file.
- When `--profile` is omitted, presents an interactive picker of SSO-enabled profiles (macOS/Linux).

### 4. Credential Process JSON
- Prints JSON compatible with the AWS SDK `credential_process` setting.

---

## **Table of Contents**
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
    - [Console Command (`console`)](#console-command-console)
    - [Export Command (`export`)](#export-command-export)
    - [Import Command (`import`)](#import-command-import)
    - [Process Command (`process`)](#process-command-process)
    - [Version Command (`version`)](#version-command-version)
- [Configuration](#configuration)
- [Logging](#logging)
- [Error handling](#error-handling)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## **Prerequisites**

- **AWS CLI v2** installed and available in `PATH`. `aws-sso-login` calls `aws sso login` and `aws sts get-caller-identity` and reads the credentials the AWS CLI caches in `~/.aws/cli/cache`.
- At least one SSO profile configured in `~/.aws/config` (see [Configuration](#configuration)).
- Your SSO user must be assigned to the account and permission set (role) used by the profile.
- **Go 1.27 or newer** (the `go` line in `go.mod`) only if you build from source. CI and releases use the toolchain pinned by the `toolchain` line; with the default `GOTOOLCHAIN=auto` Go downloads it for you.

Supported platforms: macOS and Linux on amd64 and arm64. Windows is untested.

---

## **Installation**

### Prebuilt binaries

Download the binary for your platform from the [GitHub releases page](https://github.com/witnsby/aws-sso-login/releases) (`aws-sso-login_darwin_arm64`, `aws-sso-login_darwin_amd64`, `aws-sso-login_linux_amd64`, `aws-sso-login_linux_arm64`), then make it executable and move it into your `PATH`:

```bash
chmod +x aws-sso-login_darwin_arm64
mv aws-sso-login_darwin_arm64 /usr/local/bin/aws-sso-login
```

### Build from source

```bash
git clone https://github.com/witnsby/aws-sso-login.git
cd aws-sso-login
make build
mv ./tmp/aws-sso-login /usr/local/bin/
```

`make build` builds for the host platform and embeds the version from the latest git tag. Without `make`, run `go build -o aws-sso-login ./src/cmd/bin`.

---

## **Usage**

All commands that take `--profile` expect the name of a profile from `~/.aws/config` (without the `profile ` prefix). The profile is chosen in this order:

1. `--profile <name>`, accepted by every command.
2. The `AWS_PROFILE` environment variable.
3. An interactive picker (`console`, `export` and `import` only), shown when stdin and stderr are both terminals. The picker is drawn on stderr, so `eval "$(aws-sso-login export)"` stays safe.

Otherwise the command fails with `must specify --profile`. `process` never prompts.

### **Console Command (`console`)**

Opens the AWS Management Console with SSO credentials in the default web browser.

#### Usage:
```bash
aws-sso-login console [--profile <profile-name>] [flags]
```

#### Flags:
- `--profile` (optional): Name of the AWS SSO profile. Defaults to `AWS_PROFILE`, then the picker.
- `--force-logout` (optional, default `true`): Open the console logout page before logging in, so an existing console session does not block the new one. Pass `--force-logout=false` to skip it.
- `--logout-wait` (optional, default `1`): Seconds to wait after the logout page before opening the sign-in page. Only used when `--force-logout` is on.

The console opens in the profile's `region` when it is set, otherwise in its `sso_region`. URLs are opened with `open` on macOS and `xdg-open` on Linux; if neither is available (for example on a headless server), the sign-in URL is printed to stderr so you can open it yourself. The federation sign-in request times out after 15 seconds.

#### Example:
```bash
aws-sso-login console --profile dev-account
```

---

### **Export Command (`export`)**

Prints credentials for the specified profile as shell `export` statements.

#### Usage:
```bash
aws-sso-login export [--profile <profile-name>]
```

#### Example:
Load the credentials into the current shell:

```bash
eval "$(aws-sso-login export --profile dev-account)"
```

Output format:
```bash
export AWS_ACCESS_KEY_ID="<AccessKeyId>"
export AWS_SECRET_ACCESS_KEY="<SecretAccessKey>"
export AWS_SESSION_TOKEN="<SessionToken>"
export AWS_SECURITY_TOKEN="<SessionToken>"
export AWS_DEFAULT_REGION="<region>"
```

`AWS_DEFAULT_REGION` is printed only when the profile has a `region` key.

---

### **Import Command (`import`)**

Fetches credentials for the specified profile and writes them to the shared credentials file (`~/.aws/credentials` or `AWS_SHARED_CREDENTIALS_FILE`) under a section with the same name as the profile. Existing sections are updated in place.

#### Usage:
```bash
aws-sso-login import [--profile <profile-name>]
```

`--profile` is optional. When provided (or when `AWS_PROFILE` is set), the import runs non-interactively against that profile. When both are omitted on a terminal, an interactive picker is presented (see below).

#### Example (direct profile):
```bash
aws-sso-login import --profile dev-account
```

#### Interactive selection

When neither `--profile` nor `AWS_PROFILE` is set on macOS or Linux, `console`, `export` and `import` read the AWS config file, keep the SSO profiles (legacy and sso-session, including `default`), sort them alphabetically, and present a picker on stderr:

```
? Select an AWS SSO profile:
  > dev-account                     123456789012  AdminAccess
    prod-readonly                   123456789012  ReadOnlyAccess
    staging-engineer                123456789012  EngineerAccess
```

Use the arrow keys to move and `Enter` to confirm. The selected profile is then imported exactly as if `--profile <name>` had been passed.

> **Note:** Profiles with missing or conflicting SSO settings are not shown.
> The interactive picker requires stdin and stderr to be terminals (macOS and Linux). Windows is untested.

---

### **Process Command (`process`)**

Prints `credential_process`-compatible JSON for the specified profile. It never starts an interactive login: if the SSO session has expired, it fails and you need to run `aws sso login --profile <profile-name>` first.

#### Usage:
```bash
aws-sso-login process [--profile <profile-name>]
```

#### Example `~/.aws/config` usage:
```ini
[profile dev-account-process]
credential_process = aws-sso-login process --profile dev-account
```

Always pass `--profile` in `credential_process`: the AWS SDK passes your shell's `AWS_PROFILE` to the process, which may name the `credential_process` profile itself rather than the SSO profile.

#### Sample output (formatted for readability; the tool prints a single line):
```json
{
  "Version": 1,
  "AccessKeyId": "<AccessKeyId>",
  "SecretAccessKey": "<SecretAccessKey>",
  "SessionToken": "<SessionToken>",
  "Expiration": "2023-12-01T01:23:45Z"
}
```

---

### **Version Command (`version`)**

Prints the version and commit the binary was built from.

```bash
aws-sso-login version
```

---

## **Configuration**

`aws-sso-login` reads profiles from the AWS CLI config file: `~/.aws/config`, or the path in `AWS_CONFIG_FILE`. Both profile formats written by AWS CLI v2 are supported. The `default` profile is read from `[default]` (or `[profile default]`); every other profile is read from `[profile <name>]`.

**sso-session profiles** (the default format of `aws configure sso`): the profile sets `sso_session`, `sso_account_id` and `sso_role_name`, and the referenced `[sso-session <name>]` section sets `sso_start_url` and `sso_region`.

```ini
[profile dev-account]
sso_session = my-sso
sso_account_id = 123456789012
sso_role_name = DeveloperAccess
region = eu-west-1

[sso-session my-sso]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
```

**Legacy profiles**: the profile itself sets `sso_start_url`, `sso_region`, `sso_account_id` and `sso_role_name`.

```ini
[profile dev-account]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = DeveloperAccess
region = eu-west-1
```

Optional in both formats:

- `region`: Default region, exported as `AWS_DEFAULT_REGION` by `export`.

`import` writes to the AWS shared credentials file: `~/.aws/credentials`, or the path in `AWS_SHARED_CREDENTIALS_FILE`. The file is replaced atomically and always has mode `0600`; a missing directory is created with mode `0700`. Other profiles in the file are kept. If the path is a symlink, the file it points to is updated; hard links to the file are not preserved.

---

## **Logging**

The project uses [Logrus](https://github.com/sirupsen/logrus).

- Logs are written to `stderr`, so `stdout` of `export` and `process` stays machine-readable.
- `process` and `export` log only warnings and errors by default; `console` and `import` also log progress (`info`).
- Pass `--debug` to any command, or set `AWS_SSO_LOGIN_DEBUG=1` (or `true`), to see debug logs such as the raw stderr of AWS CLI calls. Credentials and sign-in tokens are never logged.
- Each failure is reported once, as a single error line.

---

## **Error handling**

- **Recoverable errors** (for example, an expired SSO token): `import` and `console` run `aws sso login` once and retry credential retrieval. The AWS CLI's instructions (authorization URL and code) are shown on stderr, so you can finish the login in any browser when none opens automatically. If the login itself fails, the error is reported and the command exits. `export` and `process` do not start a login.
- **Unrecoverable errors** (the SSO role is not assigned to your user; AWS returns `ForbiddenException` / `AccessDeniedException` from `GetRoleCredentials`): the CLI does **not** attempt `aws sso login`. It exits immediately with a single line saying the role is unavailable and asking you to contact your AWS administrator.
- **Missing AWS CLI**: if `aws` is not in `PATH`, commands fail with `AWS CLI v2 is required but "aws" was not found in PATH`.
- All commands exit with status `1` on failure.

---

## **Development**

| Task                | Command                  |
| ------------------- | ------------------------ |
| Install deps        | `make install-deps`      |
| Run tests           | `make tests`             |
| Coverage report     | `make cover`             |
| Build (host)        | `make build`             |
| Build all platforms | `make build-all`         |
| Vet                 | `make vet`               |
| Format check        | `make fmt-check`         |
| Toolchain check     | `make check-toolchain`   |
| Action pin check    | `make check-action-pins` |
| Checksums           | `make checksums`         |
| Clean               | `make clean`             |

GitHub Actions in `.github/workflows/` are referenced by full commit SHA with the version in a comment (`uses: owner/repo@<sha> # vX.Y.Z`); CI runs `make check-action-pins` and fails on tag or branch references. Dependabot keeps the pins up to date.

Binaries are written to `./tmp/`. Each release also ships `aws-sso-login_checksums.txt`; verify a download with `sha256sum -c aws-sso-login_checksums.txt --ignore-missing` (or `shasum -a 256 -c` on macOS). See [AGENTS.md](AGENTS.md) for repository layout and coding conventions. Behavior changes are planned with [OpenSpec](https://github.com/Fission-AI/OpenSpec); project rules are in [`openspec/config.yaml`](openspec/config.yaml).

---

## **Contributing**

Contributions are welcome:

1. Fork the repository.
2. Create a branch for your feature or bug fix.
3. Add tests and make sure `make tests`, `go vet ./...` and `gofmt -l ./src` pass.
4. Update `README.md` and the `Unreleased` section of `CHANGELOG.md` for user-visible changes.
5. Open a pull request with a clear description.

---

## **License**

This project is licensed under the [Apache License 2.0](LICENSE).
