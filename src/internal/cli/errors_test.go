package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rendered returns err as the user sees it on one line.
func rendered(err error) string { return asUserError(err).oneLine() }

func TestUserErrorUnwraps(t *testing.T) {
	cause := fmt.Errorf("%w for profile %q", errSSORoleNoAccess, "dev")
	err := fmt.Errorf("outer: %w", &userError{title: "No access", code: exitNoAccess, err: cause})

	assert.ErrorIs(t, err, errSSORoleNoAccess)
	assert.Equal(t, exitNoAccess, asUserError(err).code)
	assert.Equal(t, "No access", asUserError(err).title)
}

// TestReportErrorRendering covers specs/cli-output "Error block layout" and
// "One-line errors for process".
func TestReportErrorRendering(t *testing.T) {
	detailed := &userError{
		title:   "Something failed for profile [dev]",
		details: []string{"First detail.", "Second detail."},
		hint:    "Do the next thing.",
		code:    exitNotSignedIn,
		err:     errors.New("raw cause"),
	}
	cases := []struct {
		name     string
		cmd      string
		err      error
		wantOut  string
		wantCode int
	}{
		{"block with details and hint", "import", detailed,
			"error: Something failed for profile [dev]\n  First detail.\n  Second detail.\n  Fix: Do the next thing.\n", exitNotSignedIn},
		{"process folds the block into one line", "process", detailed,
			"error: Something failed for profile [dev]. First detail. Second detail. Fix: Do the next thing.\n", exitNotSignedIn},
		{"plain error has no Fix line", "import", errors.New("reading credentials file: permission denied"),
			"error: reading credentials file: permission denied\n", exitFailure},
		{"plain error on process", "process", errors.New("boom"),
			"error: boom\n", exitFailure},
		{"role not assigned without profile context", "export", fmt.Errorf("x: %w", errSSORoleNoAccess),
			"error: No access to the configured SSO role\n  You are signed in to AWS SSO, but this role is not assigned to your user.\n" +
				"  Fix: Ask your AWS administrator to assign the role, or choose another profile.\n", exitNoAccess},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setStderrTerminal(t, false)
			logs := captureLogs(t)

			code := reportError(&cobra.Command{Use: tc.cmd}, tc.err)

			assert.Equal(t, tc.wantOut, logs.String())
			assert.Equal(t, tc.wantCode, code)
			assert.NotContains(t, logs.String(), "raw cause", "raw cause only in debug output")
		})
	}
}

// TestReportErrorDebugChain covers specs/cli-output "Error chain in debug
// output".
func TestReportErrorDebugChain(t *testing.T) {
	setStderrTerminal(t, false)
	logs := captureLogs(t)
	logrus.SetLevel(logrus.DebugLevel)

	err := &userError{title: "Title", code: exitFailure, err: errors.New("raw cause")}
	reportError(&cobra.Command{Use: "import"}, err)

	lines := nonEmptyLines(logs.String())
	require.Len(t, lines, 2)
	assert.Equal(t, "error: Title", lines[0])
	assert.Contains(t, lines[1], "level=debug")
	assert.Contains(t, lines[1], "raw cause")
}

// recordExit replaces the exit seam and records the status it was called
// with (-1 when it was not called).
func recordExit(t *testing.T) *int {
	t.Helper()
	got := -1
	orig := exit
	t.Cleanup(func() { exit = orig })
	exit = func(code int) { got = code }
	return &got
}

func TestRunExitStatus(t *testing.T) {
	setStderrTerminal(t, false)
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	keepReporter(t)
	captureLogs(t)
	captureStdout(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })

	os.Args = []string{"aws-sso-login", "process", "--profile", "missing"}
	got := recordExit(t)
	Run()
	assert.Equal(t, exitFailure, *got)

	os.Args = []string{"aws-sso-login", "version"}
	got = recordExit(t)
	Run()
	assert.Equal(t, -1, *got, "success must not call exit")
}

