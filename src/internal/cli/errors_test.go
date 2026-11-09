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
