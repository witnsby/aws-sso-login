# Spec Delta

## MODIFIED Requirements

### Requirement: Import result reporting
After a successful write, `import` SHALL report on stderr, as its last line and as a success line, the profile name, the credentials file path with the user's home directory shown as `~`, and when the credentials expire: the local time and the remaining duration rounded down to minutes. The date SHALL be added when the expiry is not on the current local day. It SHALL NOT print credential values anywhere.

#### Scenario: Successful import message
- **WHEN** `import --profile dev` succeeds
- **THEN** stderr contains the profile name `dev` and the credentials file path
- **AND** neither stdout nor stderr contains the access key, secret key or session token

#### Scenario: Expiry in the success line
- **WHEN** `import --profile dev` succeeds at 10:42 local time with credentials expiring at 18:42 local time the same day
- **THEN** the last stderr line is a success line that contains `18:42` and `8h0m`

#### Scenario: Expiry on another day
- **WHEN** `import --profile dev` succeeds at 23:30 local time with credentials expiring at 07:30 local time the next day
- **THEN** the success line contains the expiry date as well as `07:30`

#### Scenario: Home directory shortened
- **WHEN** `import --profile dev` writes to `$HOME/.aws/credentials`
- **THEN** the success line shows the path as `~/.aws/credentials`

#### Scenario: Custom credentials path shown in full
- **WHEN** `AWS_SHARED_CREDENTIALS_FILE=/tmp/creds` is set and `import --profile dev` succeeds
- **THEN** the success line shows the path `/tmp/creds`
