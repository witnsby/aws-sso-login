package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// Golden values computed independently with:
//
//	printf '%s' '<json>' | shasum
func TestCacheKey(t *testing.T) {
	cases := []struct {
		name    string
		profile profiles.Profile
		want    string
	}{
		{
			name: "legacy profile uses startUrl",
			profile: profiles.Profile{AccountID: "123456789012", RoleName: "Admin",
				StartURL: "https://example.awsapps.com/start"},
			// {"accountId":"123456789012","roleName":"Admin","startUrl":"https://example.awsapps.com/start"}
			want: "764b1819cd322a32dff569462ca869a4196b4fc1",
		},
		{
			name: "sso-session profile uses sessionName",
			profile: profiles.Profile{AccountID: "123456789012", RoleName: "Admin",
				StartURL: "https://example.awsapps.com/start", SessionName: "corp"},
			// {"accountId":"123456789012","roleName":"Admin","sessionName":"corp"}
			want: "8457bc68cf9d4aa8d629a61563948e8bd3159d49",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cacheKey(tc.profile)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCacheKey_NoHTMLEscaping(t *testing.T) {
	// Python's json.dumps keeps '&' as is; encoding/json would escape it
	// to & by default and produce a different hash.
	p := profiles.Profile{AccountID: "1", RoleName: "r", StartURL: "https://x/start?a=1&b=2"}
	got, err := cacheKey(p)
	require.NoError(t, err)
	// printf '%s' '{"accountId":"1","roleName":"r","startUrl":"https://x/start?a=1&b=2"}' | shasum
	assert.Equal(t, "345db3919329e76374c0c67dc574e5bf07c206f5", got)
}

func TestGetCachedRoleCredentials(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	p := mustResolve(t, "dev")
	path, err := buildCacheFilePath(p)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))

	t.Run("valid file", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte(`{"Credentials":{"AccessKeyId":"AK","SecretAccessKey":"SK","SessionToken":"ST","Expiration":"2030-01-01T00:00:00Z"}}`), 0o600))
		rc, err := getCachedRoleCredentials(p)
		require.NoError(t, err)
		assert.Equal(t, "AK", rc.AccessKeyId)
		assert.Equal(t, "2030-01-01T00:00:00Z", rc.Expiration)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte(`{not json`), 0o600))
		_, err := getCachedRoleCredentials(p)
		require.Error(t, err)
	})

	t.Run("missing file", func(t *testing.T) {
		require.NoError(t, os.Remove(path))
		_, err := getCachedRoleCredentials(p)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

// TestSSOSessionCacheHit covers specs/credential-retrieval "sso-session
// profile cache hit": cached credentials are found without the AWS CLI.
func TestSSOSessionCacheHit(t *testing.T) {
	setupAWSEnv(t, `[profile dev]
sso_session = corp
sso_account_id = 123456789012
sso_role_name = Admin

[sso-session corp]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`)
	runner := installFakeRunner(t, nil)
	p := mustResolve(t, "dev")
	writeCachedCreds(t, p, now().Add(time.Hour))

	rc, err := getRoleCredentials("dev", p)
	require.NoError(t, err)
	assert.Equal(t, fakeAccessKey, rc.AccessKeyId)
	assert.Empty(t, runner.Calls())
}

// TestProcess_StartURLWithHash covers legacy start URLs copied from the
// browser with a trailing '#': the cache key must be built from the full URL
// like the AWS CLI does (issue #39).
func TestProcess_StartURLWithHash(t *testing.T) {
	setupAWSEnv(t, `[profile dev]
sso_start_url = https://example.awsapps.com/start/#
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Admin
`)
	runner := installFakeRunner(t, nil)

	// printf '%s' '{"accountId":"123456789012","roleName":"Admin","startUrl":"https://example.awsapps.com/start/#"}' | shasum
	cacheFile := filepath.Join(os.Getenv("HOME"), ".aws", "cli", "cache", "15df9758d7856a6f29cc30ca915a0f94c688ebff.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(cacheFile), 0o700))
	payload := fmt.Sprintf(`{"Credentials":{"AccessKeyId":%q,"SecretAccessKey":%q,"SessionToken":%q,"Expiration":%q}}`,
		fakeAccessKey, fakeSecretKey, fakeSessionToken, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	require.NoError(t, os.WriteFile(cacheFile, []byte(payload), 0o600))

	out := captureStdout(t)
	require.NoError(t, execute(t, "process", "--profile", "dev"))

	assert.Contains(t, out.String(), fakeAccessKey)
	assert.Empty(t, runner.Calls(), "the AWS CLI must not be invoked on a cache hit")
}