// TestUsageErrors covers specs/cli-output "Exit codes" for usage errors.
func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{"import", "--no-such-flag"},
		{"import", "extra-arg"},
		{"no-such-command"},
		{"--no-such-flag"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			setStderrTerminal(t, false)
			setupAWSEnv(t, legacyConfig)
			runner := installFakeRunner(t, nil)
			logs := captureLogs(t)
			out := captureStdout(t)

			code, err := executeCode(t, args...)

			require.Error(t, err)
			assert.Equal(t, exitUsage, code)
			lines := nonEmptyLines(logs.String())
			require.Len(t, lines, 2, "one error block: %q", logs.String())
			assert.True(t, strings.HasPrefix(lines[0], "error: "), lines[0])
			assert.True(t, strings.HasPrefix(lines[1], "  Fix: Run 'aws-sso-login"), lines[1])
			assert.Empty(t, out.String())
			assert.Empty(t, runner.Calls())
		})
	}
}

// ssoSessionNoAccessConfig has a legacy and an sso-session profile whose
// role is ReadOnly in account 123456789012.
const ssoSessionNoAccessConfig = `[profile legacy]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = ReadOnly
region = eu-west-1

[profile session]
sso_session = corp
sso_account_id = 123456789012
sso_role_name = ReadOnly
region = eu-west-1

[sso-session corp]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`

// TestRoleNotAssignedNamesAccountAndRole covers specs/credential-retrieval
// "Role not assigned" for legacy and sso-session profiles on every command.
func TestRoleNotAssignedNamesAccountAndRole(t *testing.T) {
	for _, profile := range []string{"legacy", "session"} {
		for _, command := range []string{"console", "import", "export", "process"} {
			t.Run(profile+"/"+command, func(t *testing.T) {
				setStderrTerminal(t, false)
				setupAWSEnv(t, ssoSessionNoAccessConfig)
				runner := installFakeRunner(t, func(string, []string) fakeResult {
					return fakeResult{
						Stderr: "An error occurred (AccessDeniedException) when calling the GetRoleCredentials operation: No access",
						Exit:   255,
					}
				})
				logs := captureLogs(t)
				out := captureStdout(t)

				code, err := executeCode(t, command, "--profile", profile)

				require.ErrorIs(t, err, errSSORoleNoAccess)
				assert.Equal(t, exitNoAccess, code)
				assert.Contains(t, logs.String(), "["+profile+"]")
				assert.Contains(t, logs.String(), "123456789012")
				assert.Contains(t, logs.String(), "ReadOnly")
				assert.Contains(t, logs.String(), "Fix: Ask your AWS administrator")
				assert.Empty(t, out.String())
				assert.Empty(t, runner.CallsMatching("aws sso login"))
				if command == "process" {
					assert.Len(t, nonEmptyLines(logs.String()), 1)
				}
			})
		}
	}
}

// TestFailureExitCodes covers specs/cli-output "Exit codes" through the
// command tree.
func TestFailureExitCodes(t *testing.T) {
	t.Run("expired SSO session for export", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		installFakeRunner(t, func(string, []string) fakeResult {
			return fakeResult{Stderr: "Token has expired and refresh failed", Exit: 255}
		})
		logs := captureLogs(t)
		captureStdout(t)

		code, _ := executeCode(t, "export", "--profile", "dev")
		assert.Equal(t, exitNotSignedIn, code)
		assert.Contains(t, logs.String(), "Fix: Run 'aws sso login --profile dev'")
	})

	t.Run("AWS CLI missing for import", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		runner := installFakeRunner(t, nil)
		missingExecutables(t, "aws")
		captureLogs(t)

		code, _ := executeCode(t, "import", "--profile", "dev")
		assert.Equal(t, exitNoAWSCLI, code)
		assert.Empty(t, runner.Calls())
	})

	t.Run("failed login for import", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		runner := loginScript(t, []fakeResult{{Stderr: expiredTokenStderr, Exit: 255}}, 1)
		captureLogs(t)

		code, _ := executeCode(t, "import", "--profile", "dev")
		assert.Equal(t, exitNotSignedIn, code)
		assert.Len(t, runner.CallsMatching("aws sso login"), 1)
	})

	t.Run("process without a profile", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		setInteractive(t, true)
		logs := captureLogs(t)

		code, _ := executeCode(t, "process")
		assert.Equal(t, exitUsage, code)
		assert.Len(t, nonEmptyLines(logs.String()), 1)
	})
}
