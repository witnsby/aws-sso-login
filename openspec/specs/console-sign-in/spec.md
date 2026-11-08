# console-sign-in Specification

## Purpose
Defines how `console` turns role credentials into an AWS Management Console sign-in: the federation token request, optional logout, region choice and opening the browser on macOS and Linux.

## Requirements

### Requirement: Federation sign-in token request
`console` SHALL request a sign-in token from the AWS federation endpoint with a bounded timeout. A non-200 status, a malformed body or an empty `SigninToken` SHALL be an error. Error messages SHALL NOT contain the request URL, the response body or any credential value.

#### Scenario: Successful token
- **WHEN** the federation endpoint returns HTTP 200 with a non-empty `SigninToken`
- **THEN** the console sign-in URL is built with that token and opened

#### Scenario: Non-200 response
- **WHEN** the federation endpoint returns HTTP 400 with an HTML body
- **THEN** stderr reports that the federation endpoint returned status 400
- **AND** stderr does not contain the request URL, the body or any credential value, and the exit status is 1

#### Scenario: Empty token
- **WHEN** the federation endpoint returns HTTP 200 with an empty `SigninToken`
- **THEN** stderr reports that the federation endpoint returned no sign-in token and the exit status is 1

#### Scenario: Endpoint does not respond
- **WHEN** the federation endpoint accepts the connection but never responds
- **THEN** the command fails after the timeout with an error that does not contain the request URL, and the exit status is 1

### Requirement: Optional logout
`console` SHALL open the console logout page before signing in only when `--force-logout` is true (its default). `--logout-wait` SHALL only set the number of seconds to wait after that logout. A value of 0 or less SHALL mean no wait.

#### Scenario: Default behavior
- **WHEN** the user runs `console --profile dev` without logout flags
- **THEN** the logout page opens, the command waits 1 second, and then the sign-in page opens

#### Scenario: Logout disabled
- **WHEN** the user runs `console --profile dev --force-logout=false`
- **THEN** only the sign-in page opens and there is no wait

#### Scenario: Logout without wait
- **WHEN** the user runs `console --profile dev --logout-wait=0`
- **THEN** the logout page opens and the sign-in page opens immediately after

### Requirement: Console region
The console destination and logout pages SHALL use the profile's `region` when it is set, otherwise its `sso_region`. The sign-in URL SHALL target the profile's `sso_account_id`.

#### Scenario: Profile region set
- **WHEN** profile `dev` has `region = eu-west-1` and `sso_region = us-east-1`
- **THEN** the console opens in `eu-west-1`

#### Scenario: Profile region unset
- **WHEN** profile `dev` has no `region` and `sso_region = us-east-1`
- **THEN** the console opens in `us-east-1`

### Requirement: Opening the browser
`console` SHALL open URLs with `open` on macOS and `xdg-open` on Linux, using only the command for the current OS. When that command is missing or fails to start, the URL SHALL be printed to stderr with a request to open it manually. Stdout SHALL stay empty.

#### Scenario: Linux with xdg-open
- **WHEN** `console` runs on Linux and `xdg-open` is available
- **THEN** the sign-in URL is opened with `xdg-open` and `open` is never invoked

#### Scenario: Linux without xdg-open
- **WHEN** `console` runs on Linux and `xdg-open` is not in `PATH`
- **THEN** stderr asks the user to open the sign-in URL manually and shows it
- **AND** `open` is not invoked and stdout is empty

#### Scenario: macOS
- **WHEN** `console` runs on macOS
- **THEN** the sign-in URL is opened with `open`
