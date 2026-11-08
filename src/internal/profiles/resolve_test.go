package profiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const resolveFixture = `
[default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = DefaultRole

[profile legacy]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Admin
region = eu-west-1

[profile incomplete]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012

[profile dev]
sso_session = corp
sso_account_id = 123456789012
sso_role_name = Admin
region = eu-central-1

[profile dev-same-url]
sso_session = corp
sso_start_url = https://example.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Admin

[profile dev-conflict]
sso_session = corp
sso_start_url = https://other.awsapps.com/start
sso_account_id = 123456789012
sso_role_name = Admin

[profile dev-region-conflict]
sso_session = corp
sso_region = eu-west-1
sso_account_id = 123456789012
sso_role_name = Admin

[profile orphan]
sso_session = missing
sso_account_id = 123456789012
sso_role_name = Admin

[profile half-session]
sso_session = half
sso_account_id = 123456789012
sso_role_name = Admin

[profile session-no-role]
sso_session = corp
sso_account_id = 123456789012

[profile plain]
region = us-west-2

[sso-session corp]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1

[sso-session half]
sso_start_url = https://example.awsapps.com/start
`

func TestResolve(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, resolveFixture)

	cases := []struct {
		name    string
		profile string
		want    Profile
		wantErr string
	}{
		{
			name:    "default maps to [default]",
			profile: "default",
			want: Profile{Name: "default", AccountID: "123456789012", RoleName: "DefaultRole",
				StartURL: "https://example.awsapps.com/start", SSORegion: "us-east-1"},
		},
		{
			name:    "complete legacy profile",
			profile: "legacy",
			want: Profile{Name: "legacy", AccountID: "123456789012", RoleName: "Admin",
				StartURL: "https://example.awsapps.com/start", SSORegion: "us-east-1", Region: "eu-west-1"},
		},
		{
			name:    "incomplete legacy profile",
			profile: "incomplete",
			wantErr: `missing required attribute "sso_role_name" in profile incomplete`,
		},
		{
			name:    "complete sso-session profile",
			profile: "dev",
			want: Profile{Name: "dev", AccountID: "123456789012", RoleName: "Admin",
				StartURL: "https://example.awsapps.com/start", SSORegion: "us-east-1",
				Region: "eu-central-1", SessionName: "corp"},
		},
		{
			name:    "sso-session profile with matching start URL",
			profile: "dev-same-url",
			want: Profile{Name: "dev-same-url", AccountID: "123456789012", RoleName: "Admin",
				StartURL: "https://example.awsapps.com/start", SSORegion: "us-east-1", SessionName: "corp"},
		},
		{
			name:    "conflicting start URL",
			profile: "dev-conflict",
			wantErr: `conflicting sso_start_url values for profile "dev-conflict"`,
		},
		{
			name:    "conflicting region",
			profile: "dev-region-conflict",
			wantErr: `conflicting sso_region values for profile "dev-region-conflict"`,
		},
		{
			name:    "referenced session missing",
			profile: "orphan",
			wantErr: `sso-session "missing" referenced by profile "orphan" not found`,
		},
		{
			name:    "session missing region",
			profile: "half-session",
			wantErr: `missing required attribute "sso_region" in sso-session "half"`,
		},
		{
			name:    "sso-session profile missing role",
			profile: "session-no-role",
			wantErr: `missing required attribute "sso_role_name" in profile session-no-role`,
		},
		{
			name:    "non-SSO profile",
			profile: "plain",
			wantErr: `missing required attribute "sso_start_url" in profile plain`,
		},
		{
			name:    "missing profile",
			profile: "missing",
			wantErr: `profile "missing" not found in ` + path,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Resolve(path, tc.profile)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolve_DefaultFallsBackToProfileDefault(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Fallback
`)
	got, err := Resolve(path, "default")
	require.NoError(t, err)
	assert.Equal(t, "Fallback", got.RoleName)
}

func TestResolve_DefaultPrefersBareSection(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Fallback

[default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Preferred
`)
	got, err := Resolve(path, "default")
	require.NoError(t, err)
	assert.Equal(t, "Preferred", got.RoleName)

	list, err := ListSSOProfiles(path)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Preferred", list[0].RoleName)
}

func TestResolve_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := Resolve(t.TempDir()+"/nope", "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading aws config")
}

func TestListSSOProfiles_MixedFormats(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, resolveFixture)

	list, err := ListSSOProfiles(path)
	require.NoError(t, err)

	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	// Valid legacy, sso-session and default profiles are listed; invalid
	// ones (incomplete, conflicts, orphan, half-session, session-no-role)
	// and non-SSO profiles are skipped.
	assert.Equal(t, []string{"default", "dev", "dev-same-url", "legacy"}, names)

	for _, p := range list {
		if p.Name == "dev" {
			assert.Equal(t, "corp", p.SessionName)
			assert.Equal(t, "https://example.awsapps.com/start", p.StartURL)
		}
	}
}

// TestResolve_ReadmeExample keeps the sso-session example in README.md
// (Configuration section) working.
func TestResolve_ReadmeExample(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[profile dev-account]
sso_session = my-sso
sso_account_id = 123456789012
sso_role_name = DeveloperAccess
region = eu-west-1

[sso-session my-sso]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`)
	got, err := Resolve(path, "dev-account")
	require.NoError(t, err)
	assert.Equal(t, "my-sso", got.SessionName)
	assert.Equal(t, "us-east-1", got.SSORegion)
	assert.Equal(t, "eu-west-1", got.Region)
}
