package cli

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportDefaultProfile covers specs/profile-resolution "Default profile
// via --profile": [default] resolves for import and credentials are written.
func TestImportDefaultProfile(t *testing.T) {
	env := setupAWSEnv(t, legacyConfig)
	runner := installFakeRunner(t, nil)
	writeCachedCreds(t, mustResolve(t, "default"), time.Now().Add(time.Hour))

	require.NoError(t, importCreds("default"))

	assert.Empty(t, runner.Calls(), "valid cache must not invoke the AWS CLI")
	data, err := os.ReadFile(env.CredentialsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[default]")
	assert.Contains(t, string(data), fakeAccessKey)
}
