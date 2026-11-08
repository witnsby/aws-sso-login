package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixClock pins now() for the duration of the test.
func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	orig := now
	t.Cleanup(func() { now = orig })
	now = func() time.Time { return at }
}

func TestIsExpired(t *testing.T) {
	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	fixClock(t, base)

	cases := []struct {
		name       string
		expiration string
		want       bool
	}{
		{"unparsable", "not a time", true},
		{"empty", "", true},
		{"already expired", base.Add(-time.Minute).Format(time.RFC3339), true},
		{"two minutes left", base.Add(2 * time.Minute).Format(time.RFC3339), true},
		{"just under the margin", base.Add(minCredentialValidity - time.Second).Format(time.RFC3339), true},
		{"exactly the margin", base.Add(minCredentialValidity).Format(time.RFC3339), false},
		{"thirty minutes left", base.Add(30 * time.Minute).Format(time.RFC3339), false},
		{"AWS CLI UTC suffix", base.Add(time.Hour).Format("2006-01-02T15:04:05UTC"), false},
		{"numeric offset", base.Add(time.Hour).In(time.FixedZone("x", 3600)).Format("2006-01-02T15:04:05-0700"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isExpired(tc.expiration))
		})
	}
}

// TestGetRoleCredentials_Refresh covers specs/credential-retrieval "Expiry
// margin" scenarios.
func TestGetRoleCredentials_Refresh(t *testing.T) {
	base := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	t.Run("near-expiry credentials are refreshed", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		fixClock(t, base)
		p := mustResolve(t, "dev")
		writeCachedCreds(t, p, base.Add(2*time.Minute))

		runner := installFakeRunner(t, func(name string, args []string) fakeResult {
			return fakeResult{
				Stdout: "arn:aws:sts::123456789012:assumed-role/Admin/user",
				Before: func() { writeCachedCreds(t, p, base.Add(time.Hour)) },
			}
		})

		rc, err := getRoleCredentials("dev", p, true)
		require.NoError(t, err)
		assert.Equal(t, base.Add(time.Hour).Format(time.RFC3339), rc.Expiration)

		calls := runner.CallsMatching("aws sts get-caller-identity")
		require.Len(t, calls, 1)
		assert.Contains(t, calls[0].Argv(), "--profile dev")
	})

	t.Run("comfortably valid credentials skip the AWS CLI", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		fixClock(t, base)
		p := mustResolve(t, "dev")
		writeCachedCreds(t, p, base.Add(30*time.Minute))
		runner := installFakeRunner(t, nil)

		_, err := getRoleCredentials("dev", p, true)
		require.NoError(t, err)
		assert.Empty(t, runner.Calls())
	})

	t.Run("refresh still near expiry fails", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		fixClock(t, base)
		p := mustResolve(t, "dev")
		writeCachedCreds(t, p, base.Add(2*time.Minute))
		installFakeRunner(t, func(string, []string) fakeResult { return fakeResult{} })

		_, err := getRoleCredentials("dev", p, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "credentials for 'dev' are expired")
	})
}
