# Spec Delta

## MODIFIED Requirements

### Requirement: Interactive picker availability
When no profile name comes from the flag or `AWS_PROFILE`, `console`, `export` and `import` SHALL show the interactive picker only if both stdin and stderr are terminals. The picker SHALL render on stderr. Otherwise these commands, and `process` in every case, SHALL fail with an error that asks for `--profile` or `AWS_PROFILE` and exit with status 2. A cancelled picker SHALL exit with status 130.

#### Scenario: Picker on a terminal
- **WHEN** the user runs `aws-sso-login import` on a terminal with no `--profile` and no `AWS_PROFILE`
- **THEN** the picker lists the SSO-enabled profiles on stderr
- **AND** credentials for the chosen profile are imported

#### Scenario: Picker keeps export eval-safe
- **WHEN** the user runs `eval "$(aws-sso-login export)"` on a terminal with no `--profile` and no `AWS_PROFILE`
- **THEN** the picker is drawn on stderr
- **AND** stdout contains only the `export` lines for the chosen profile

#### Scenario: Non-interactive invocation without a profile
- **WHEN** `aws-sso-login import` runs with stdin not attached to a terminal and with no `--profile` and no `AWS_PROFILE`
- **THEN** stderr reports that `--profile` must be specified, with a `Fix:` line naming `--profile <name>` and `AWS_PROFILE`
- **AND** stdout is empty and the exit status is 2

#### Scenario: process never prompts
- **WHEN** `aws-sso-login process` runs on a terminal with no `--profile` and no `AWS_PROFILE`
- **THEN** no picker is shown
- **AND** stderr reports that `--profile` must be specified, stdout is empty and the exit status is 2

#### Scenario: Picker cancelled
- **WHEN** the user cancels the picker
- **THEN** stderr reports that profile selection was cancelled
- **AND** stdout is empty and the exit status is 130
