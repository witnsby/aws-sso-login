# Spec Delta

## ADDED Requirements

### Requirement: Config values read like the AWS CLI
Values in the AWS config file SHALL be read the way AWS CLI v2 reads them: `#` and `;` after the start of a value SHALL be part of the value, and surrounding double quotes SHALL be kept. Only lines whose first non-blank character is `#` or `;` SHALL be comments. This SHALL apply to legacy profiles, sso-session profiles, `[sso-session <name>]` sections and the picker alike.

#### Scenario: Legacy start URL ending in a hash
- **WHEN** `[profile dev]` has `sso_start_url = https://example.awsapps.com/start/#` and the user runs `export --profile dev`
- **THEN** the profile resolves with the start URL `https://example.awsapps.com/start/#`, including the trailing `#`

#### Scenario: sso-session start URL ending in a hash
- **WHEN** `[profile dev]` has `sso_session = corp` and `[sso-session corp]` has `sso_start_url = https://example.awsapps.com/start/#`, and `[profile dev]` repeats the same `sso_start_url`
- **THEN** the profile resolves without a conflicting-start-URL error

#### Scenario: Semicolon inside a value
- **WHEN** `[profile dev]` has `sso_role_name = Admin;Ops`
- **THEN** the role name is `Admin;Ops`

#### Scenario: Full-line comments ignored
- **WHEN** the config file contains lines `# note` and `; note` between keys of `[profile dev]`
- **THEN** those lines are ignored and the profile resolves from its keys

#### Scenario: Picker with an inline hash
- **WHEN** the user runs `import` on a terminal with no `--profile` and no `AWS_PROFILE`, and the only SSO profile has `sso_start_url = https://example.awsapps.com/start/#`
- **THEN** the picker on stderr lists that profile and importing it uses the start URL including the `#`

#### Scenario: Missing profile still reported
- **WHEN** the config file contains values with inline `#` and the user runs `process --profile missing`
- **THEN** stderr reports that profile `missing` was not found in the config file path
- **AND** stdout is empty and the exit status is 1
