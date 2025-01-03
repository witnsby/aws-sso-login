# **AWS SSO Login Utility (aws-sso-login)**

This is a Go-based command-line tool designed to simplify AWS SSO (Single Sign-On) authentication and credentials management. This utility allows developers, engineers, and DevOps professionals to log into the AWS Management Console, fetch and export credentials, and automate workflows using credential process-compatible JSON. It provides a seamless experience for working with AWS profiles and credentials.

## **Features**

### 1. Console Login
- Logs into the AWS Web Console using SSO and opens the session in the default web browser.
- Allows forced logouts of existing sessions.
- Automatically constructs secure sign-in URLs.

### 2. Export Credentials
- Exports AWS credentials for a specified profile in a shell-exportable format.
- Outputs environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, and others).

### 3. Import Credentials
- Fetches and saves credentials for a specified profile into the AWS credentials file.
- Ensures the credentials file is properly updated.

### 4. Credential Process JSON
- Outputs JSON payload compatible with AWS SDK's `credential_process` feature.
- Useful for programmatically authenticating AWS profiles in custom applications or scripts.

---

## **Table of Contents**
- [Installation](#installation)
- [Prerequisites](#prerequisites)
- [Usage](#usage)
    - [Console Command (`console`)](#console-command-console)
    - [Export Command (`export`)](#export-command-export)
    - [Import Command (`import`)](#import-command-import)
    - [Process Command (`process`)](#process-command-process)
- [Configuration](#configuration)
- [Logging](#logging)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## **Installation**

1. Clone the repository:

   ```bash
   git clone https://github.com/witnsby/aws-sso-login.git
   cd aws-sso-login
   ```

2. Build the binary:

   ```bash
   go build -o aws-sso-login .
   ```

3. Move the binary to a directory in your `PATH` (optional):

   ```bash
   mv aws-sso-login /usr/local/bin
   ```

Now, you can use the `aws-sso-login` command globally from your terminal!

---

## **Prerequisites**

- **Go 1.23 or newer** must be installed on your system ([Install Go](https://golang.org/doc/install)).
- Ensure you have an existing AWS SSO profile configured (`~/.aws/config` and `~/.aws/credentials` files).
- SSO permissions must allow access to retrieve credentials and sign-in tokens for your AWS account.

---

## **Usage**

### **Console Command (`console`)**

Opens the AWS Management Console with SSO authentication in the default web browser.

#### Usage:
```bash
aws-sso-login console --profile <profile-name> [flags]
```

#### Flags:
- `--profile` (required): Name of the AWS SSO profile.
- `--force-logout` (optional): Logout of any existing session before login (default: true).
- `--logout-wait` (optional): Time (in seconds) to wait after logout before logging in again.

#### Example:
```bash
aws-sso-login console --profile dev-account --force-logout
```

---

### **Export Command (`export`)**

Exports credentials for the specified AWS profile in a shell-exportable format.

#### Usage:
```bash
aws-sso-login export --profile <profile-name>
```

#### Description:
This command fetches the credentials for the specified AWS profile and outputs them as environment variables.

#### Example:
```bash
aws-sso-login export --profile dev-account
```

Shell-compatible output:
```bash
export AWS_ACCESS_KEY_ID=<AccessKeyId>
export AWS_SECRET_ACCESS_KEY=<SecretAccessKey>
export AWS_SESSION_TOKEN=<SessionToken>
export AWS_DEFAULT_REGION=<Region>
```

---

### **Import Command (`import`)**

Fetches credentials for the specified AWS profile and writes them to the AWS credentials file.

#### Usage:
```bash
aws-sso-login import --profile <profile-name>
```

#### Description:
This command writes the credentials to the AWS credentials file (`~/.aws/credentials`) under the specified profile.

#### Example:
```bash
aws-sso-login import --profile dev-account
```

---

### **Process Command (`process`)**

Generates credential process-compatible JSON output for the specified AWS profile.

#### Usage:
```bash
aws-sso-login process --profile <profile-name>
```

#### Example:
```bash
aws-sso-login process --profile dev-account
```

#### Sample Output:
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

## **Configuration**

AWS SSO profiles are configured in your AWS CLI configuration files (`~/.aws/config` and `~/.aws/credentials`). Ensure the following properties are set up for each profile:

1. `sso_start_url`: The AWS SSO URL for your organization.
2. `sso_region`: The AWS region for the SSO service.
3. `sso_account_id`: The Account ID associated with the profile.
4. `sso_role_name`: The IAM role name assigned for SSO login.

Example AWS CLI config file (`~/.aws/config`):
```ini
[profile dev-account]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = DeveloperAccess
```

---

## **Logging**

This project uses the [Logrus](https://github.com/sirupsen/logrus) logging library for structured logging.

- Logs are output to `stdout` by default for both errors and normal operations.
- Debugging information is logged for all major processes (e.g., credential retrieval, profile validation).

---

## **Contributing**

We welcome contributions to improve this project! Please follow these steps to contribute:

1. Fork the repository.
2. Create a new branch for your feature or bug fix.
3. Write tests (if applicable) and ensure no existing functionality is broken.
4. Submit a detailed pull request for review.

---

## **License**

This project is licensed under the [Apache License](LICENSE). Feel free to use, modify, and distribute it.
