package cli

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExportOutput covers specs/cli-output "export output": only eval-safe
// export lines on stdout.
func TestExportOutput(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		want    []string
	}{
		{
			name:    "with region",
			profile: "dev",
			want: []string{
				`export AWS_ACCESS_KEY_ID="` + fakeAccessKey + `"`,
				`export AWS_SECRET_ACCESS_KEY="` + fakeSecretKey + `"`,
				`export AWS_SESSION_TOKEN="` + fakeSessionToken + `"`,
				`export AWS_SECURITY_TOKEN="` + fakeSessionToken + `"`,
				`export AWS_DEFAULT_REGION="eu-west-1"`,
			},
		},
		{
			name:    "without region",
			profile: "default",
			want: []string{
				`export AWS_ACCESS_KEY_ID="` + fakeAccessKey + `"`,
				`export AWS_SECRET_ACCESS_KEY="` + fakeSecretKey + `"`,
				`export AWS_SESSION_TOKEN="` + fakeSessionToken + `"`,
				`export AWS_SECURITY_TOKEN="` + fakeSessionToken + `"`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupAWSEnv(t, legacyConfig)
			installFakeRunner(t, nil)
			writeCachedCreds(t, mustResolve(t, tc.profile), time.Now().Add(time.Hour))
			logs := captureLogs(t)
			out := captureStdout(t)

			require.NoError(t, execute(t, "export", "--profile", tc.profile))
			assert.Equal(t, strings.Join(tc.want, "\n")+"\n", out.String())
			assert.Empty(t, logs.String())
		})
	}
}

// TestExportQuotingIsEvalSafe checks that values with shell metacharacters
// survive `eval` in a POSIX shell unchanged.
func TestExportQuotingIsEvalSafe(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no POSIX shell available")
	}
	out := captureStdout(t)
	tricky := `a"b\c` + "`d`" + ` $HOME $(echo x) 'q' ` + "\n" + `end`
	printEnvVariable("TRICKY", tricky)

	cmd := exec.Command(sh, "-c", `eval "$1"; printf '%s' "$TRICKY"`, "sh", out.String())
	got, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, tricky, string(got))
}

// TestProcessOutput covers specs/cli-output "process output".
func TestProcessOutput(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	writeCachedCreds(t, mustResolve(t, "dev"), exp)
	logs := captureLogs(t)
	out := captureStdout(t)

	require.NoError(t, execute(t, "process", "--profile", "dev"))

	lines := nonEmptyLines(out.String())
	require.Len(t, lines, 1, "a single JSON line")
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &doc))
	assert.Equal(t, map[string]any{
		"Version":         float64(1),
		"AccessKeyId":     fakeAccessKey,
		"SecretAccessKey": fakeSecretKey,
		"SessionToken":    fakeSessionToken,
		"Expiration":      exp.Format(time.RFC3339),
	}, doc)
	assert.Empty(t, logs.String())
}

// TestRoleNotAssignedMessage covers specs/credential-retrieval "Role not
// assigned during console": one actionable message, no login.
func TestRoleNotAssignedMessage(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	runner := installFakeRunner(t, func(string, []string) fakeResult {
		return fakeResult{
			Stderr: "An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access",
			Exit:   255,
		}
	})
	logs := captureLogs(t)

	require.ErrorIs(t, execute(t, "console", "--profile", "dev"), errSSORoleNoAccess)
	lines := nonEmptyLines(logs.String())
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "not assigned to your user")
	assert.Empty(t, runner.CallsMatching("aws sso login"))
}

// TestConsoleCommandFlags runs console through the command tree to check
// flag wiring.
func TestConsoleCommandFlags(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))
	federationServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"SigninToken":"TOKEN"}`))
	})
	rec := installRecordBrowser(t)
	captureLogs(t)
	out := captureStdout(t)

	require.NoError(t, execute(t, "console", "--profile", "dev", "--logout-wait", "2"))

	require.Len(t, rec.events, 3)
	assert.Contains(t, rec.events[0], "logout!doLogout")
	assert.Equal(t, "sleep 2s", rec.events[1])
	assert.Contains(t, rec.events[2], "signin.aws.amazon.com/federation")
	assert.Empty(t, out.String(), "console writes nothing to stdout")
}
