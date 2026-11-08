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
- **Go** (version from `go.mod`) only if you build from source.

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

All commands that take `--profile` expect the name of a profile from `~/.aws/config` (without the `profile ` prefix).

### **Console Command (`console`)**

Opens the AWS Management Console with SSO credentials in the default web browser.

#### Usage:
```bash
aws-sso-login console --profile <profile-name> [flags]
```

#### Flags:
- `--profile` (required): Name of the AWS SSO profile.
- `--force-logout` (optional, default `true`): Open the console logout page before logging in, so an existing console session does not block the new one.
- `--logout-wait` (optional, default `1`): Seconds to wait after the logout page before opening the sign-in page.

The console opens in the profile's `sso_region`.

#### Example:
```bash
aws-sso-login console --profile dev-account
```

---

### **Export Command (`export`)**

Prints credentials for the specified profile as shell `export` statements.

#### Usage:
```bash
aws-sso-login export --profile <profile-name>
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

Fetches credentials for the specified profile and writes them to `~/.aws/credentials` under a section with the same name as the profile. Existing sections are updated in place.

#### Usage:
```bash
aws-sso-login import [--profile <profile-name>]
```

`--profile` is optional. When provided, the import runs non-interactively against that profile. When omitted, an interactive picker is presented (see below).

#### Example (direct profile):
```bash
aws-sso-login import --profile dev-account
```

#### Interactive selection

When `--profile` is omitted on macOS or Linux, the tool reads `~/.aws/config`, filters to profiles that have `sso_start_url` set, sorts them alphabetically, and presents a picker:

```
? Select an AWS SSO profile:
  > dev-account                     123456789012  AdminAccess
    prod-readonly                   123456789012  ReadOnlyAccess
    staging-engineer                123456789012  EngineerAccess
```

Use the arrow keys to move and `Enter` to confirm. The selected profile is then imported exactly as if `--profile <name>` had been passed.

> **Note:** Only profiles with `sso_start_url` present in `~/.aws/config` are shown.
> The interactive picker requires a TTY (macOS and Linux). Windows is untested.

---

### **Process Command (`process`)**

Prints `credential_process`-compatible JSON for the specified profile. It never starts an interactive login: if the SSO session has expired, it fails and you need to run `aws sso login --profile <profile-name>` first.

#### Usage:
```bash
aws-sso-login process --profile <profile-name>
```

#### Example `~/.aws/config` usage:
```ini
[profile dev-account-process]
credential_process = aws-sso-login process --profile dev-account
```

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

> **Limitations:**
> - `import` always writes to `~/.aws/credentials`; `AWS_SHARED_CREDENTIALS_FILE` is not honored yet.

---

## **Logging**

The project uses [Logrus](https://github.com/sirupsen/logrus).

- Logs are written to `stderr`, so `stdout` of `export` and `process` stays machine-readable.
- The log level is `info`. Debug-level messages, such as raw AWS CLI stderr, are not shown because there is no flag to change the level yet.

---

## **Error handling**

- **Recoverable errors** (for example, an expired SSO token): `import` and `console` run `aws sso login` once and retry credential retrieval. If the login itself fails, the error is reported and the command exits. `export` and `process` do not start a login.
- **Unrecoverable errors** (the SSO role is not assigned to your user; AWS returns `ForbiddenException` / `AccessDeniedException` from `GetRoleCredentials`): the CLI does **not** attempt `aws sso login`. It exits immediately with a single line saying the role is unavailable and asking you to contact your AWS administrator.
- All commands exit with status `1` on failure.

---

## **Development**

| Task                | Command              |
| ------------------- | -------------------- |
| Install deps        | `make install-deps`  |
| Run tests           | `make tests`         |
| Coverage report     | `make cover`         |
| Build (host)        | `make build`         |
| Build all platforms | `make build-all`     |

Binaries are written to `./tmp/`. See [AGENTS.md](AGENTS.md) for repository layout and coding conventions. Behavior changes are planned with [OpenSpec](https://github.com/Fission-AI/OpenSpec); project rules are in [`openspec/config.yaml`](openspec/config.yaml).

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
