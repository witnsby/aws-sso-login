package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expiredTokenStderr = "Error when retrieving token from sso: Token has expired and refresh failed"

// loginScript scripts the AWS CLI for the login retry flow. stsResults are
// returned for successive `aws sts get-caller-identity` calls; loginExit is
// the exit code of `aws sso login`.
func loginScript(t *testing.T, stsResults []fakeResult, loginExit int) *fakeRunner {
	t.Helper()
	sts := 0
	return installFakeRunner(t, func(name string, args []string) fakeResult {
		argv := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.HasPrefix(argv, "aws sso login"):
			return fakeResult{Stdout: "Open https://device.sso.example/ and enter CODE-1234", Exit: loginExit}
		case strings.HasPrefix(argv, "aws sts get-caller-identity"):
			require.Less(t, sts, len(stsResults), "unexpected extra sts call")
			r := stsResults[sts]
			sts++
			return r
		}
		t.Fatalf("unexpected command %q", argv)
		return fakeResult{}
	})
}

func TestLoginRetryFlow(t *testing.T) {
	expired := fakeResult{Stderr: expiredTokenStderr, Exit: 255}

	t.Run("expired token triggers one login and a retry", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		p := mustResolve(t, "dev")
		ok := fakeResult{Stdout: "arn", Before: func() { writeCachedCreds(t, p, time.Now().Add(time.Hour)) }}
		runner := loginScript(t, []fakeResult{expired, ok}, 0)

		_, rc, err := retrieveCredentials("dev", true)
		require.NoError(t, err)
		assert.Equal(t, fakeAccessKey, rc.AccessKeyId)

		logins := runner.CallsMatching("aws sso login")
		require.Len(t, logins, 1)
		assert.Equal(t, "aws sso login --profile dev", logins[0].Argv())
		// Instructions reach the user on stderr; stdout stays clean.
		assert.Same(t, os.Stdin, logins[0].Cmd.Stdin)
		assert.Same(t, os.Stderr, logins[0].Cmd.Stdout)
		assert.Same(t, os.Stderr, logins[0].Cmd.Stderr)
	})

	t.Run("failed login is not retried", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		runner := loginScript(t, []fakeResult{expired}, 1)

		_, _, err := retrieveCredentials("dev", true)
		require.Error(t, err)
		assert.Contains(t, rendered(err), "aws sso login failed for profile [dev]")
		assert.Contains(t, rendered(err), "Fix: Check the AWS CLI output above and try again.")
		assert.Equal(t, exitNotSignedIn, asUserError(err).code)
		assert.Len(t, runner.CallsMatching("aws sso login"), 1)
	})

	t.Run("retry after login still fails", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		runner := loginScript(t, []fakeResult{expired, expired}, 0)

		_, _, err := retrieveCredentials("dev", true)
		require.Error(t, err)
		assert.Contains(t, rendered(err), "aws sso login --profile dev")
		assert.Equal(t, exitNotSignedIn, asUserError(err).code)
		assert.Len(t, runner.CallsMatching("aws sso login"), 1)
	})

	t.Run("role not assigned skips login", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		noAccess := fakeResult{
			Stderr: "An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access",
			Exit:   255,
		}
		runner := loginScript(t, []fakeResult{noAccess}, 0)

		_, _, err := retrieveCredentials("dev", true)
		require.ErrorIs(t, err, errSSORoleNoAccess)
		assert.Empty(t, runner.CallsMatching("aws sso login"))
	})

	t.Run("unrelated no access text logs in during import", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		p := mustResolve(t, "dev")
		proxy := fakeResult{Stderr: "proxy error: no access to upstream host", Exit: 255}
		ok := fakeResult{Stdout: "arn", Before: func() { writeCachedCreds(t, p, time.Now().Add(time.Hour)) }}
		runner := loginScript(t, []fakeResult{proxy, ok}, 0)

		require.NoError(t, importCreds("dev"))
		assert.Len(t, runner.CallsMatching("aws sso login --profile dev"), 1)
	})

	t.Run("unrelated no access text advises login during export", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		proxy := fakeResult{Stderr: "proxy error: no access to upstream host", Exit: 255}
		runner := loginScript(t, []fakeResult{proxy}, 0)
		out := captureStdout(t)

		err := exportCredsToOutput("dev")
		require.Error(t, err)
		assert.NotErrorIs(t, err, errSSORoleNoAccess)
		assert.Contains(t, rendered(err), "aws sso login --profile dev")
		assert.Equal(t, exitNotSignedIn, asUserError(err).code)
		assert.Empty(t, out.String())
		assert.Empty(t, runner.CallsMatching("aws sso login"))
	})

	t.Run("export and process never log in", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		runner := loginScript(t, []fakeResult{expired, expired}, 0)

		err := exportCredsToOutput("dev")
		require.Error(t, err)
		assert.Contains(t, rendered(err), "aws sso login --profile dev")
		assert.Equal(t, exitNotSignedIn, asUserError(err).code)

		err = processCreds("dev")
		require.Error(t, err)
		assert.Contains(t, rendered(err), "aws sso login --profile dev")
		assert.Equal(t, exitNotSignedIn, asUserError(err).code)
		assert.Empty(t, runner.CallsMatching("aws sso login"))
	})
}
