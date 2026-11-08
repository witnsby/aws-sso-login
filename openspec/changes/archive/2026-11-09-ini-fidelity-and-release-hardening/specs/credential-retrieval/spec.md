# Spec Delta

## MODIFIED Requirements

### Requirement: AWS CLI cache lookup
Role credentials SHALL be read from `~/.aws/cli/cache/<key>.json`, where `<key>` is the SHA-1 hex digest of compact JSON with sorted keys. The JSON SHALL hold `accountId`, `roleName` and `startUrl` for legacy profiles, and `accountId`, `roleName` and `sessionName` for sso-session profiles, matching AWS CLI v2. `startUrl` SHALL be the configured `sso_start_url` exactly as the AWS CLI reads it, including any `#` or `;`.

#### Scenario: Legacy profile cache hit
- **WHEN** a legacy profile resolves and the cache file keyed by its `accountId`, `roleName` and `startUrl` holds credentials valid for more than 5 minutes
- **THEN** those credentials are used and the AWS CLI is not invoked

#### Scenario: Legacy start URL with a trailing hash
- **WHEN** a legacy profile has `sso_start_url = https://example.awsapps.com/start/#` and the cache file keyed by `startUrl` `https://example.awsapps.com/start/#` holds valid credentials
- **THEN** `process --profile dev` prints those credentials on stdout and the AWS CLI is not invoked

#### Scenario: sso-session profile cache hit
- **WHEN** an sso-session profile with session `corp` resolves and the cache file keyed by its `accountId`, `roleName` and `sessionName` `corp` holds valid credentials
- **THEN** those credentials are used and the AWS CLI is not invoked

### Requirement: Role not assigned
When the AWS CLI stderr names `ForbiddenException` or `AccessDeniedException` (case-insensitive), every command SHALL fail immediately with one actionable message, and no login SHALL be attempted. Stderr that does not name either exception SHALL NOT be treated as a missing role assignment, even when it contains the words "no access".

#### Scenario: Role not assigned during console
- **WHEN** the configured role is not assigned to the user and `console --profile dev` runs
- **THEN** stderr shows a single message saying the SSO role is not assigned and to ask an administrator
- **AND** `aws sso login` is not run and the exit status is 1

#### Scenario: AWS CLI unassigned-role message
- **WHEN** the AWS CLI stderr is `An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access` during `process --profile dev`
- **THEN** stderr shows the single "role not assigned" message, stdout is empty and the exit status is 1

#### Scenario: Unrelated "no access" text during import
- **WHEN** the AWS CLI refresh fails with stderr `proxy error: no access to upstream host` during `import --profile dev`
- **THEN** the failure is not reported as a missing role assignment
- **AND** `aws sso login --profile dev` runs once, as for any other refresh failure

#### Scenario: Unrelated "no access" text during export
- **WHEN** the AWS CLI refresh fails with stderr `proxy error: no access to upstream host` during `export --profile dev`
- **THEN** stderr advises running `aws sso login --profile dev`
- **AND** stdout is empty and the exit status is 1
