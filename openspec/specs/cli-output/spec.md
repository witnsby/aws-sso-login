# cli-output Specification

## Purpose
Defines the stdout and stderr contracts, log levels, debug switch and error reporting shared by all aws-sso-login commands, so output stays scriptable and failures are reported once.

## Requirements

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
Every failure SHALL produce exactly one error block on stderr and exit with the status defined in "Exit codes". Intermediate steps SHALL NOT log the same error that is returned.

#### Scenario: Missing profile reported once
- **WHEN** `process --profile missing` fails because the profile does not exist
- **THEN** stderr contains exactly one error line about the missing profile
- **AND** the exit status is 1

### Requirement: Default log levels
`process` and `export` SHALL write nothing to stderr on success and only warning and error lines on failure, by default. `console` and `import` SHALL also write step and success lines by default. All of these SHALL go to stderr.

#### Scenario: Quiet process
- **WHEN** `process --profile dev` succeeds without debug enabled
- **THEN** stderr is empty

#### Scenario: Informative import
- **WHEN** `import --profile dev` succeeds without debug enabled
- **THEN** stderr reports where the credentials were written

#### Scenario: Refresh notice without the caller ARN
- **WHEN** `import --profile dev` refreshes credentials through the AWS CLI without debug enabled
- **THEN** stderr contains a step line saying the credentials for `dev` were refreshed
- **AND** stderr does not contain the caller ARN or the account ID returned by the AWS CLI

### Requirement: Debug switch
A persistent `--debug` flag on every command, or `AWS_SSO_LOGIN_DEBUG` set to `1` or `true`, SHALL enable debug-level logs on stderr. Debug log lines SHALL carry a full timestamp. These include the raw stderr of AWS CLI calls, the caller ARN after a refresh, and the full wrapped error chain of a failure after its error block. Debug output SHALL NOT contain access keys, secret keys, session tokens, sign-in tokens or SSO access tokens.

#### Scenario: Debug flag
- **WHEN** the user runs `process --profile dev --debug` and the AWS CLI refresh fails
- **THEN** stderr includes the AWS CLI stderr at debug level and stdout is empty

#### Scenario: Debug environment variable
- **WHEN** `AWS_SSO_LOGIN_DEBUG=1` is set and the user runs `export --profile dev`
- **THEN** debug logs appear on stderr and stdout contains only the `export` lines

#### Scenario: Error chain in debug output
- **WHEN** `import --profile dev --debug` fails because the role is not assigned
- **THEN** stderr contains the error block and, after it, a debug line with the full wrapped error

#### Scenario: Debug timestamps
- **WHEN** any command runs with debug enabled
- **THEN** every debug log line includes a full date and time

#### Scenario: No secrets in debug logs
- **WHEN** any command runs with debug enabled
- **THEN** stderr contains none of the access key, secret key, session token or sign-in token values

### Requirement: Status line kinds
Human-readable status on stderr SHALL use four line kinds: step (an action that starts), success (a finished result), warning, and error. Each status line SHALL start with the prefix of its kind. Status lines SHALL never be written to stdout.

#### Scenario: Prefixes on a terminal
- **WHEN** stderr is a terminal, `NO_COLOR` is unset and `TERM` is not `dumb`
- **THEN** step lines start with `→ `, success lines with `✓ `, warning lines with `! ` and error lines with `✗ `
- **AND** each prefix is colored: step dim, success green, warning yellow, error red

#### Scenario: Prefixes without a terminal
- **WHEN** stderr is redirected to a file or a pipe
- **THEN** step lines start with `-> `, success lines with `ok: `, warning lines with `warning: ` and error lines with `error: `
- **AND** stderr contains no ANSI escape sequences

#### Scenario: Color disabled on a terminal
- **WHEN** stderr is a terminal and `NO_COLOR` is set to any non-empty value, or `TERM` is `dumb`
- **THEN** the terminal symbols `→`, `✓`, `!` and `✗` are used
- **AND** stderr contains no ANSI escape sequences

