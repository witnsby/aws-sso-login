# Spec Delta

## Purpose

Defines how role credentials for a resolved SSO profile are obtained from the AWS CLI v2 cache, refreshed through the AWS CLI, and how login, expiry and failure conditions are handled.

## ADDED Requirements

### Requirement: AWS CLI cache lookup
Role credentials SHALL be read from `~/.aws/cli/cache/<key>.json`, where `<key>` is the SHA-1 hex digest of compact JSON with sorted keys. The JSON SHALL hold `accountId`, `roleName` and `startUrl` for legacy profiles, and `accountId`, `roleName` and `sessionName` for sso-session profiles, matching AWS CLI v2.

#### Scenario: Legacy profile cache hit
- **WHEN** a legacy profile resolves and the cache file keyed by its `accountId`, `roleName` and `startUrl` holds credentials valid for more than 5 minutes
- **THEN** those credentials are used and the AWS CLI is not invoked

#### Scenario: sso-session profile cache hit
- **WHEN** an sso-session profile with session `corp` resolves and the cache file keyed by its `accountId`, `roleName` and `sessionName` `corp` holds valid credentials
- **THEN** those credentials are used and the AWS CLI is not invoked

### Requirement: Expiry margin
Cached credentials SHALL count as expired when their expiration time cannot be parsed or is less than 5 minutes away. Expired credentials SHALL be refreshed by running `aws sts get-caller-identity --profile <name>` and re-reading the cache. No command SHALL return credentials that expire within 5 minutes.

#### Scenario: Credentials close to expiry are refreshed
- **WHEN** the cached credentials expire in 2 minutes
- **THEN** the AWS CLI refresh runs and the refreshed credentials are returned

#### Scenario: Credentials comfortably valid
- **WHEN** the cached credentials expire in 30 minutes
- **THEN** they are returned without running the AWS CLI

#### Scenario: Refresh still yields near-expiry credentials
- **WHEN** the refreshed cache still holds credentials that expire within 5 minutes
- **THEN** stderr reports that the credentials for the profile are expired
- **AND** stdout is empty and the exit status is 1

### Requirement: Missing AWS CLI
When the `aws` executable cannot be found in `PATH`, every command that needs credentials SHALL fail with an error stating that AWS CLI v2 is required but `aws` was not found in `PATH`. It SHALL NOT advise running `aws sso login` and SHALL NOT attempt a login.

#### Scenario: AWS CLI not installed
- **WHEN** the cached credentials are missing and `aws` is not in `PATH` while running `import --profile dev`
- **THEN** stderr reports that AWS CLI v2 is required but `aws` was not found in `PATH`
- **AND** no `aws sso login` is attempted and the exit status is 1

### Requirement: Login on expired SSO session for interactive commands
When credentials cannot be refreshed for a reason other than missing role access or a missing AWS CLI, `import` and `console` SHALL run `aws sso login --profile <name>` once, then retry retrieval once. The login's stdin SHALL be the terminal, and its output SHALL go to stderr so the authorization URL and code are visible.

#### Scenario: Expired SSO token during import
- **WHEN** the SSO token for profile `dev` has expired and the user runs `import --profile dev`
- **THEN** `aws sso login --profile dev` runs once with its URL and verification code shown on stderr
- **AND** after a successful login the credentials are retrieved again and imported

#### Scenario: Login fails
- **WHEN** `aws sso login` exits with a non-zero status
- **THEN** stderr reports that `aws sso login` failed for the profile
- **AND** no second login is attempted and the exit status is 1

#### Scenario: Retry after login still fails
- **WHEN** login succeeds but retrieving credentials fails again
- **THEN** the error is reported once on stderr and the exit status is 1

### Requirement: No login for non-interactive commands
`export` and `process` SHALL NOT run `aws sso login`. When credentials cannot be refreshed because the SSO session has expired, they SHALL fail with an error advising `aws sso login --profile <name>`.

#### Scenario: Expired SSO token during process
- **WHEN** the SSO token for profile `dev` has expired and `process --profile dev` runs
- **THEN** stderr advises running `aws sso login --profile dev`
- **AND** stdout is empty and the exit status is 1

### Requirement: Role not assigned
When the AWS CLI reports that the user has no access to the configured role (`ForbiddenException`, `AccessDeniedException` or a "no access" message), every command SHALL fail immediately with one actionable message. No login SHALL be attempted.

#### Scenario: Role not assigned during console
- **WHEN** the configured role is not assigned to the user and `console --profile dev` runs
- **THEN** stderr shows a single message saying the SSO role is not assigned and to ask an administrator
- **AND** `aws sso login` is not run and the exit status is 1
