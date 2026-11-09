# Spec Delta

## REMOVED Requirements

### Requirement: Release notes from the changelog
**Reason**: The release description is now generated from the commits, pull requests and closed issues in the release range, so it no longer equals the `CHANGELOG.md` section.
**Migration**: The description format is specified by `release-automation` ("Generated release notes", "Closed issues in release notes"). The check that `CHANGELOG.md` has a non-empty section for the tag before anything is published is kept as `release-automation` "Changelog section required for release". Re-running publish for an existing release still uploads assets to it (see "Release artifacts").

## MODIFIED Requirements

### Requirement: Release artifacts
A `vX.Y.Z` tag SHALL publish binaries for darwin/amd64, darwin/arm64, linux/amd64 and linux/arm64, plus a SHA-256 checksums file covering every binary, whether the tag was pushed by hand or created by the release automation. The workflow SHALL authenticate with the workflow token scoped to `contents: write`. It SHALL fail when creating the release or uploading assets fails for any reason other than the release already existing.

#### Scenario: Checksums published
- **WHEN** tag `v1.2.3` is pushed
- **THEN** the release contains the four binaries and a checksums file with one SHA-256 line per binary

#### Scenario: Upload failure
- **WHEN** uploading an asset fails
- **THEN** the release workflow fails

#### Scenario: Release already exists
- **WHEN** a release for the tag already exists
- **THEN** its assets are uploaded with replacement and the workflow does not fail because the release exists
