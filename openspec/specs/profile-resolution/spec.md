# profile-resolution Specification

## Purpose
Defines how every command turns a profile name (from `--profile`, `AWS_PROFILE` or the interactive picker) into a validated AWS SSO configuration read from the AWS config file, for both legacy and sso-session profiles.

## Requirements

### Requirement: Profile name source precedence
Every command SHALL choose the profile name from, in order: a non-empty `--profile` flag, then a non-empty `AWS_PROFILE` environment variable, then the interactive picker (only where interactive selection is allowed). The `--profile` flag SHALL be accepted by every subcommand with the same name and empty default.

#### Scenario: Explicit --profile wins over AWS_PROFILE
- **WHEN** the user runs `aws-sso-login export --profile dev` with `AWS_PROFILE=prod` set
- **THEN** credentials for profile `dev` are exported
- **AND** the picker is not shown

#### Scenario: AWS_PROFILE used when the flag is omitted
- **WHEN** the user runs `aws-sso-login console` with `AWS_PROFILE=dev` set and no `--profile`
- **THEN** the console sign-in uses profile `dev`
- **AND** the picker is not shown

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

### Requirement: Profile section lookup
A profile name SHALL map to its AWS config section the same way for every command and for the picker: `default` SHALL map to `[default]`, falling back to `[profile default]`. Every other name SHALL map to `[profile <name>]`. The config file SHALL be read from `AWS_CONFIG_FILE` when set, otherwise from `~/.aws/config`.

#### Scenario: Default profile via --profile
- **WHEN** `~/.aws/config` contains an SSO-enabled `[default]` section and the user runs `aws-sso-login import --profile default`
- **THEN** credentials are imported for the `default` profile

#### Scenario: Default profile via the picker
- **WHEN** the picker lists `default` and the user selects it
- **THEN** credentials are imported for the `default` profile

#### Scenario: AWS_CONFIG_FILE honored
- **WHEN** `AWS_CONFIG_FILE` points to a file that defines `[profile dev]`
- **THEN** `--profile dev` resolves from that file and `~/.aws/config` is not read

#### Scenario: Missing profile
- **WHEN** the user runs any command with `--profile missing` and no such section exists
- **THEN** stderr reports that profile `missing` was not found in the config file path
- **AND** stdout is empty and the exit status is 1

### Requirement: Legacy SSO profiles
A profile that has no `sso_session` key SHALL be valid only when it defines `sso_start_url`, `sso_region`, `sso_account_id` and `sso_role_name`.

#### Scenario: Complete legacy profile
- **WHEN** `[profile dev]` defines `sso_start_url`, `sso_region`, `sso_account_id` and `sso_role_name`
- **THEN** the profile resolves using those values

#### Scenario: Incomplete legacy profile
- **WHEN** `[profile dev]` defines `sso_start_url` but not `sso_role_name`
- **THEN** stderr reports that `sso_role_name` is missing in profile `dev`
- **AND** the exit status is 1

### Requirement: sso-session profiles
A profile with an `sso_session` key SHALL read `sso_start_url` and `sso_region` from the `[sso-session <name>]` section, and `sso_account_id` and `sso_role_name` from the profile. A missing session section or key, or a profile-level `sso_start_url` or `sso_region` that differs from the session's value, SHALL be an error.

#### Scenario: Complete sso-session profile
- **WHEN** `[profile dev]` has `sso_session = corp`, `sso_account_id` and `sso_role_name`, and `[sso-session corp]` has `sso_start_url` and `sso_region`
- **THEN** the profile resolves with the start URL and region from `[sso-session corp]`

#### Scenario: Referenced session section missing
- **WHEN** `[profile dev]` has `sso_session = corp` and there is no `[sso-session corp]` section
- **THEN** stderr reports that sso-session `corp` referenced by profile `dev` was not found
- **AND** the exit status is 1

#### Scenario: Conflicting start URL
- **WHEN** `[profile dev]` has `sso_session = corp` and an `sso_start_url` that differs from the one in `[sso-session corp]`
- **THEN** stderr reports the conflicting `sso_start_url` values for profile `dev`
- **AND** the exit status is 1

### Requirement: Picker lists all SSO-enabled profiles
The picker SHALL list every profile that resolves as a legacy or sso-session SSO profile, including `default`, sorted case-insensitively by name, and showing name, account ID and role name. When there are none, the command SHALL fail with an error naming the config file.

#### Scenario: Mixed profile formats
- **WHEN** the config file defines one legacy SSO profile, one sso-session profile and one non-SSO profile
- **THEN** the picker lists the two SSO profiles and not the non-SSO profile

#### Scenario: No SSO profiles
- **WHEN** the config file defines no SSO-enabled profiles and the picker would be shown
- **THEN** stderr reports that no SSO-enabled profiles were found in the config file path
- **AND** the exit status is 1

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

### Requirement: Picker filters as the user types
The picker SHALL open with its filter input focused and SHALL show the hint `Type to filter by name, account ID or role`. While the user types, the picker SHALL list only the profiles whose row (name, account ID and role name) contains the typed text, compared case-insensitively. `Esc` SHALL clear the filter. The filter input and hint SHALL be drawn on stderr.

#### Scenario: Filter by name fragment
- **WHEN** the picker lists `dev`, `hipaa-vg-prod-ro` and `hipaa-vg-prod-rw` and the user types `vg-prod`
- **THEN** the picker lists `hipaa-vg-prod-ro` and `hipaa-vg-prod-rw` and not `dev`

#### Scenario: Filter by account ID
- **WHEN** the user types part of an account ID such as `1234567`
- **THEN** the picker lists only the profiles whose account ID contains that text

#### Scenario: Filter by role name, case-insensitively
- **WHEN** the user types `readonly` and a profile has role `ReadOnlyAccess`
- **THEN** that profile is listed

#### Scenario: Filter matches both profile formats
- **WHEN** the config file defines a legacy SSO profile `legacy-dev` and an sso-session profile `session-dev` and the user types `dev`
- **THEN** the picker lists both profiles

#### Scenario: No profile matches
- **WHEN** the user types text that no row contains
- **THEN** the picker lists no profiles
- **AND** pressing `Esc` clears the filter and lists every SSO profile again

#### Scenario: Filtered picker keeps export eval-safe
- **WHEN** the user runs `eval "$(aws-sso-login export)"` on a terminal, types a filter and selects a profile
- **THEN** the picker, the filter input and the hint are drawn on stderr
- **AND** stdout contains only the `export` lines

#### Scenario: Explicit profile skips the filter
- **WHEN** the user runs `aws-sso-login import --profile dev`
- **THEN** no picker and no filter input are shown

#### Scenario: Cancelled while filtering
- **WHEN** the user types a filter and then presses `Ctrl+C`
- **THEN** stderr reports that profile selection was cancelled
- **AND** stdout is empty and the exit status is 130

### Requirement: Picker height is bounded
The picker SHALL be at most 15 lines tall, filter input and hint included, and SHALL scroll when more profiles match than fit.

#### Scenario: Long profile list scrolls
- **WHEN** the config file defines more SSO profiles than fit in 15 lines
- **THEN** the picker is 15 lines tall
- **AND** the arrow keys scroll through the remaining profiles

#### Scenario: Short profile list
- **WHEN** the config file defines two SSO profiles
- **THEN** the picker lists both without scrolling
