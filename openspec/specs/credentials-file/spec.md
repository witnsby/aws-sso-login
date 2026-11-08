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
`import` SHALL replace the credentials file atomically: the complete new content SHALL be written to a temp file in the same directory and then renamed over the target. Sections other than the imported profile SHALL keep their keys, values and `key = value` lines unchanged, and keys SHALL NOT be padded into aligned columns. When the target is a symlink, the file it points to SHALL be updated.

#### Scenario: Other profiles preserved
- **WHEN** the credentials file contains sections `[a]` and `[b]` and the user imports profile `dev`
- **THEN** `[a]` and `[b]` are unchanged and `[dev]` holds `aws_access_key_id`, `aws_secret_access_key`, `aws_session_token` and `aws_security_token`

#### Scenario: Untouched section keeps its formatting
- **WHEN** the credentials file contains `[other]` with the lines `aws_access_key_id = AKIDEXAMPLE` and `region = eu-west-1` and the user runs `import --profile dev`
- **THEN** `[other]` still contains exactly the lines `aws_access_key_id = AKIDEXAMPLE` and `region = eu-west-1`, with no padding before `=`

#### Scenario: Imported section written without alignment
- **WHEN** `import --profile dev` writes the `[dev]` section
- **THEN** each of its keys is written as `key = value` with a single space on each side of `=`

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

### Requirement: Credentials values read like the AWS CLI
`import` SHALL read the existing credentials file the way AWS CLI v2 reads it: `#` and `;` after the start of a value and surrounding double quotes SHALL be part of the value, and SHALL be written back unchanged. Only lines whose first non-blank character is `#` or `;` SHALL be comments.

#### Scenario: Inline hash and semicolon in another profile
- **WHEN** `[other]` contains `aws_secret_access_key = abc#def;ghi` and `credential_process = sh -c "a; b"`, and the user runs `import --profile dev`
- **THEN** `[other]` still contains `aws_secret_access_key = abc#def;ghi` and `credential_process = sh -c "a; b"` exactly, and no new comment lines are added

#### Scenario: Quoted value in another profile
- **WHEN** `[other]` contains `credential_process = "/opt/my tool/bin" --profile x` and the user runs `import --profile dev`
- **THEN** that line is written back unchanged, including both double quotes

#### Scenario: Re-importing a profile with special characters elsewhere
- **WHEN** `import --profile dev` runs twice against a file whose `[other]` section has values containing `#` and `;`
- **THEN** after the second run `[other]` is identical to the original
- **AND** stdout is empty and stderr reports the profile name and credentials file path without credential values
