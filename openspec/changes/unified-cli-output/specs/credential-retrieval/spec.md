# Spec Delta

## MODIFIED Requirements

### Requirement: Missing AWS CLI
When the `aws` executable cannot be found in `PATH`, every command that needs credentials SHALL fail with an error stating that AWS CLI v2 is required but `aws` was not found in `PATH`, with a `Fix:` line pointing to the AWS CLI v2 installation guide, and exit with status 5. It SHALL NOT advise running `aws sso login` and SHALL NOT attempt a login.

#### Scenario: AWS CLI not installed
- **WHEN** the cached credentials are missing and `aws` is not in `PATH` while running `import --profile dev`
- **THEN** stderr reports that AWS CLI v2 is required but `aws` was not found in `PATH`
- **AND** the `Fix:` line points to the AWS CLI v2 installation guide
- **AND** no `aws sso login` is attempted and the exit status is 5

### Requirement: Login on expired SSO session for interactive commands
When credentials cannot be refreshed for a reason other than missing role access or a missing AWS CLI, `import` and `console` SHALL write a step line naming the profile, run `aws sso login --profile <name>` once, then retry retrieval once. The login's stdin SHALL be the terminal, and its output SHALL go to stderr unchanged and unbuffered so the authorization URL and code are visible as soon as the AWS CLI prints them. A successful login SHALL be followed by a success line.

#### Scenario: Expired SSO token during import
- **WHEN** the SSO token for profile `dev` has expired and the user runs `import --profile dev`
- **THEN** a step line naming `dev` is written before `aws sso login --profile dev` starts
- **AND** `aws sso login --profile dev` runs once with its URL and verification code shown on stderr
- **AND** a success line saying the SSO sign-in succeeded follows it
- **AND** after a successful login the credentials are retrieved again and imported

#### Scenario: Expired SSO token after choosing a profile in the picker
- **WHEN** the user runs `import` on a terminal without `--profile` or `AWS_PROFILE`, picks `dev`, and the SSO token for `dev` has expired
- **THEN** the step line names `dev` and `aws sso login --profile dev` runs once

#### Scenario: Login fails
- **WHEN** `aws sso login` exits with a non-zero status
- **THEN** stderr reports that `aws sso login` failed for the profile, with a `Fix:` line asking to check the AWS CLI output above and try again
- **AND** no second login is attempted and the exit status is 3

#### Scenario: Retry after login still fails
- **WHEN** login succeeds but retrieving credentials fails again
- **THEN** the error is reported once on stderr and the exit status is the one defined for that failure

### Requirement: No login for non-interactive commands
`export` and `process` SHALL NOT run `aws sso login`. When credentials cannot be refreshed because the SSO session has expired, they SHALL fail with an error saying the user is not signed in to AWS SSO for the profile, with a `Fix:` line advising `aws sso login --profile <name>`, and exit with status 3.

#### Scenario: Expired SSO token during process
- **WHEN** the SSO token for profile `dev` has expired and `process --profile dev` runs
- **THEN** stderr advises running `aws sso login --profile dev`
- **AND** stdout is empty and the exit status is 3

### Requirement: Role not assigned
When the AWS CLI stderr names `ForbiddenException` or `AccessDeniedException` (case-insensitive), every command SHALL fail immediately with one actionable error block and exit with status 4, and no login SHALL be attempted. The block SHALL name the profile, the account ID and the role name from the profile, and SHALL tell the user to ask their AWS administrator to assign the role or to choose another profile. Stderr that does not name either exception SHALL NOT be treated as a missing role assignment, even when it contains the words "no access".

#### Scenario: Role not assigned during console
- **WHEN** the configured role is not assigned to the user and `console --profile dev` runs
- **THEN** stderr shows a single error block saying the SSO role is not assigned and to ask an administrator
- **AND** `aws sso login` is not run and the exit status is 4

#### Scenario: Role not assigned for an sso-session profile
- **WHEN** profile `dev` uses `sso_session = corp` with `sso_account_id = 123456789012` and `sso_role_name = ReadOnly`, the role is not assigned, and `import --profile dev` runs
- **THEN** the error block names `123456789012` and `ReadOnly`
- **AND** the exit status is 4

#### Scenario: Role not assigned for a legacy profile
- **WHEN** legacy profile `dev` has `sso_start_url = https://example.awsapps.com/start`, `sso_account_id = 123456789012` and `sso_role_name = ReadOnly`, the role is not assigned, and `export --profile dev` runs
- **THEN** the error block names `123456789012` and `ReadOnly`, stdout is empty and the exit status is 4

#### Scenario: AWS CLI unassigned-role message
- **WHEN** the AWS CLI stderr is `An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access` during `process --profile dev`
- **THEN** stderr shows the single "role not assigned" line, stdout is empty and the exit status is 4

#### Scenario: Unrelated "no access" text during import
- **WHEN** the AWS CLI refresh fails with stderr `proxy error: no access to upstream host` during `import --profile dev`
- **THEN** the failure is not reported as a missing role assignment
- **AND** `aws sso login --profile dev` runs once, as for any other refresh failure

#### Scenario: Unrelated "no access" text during export
- **WHEN** the AWS CLI refresh fails with stderr `proxy error: no access to upstream host` during `export --profile dev`
- **THEN** stderr advises running `aws sso login --profile dev`
- **AND** stdout is empty and the exit status is 3
