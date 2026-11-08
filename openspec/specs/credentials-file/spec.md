# credentials-file Specification

## Purpose
Defines where and how `import` writes role credentials into the AWS shared credentials file, so the file stays private, intact and compatible with AWS CLI v2.

## Requirements

### Requirement: Credentials file location
`import` SHALL write to the path in `AWS_SHARED_CREDENTIALS_FILE` when it is set, otherwise to `~/.aws/credentials`. A missing parent directory SHALL be created with mode `0700`.

#### Scenario: Environment override
- **WHEN** `AWS_SHARED_CREDENTIALS_FILE` is set to a path in a temp directory and the user runs `import --profile dev`
- **THEN** credentials are written to that path and `~/.aws/credentials` is not touched

#### Scenario: Missing directory
- **WHEN** the parent directory of the credentials file does not exist
- **THEN** it is created with mode `0700` and the credentials file is written

### Requirement: Private file permissions
The credentials file SHALL have mode `0600` after every `import`, whether it was newly created or already existed with looser permissions.

#### Scenario: New file
- **WHEN** the credentials file does not exist and `import --profile dev` succeeds
- **THEN** the file is created with mode `0600`

#### Scenario: Existing world-readable file
- **WHEN** the credentials file exists with mode `0644` and `import --profile dev` succeeds
- **THEN** the file has mode `0600` afterwards

### Requirement: Atomic update preserving other profiles
`import` SHALL replace the credentials file atomically: the complete new content SHALL be written to a temp file in the same directory and then renamed over the target. Sections other than the imported profile SHALL be preserved. When the target is a symlink, the file it points to SHALL be updated.

#### Scenario: Other profiles preserved
- **WHEN** the credentials file contains sections `[a]` and `[b]` and the user imports profile `dev`
- **THEN** `[a]` and `[b]` are unchanged and `[dev]` holds `aws_access_key_id`, `aws_secret_access_key`, `aws_session_token` and `aws_security_token`

#### Scenario: Write failure leaves the original intact
- **WHEN** writing the temp file fails
- **THEN** the original credentials file is unchanged, no temp file is left behind, and the exit status is 1

#### Scenario: Symlinked credentials file
- **WHEN** the credentials path is a symlink to another file
- **THEN** the symlink target is updated and the symlink remains a symlink

### Requirement: Import result reporting
After a successful write, `import` SHALL report the profile name and credentials file path on stderr. It SHALL NOT print credential values anywhere.

#### Scenario: Successful import message
- **WHEN** `import --profile dev` succeeds
- **THEN** stderr contains the profile name `dev` and the credentials file path
- **AND** neither stdout nor stderr contains the access key, secret key or session token
