# Spec Delta

## Purpose

Defines the stdout and stderr contracts, log levels, debug switch and error reporting shared by all aws-sso-login commands, so output stays scriptable and failures are reported once.

## ADDED Requirements

### Requirement: Stdout contracts
`export` stdout SHALL contain only `export NAME="value"` lines safe to `eval` in POSIX shells. `process` stdout SHALL contain only the `credential_process` JSON with `Version` 1. `version` stdout SHALL contain one line. `console` and `import` SHALL write nothing to stdout. Everything else SHALL go to stderr.

#### Scenario: export output
- **WHEN** `export --profile dev` succeeds for a profile with `region = eu-west-1`
- **THEN** stdout contains exactly the `export` lines for `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_SECURITY_TOKEN` and `AWS_DEFAULT_REGION`

#### Scenario: process output
- **WHEN** `process --profile dev` succeeds
- **THEN** stdout is a single JSON object with `Version` 1, `AccessKeyId`, `SecretAccessKey`, `SessionToken` and `Expiration`, and nothing else

#### Scenario: Credentials refreshed during import
- **WHEN** `import --profile dev` refreshes credentials through the AWS CLI
- **THEN** the refresh notice is written to stderr and stdout stays empty

#### Scenario: version output
- **WHEN** the user runs `aws-sso-login version`
- **THEN** stdout is exactly one line with the version and commit, without a trailing blank line

### Requirement: Single error report
Every failure SHALL produce exactly one error message on stderr and exit with status 1. Intermediate steps SHALL NOT log the same error that is returned.

#### Scenario: Missing profile reported once
- **WHEN** `process --profile missing` fails because the profile does not exist
- **THEN** stderr contains exactly one error line about the missing profile
- **AND** the exit status is 1

### Requirement: Default log levels
`process` and `export` SHALL log only warnings and errors by default. `console` and `import` SHALL also log informational progress by default. Logs SHALL go to stderr.

#### Scenario: Quiet process
- **WHEN** `process --profile dev` succeeds without debug enabled
- **THEN** stderr is empty

#### Scenario: Informative import
- **WHEN** `import --profile dev` succeeds without debug enabled
- **THEN** stderr reports where the credentials were written

### Requirement: Debug switch
A persistent `--debug` flag on every command, or `AWS_SSO_LOGIN_DEBUG` set to `1` or `true`, SHALL enable debug-level logs on stderr. These include the raw stderr of AWS CLI calls. Debug output SHALL NOT contain access keys, secret keys, session tokens, sign-in tokens or SSO access tokens.

#### Scenario: Debug flag
- **WHEN** the user runs `process --profile dev --debug` and the AWS CLI refresh fails
- **THEN** stderr includes the AWS CLI stderr at debug level and stdout is empty

#### Scenario: Debug environment variable
- **WHEN** `AWS_SSO_LOGIN_DEBUG=1` is set and the user runs `export --profile dev`
- **THEN** debug logs appear on stderr and stdout contains only the `export` lines

#### Scenario: No secrets in debug logs
- **WHEN** any command runs with debug enabled
- **THEN** stderr contains none of the access key, secret key, session token or sign-in token values
