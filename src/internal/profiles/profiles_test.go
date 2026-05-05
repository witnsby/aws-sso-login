package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestListSSOProfiles_HappyPath(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile zebra-sso]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = ReadOnly
sso_region = us-east-1

[profile alpha-sso]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Admin
sso_region = eu-west-1

[profile non-sso]
region = us-west-2
`)
	profiles, err := ListSSOProfiles(path)
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	assert.Equal(t, "alpha-sso", profiles[0].Name)
	assert.Equal(t, "123456789012", profiles[0].AccountID)
	assert.Equal(t, "Admin", profiles[0].RoleName)
	assert.Equal(t, "eu-west-1", profiles[0].Region)
	assert.Equal(t, "https://example.awsapps.com/start", profiles[0].StartURL)

	assert.Equal(t, "zebra-sso", profiles[1].Name)
}

func TestListSSOProfiles_DefaultProfileIncluded(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[default]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = DefaultRole
sso_region = us-east-1

[profile named-profile]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = NamedRole
sso_region = us-west-2
`)
	profiles, err := ListSSOProfiles(path)
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	names := []string{profiles[0].Name, profiles[1].Name}
	assert.Contains(t, names, "default")
	assert.Contains(t, names, "named-profile")
}

func TestListSSOProfiles_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")

	_, err := ListSSOProfiles(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading aws config")
}

func TestListSSOProfiles_EmptyFile(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "")

	_, err := ListSSOProfiles(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no SSO-enabled profiles found")
	assert.Contains(t, err.Error(), path)
}

func TestListSSOProfiles_NonSSOOnly(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile plain]
region = us-east-1
output = json
`)
	_, err := ListSSOProfiles(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no SSO-enabled profiles found")
	assert.Contains(t, err.Error(), path)
}

func TestListSSOProfiles_MalformedINI(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile broken
sso_start_url = https://example.awsapps.com/start
`)
	_, err := ListSSOProfiles(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading aws config")
}

func TestListSSOProfiles_SortedCaseInsensitive(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile Zulu]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Role
sso_region = us-east-1

[profile alpha]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Role
sso_region = us-east-1

[profile Beta]
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Role
sso_region = us-east-1
`)
	profiles, err := ListSSOProfiles(path)
	require.NoError(t, err)
	require.Len(t, profiles, 3)

	assert.Equal(t, "alpha", profiles[0].Name)
	assert.Equal(t, "Beta", profiles[1].Name)
	assert.Equal(t, "Zulu", profiles[2].Name)
}