### Requirement: No log prefixes in normal output
Without debug enabled, stderr SHALL NOT contain log level markers such as `INFO[0014]` or `ERRO[0032]`, nor timestamps. Every line the tool itself writes to stderr SHALL be a status line, a continuation line of an error block, the interactive picker, or the manual-open browser fallback. Output of `aws sso login` SHALL be passed through unchanged.

#### Scenario: Import without debug
- **WHEN** `import --profile dev` refreshes credentials and succeeds without debug enabled
- **THEN** no stderr line matches `^(DEBU|INFO|WARN|ERRO|FATA)\[`

#### Scenario: Error without debug
- **WHEN** `export --profile missing` fails without debug enabled
- **THEN** stderr starts with the error prefix and contains no log level marker

### Requirement: Error block layout
A failure SHALL be rendered as one error block: a title line with the error prefix, then up to two detail lines, then a `Fix:` line with the next step when one is known. Detail and `Fix:` lines SHALL be indented by two spaces. The block SHALL NOT repeat the title text in parentheses or append the raw wrapped error.

#### Scenario: Role not assigned during import
- **WHEN** `import --profile dev` fails because the role is not assigned to the user, on a terminal
- **THEN** stderr ends with a line starting with `✗ ` that names profile `dev`
- **AND** the following indented lines name the account ID and role name from the profile and a `Fix:` line tells the user to ask their AWS administrator or choose another profile
- **AND** the phrase "no access to the configured SSO role" appears at most once

#### Scenario: Error without a known next step
- **WHEN** a command fails with an error that has no known next step, such as an unreadable credentials file
- **THEN** stderr contains only the error title line, with no `Fix:` line

### Requirement: One-line errors for process
`process` SHALL render a failure as exactly one stderr line: the error prefix, the title and, when known, `Fix:` followed by the next step, separated by spaces. Its stdout SHALL be empty on failure.

#### Scenario: process with an expired SSO session
- **WHEN** the SSO session for `dev` has expired and `process --profile dev` runs with stderr redirected
- **THEN** stderr is exactly one line starting with `error: ` that contains `Fix:` and `aws sso login --profile dev`
- **AND** stdout is empty

### Requirement: Exit codes
The process exit status SHALL identify the failure class: `0` success; `1` any failure without a more specific code; `2` usage error (unknown command or flag, invalid flag value, unexpected arguments, or no profile name where no picker may be shown); `3` not signed in to AWS SSO or `aws sso login` failed; `4` SSO role not assigned to the user; `5` AWS CLI not found in `PATH`; `130` profile picker cancelled.

#### Scenario: Success
- **WHEN** `process --profile dev` succeeds
- **THEN** the exit status is 0

#### Scenario: Unknown flag
- **WHEN** the user runs `import --no-such-flag`
- **THEN** stderr contains one error block about the unknown flag and the exit status is 2

#### Scenario: Missing profile in the config file
- **WHEN** `export --profile missing` runs and the profile is not in the config file
- **THEN** stderr reports that profile `missing` was not found and the exit status is 1

#### Scenario: Expired SSO session for export
- **WHEN** the SSO session for `dev` has expired and `export --profile dev` runs
- **THEN** the exit status is 3

#### Scenario: Role not assigned
- **WHEN** the configured role is not assigned to the user and `console --profile dev` runs
- **THEN** the exit status is 4

#### Scenario: AWS CLI missing
- **WHEN** `aws` is not in `PATH` and `import --profile dev` needs to refresh credentials
- **THEN** the exit status is 5

### Requirement: Success summary
On success, `import` and `console` SHALL end their stderr output with exactly one success line. For `console` it SHALL name the profile and the region the console opens in. `export`, `process` and `version` SHALL NOT print a success line.

#### Scenario: Console success
- **WHEN** `console --profile dev` succeeds for a profile with `region = eu-west-1`
- **THEN** the last stderr line is a success line naming `dev` and `eu-west-1`
- **AND** stdout is empty

#### Scenario: Console success via the picker
- **WHEN** the user runs `console` on a terminal without `--profile` or `AWS_PROFILE` and picks `dev` in the picker
- **THEN** the last stderr line is a success line naming `dev`

#### Scenario: Export success stays quiet
- **WHEN** `export --profile dev` succeeds without debug enabled
- **THEN** stderr is empty
