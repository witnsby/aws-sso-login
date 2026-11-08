package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMissingAWSCLI covers specs/credential-retrieval "AWS CLI not
// installed": a clear error and no login attempt.
func TestMissingAWSCLI(t *testing.T) {
	t.Run("not in PATH", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		runner := installFakeRunner(t, nil)
		missingExecutables(t, "aws")

		err := importCreds("dev")
		require.ErrorIs(t, err, errAWSCLINotFound)
		assert.Equal(t, `AWS CLI v2 is required but "aws" was not found in PATH`, err.Error())
		assert.NotContains(t, err.Error(), "aws sso login")
		assert.Empty(t, runner.Calls(), "no AWS CLI command may run")
	})

	t.Run("exec reports not found", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		runner := installFakeRunner(t, func(string, []string) fakeResult {
			return fakeResult{NotFound: true}
		})

		err := importCreds("dev")
		require.ErrorIs(t, err, errAWSCLINotFound)
		assert.Empty(t, runner.CallsMatching("aws sso login"))
	})

	t.Run("valid cache needs no AWS CLI", func(t *testing.T) {
		setupAWSEnv(t, legacyConfig)
		installFakeRunner(t, nil)
		missingExecutables(t, "aws")
		writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))

		require.NoError(t, exportCredsToOutput("dev"))
	})
}
