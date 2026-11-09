## ADDED Requirements

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
