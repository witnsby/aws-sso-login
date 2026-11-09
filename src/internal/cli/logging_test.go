package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs redirects everything the tool writes to stderr (logrus and
// the stderrOut seam) to one buffer and restores output, level and
// formatter afterwards.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	origLevel, origFormatter, origStderr := logrus.GetLevel(), logrus.StandardLogger().Formatter, stderrOut
	logrus.SetOutput(&buf)
	stderrOut = &buf
	t.Cleanup(func() {
		logrus.SetOutput(os.Stderr)
		logrus.SetLevel(origLevel)
		logrus.SetFormatter(origFormatter)
		stderrOut = origStderr
	})
	return &buf
}

// captureStdout redirects the stdout seam to a buffer.
func captureStdout(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := stdout
	t.Cleanup(func() { stdout = orig })
	stdout = &buf
	return &buf
}

// execute runs the real command tree like Run does, without exiting.
func execute(t *testing.T, args ...string) error {
	t.Helper()
	_, err := executeCode(t, args...)
	return err
}

// executeCode runs the real command tree like Run does and returns the exit
// status Run would use.
func executeCode(t *testing.T, args ...string) (int, error) {
	t.Helper()
	// Commands set the global log level and reporter; restore them so later
	// tests do not depend on which command ran before them.
	level := logrus.GetLevel()
	t.Cleanup(func() { logrus.SetLevel(level) })
	keepReporter(t)
	root := newRootCmd()
	root.SetArgs(args)
	return runCLI(root)
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

func TestConfigureLogging(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  string
		want logrus.Level
	}{
		{"process defaults to warn", []string{"process", "--profile", "dev"}, "", logrus.WarnLevel},
		{"export defaults to warn", []string{"export", "--profile", "dev"}, "", logrus.WarnLevel},
		{"import defaults to warn", []string{"import", "--profile", "dev"}, "", logrus.WarnLevel},
		{"console defaults to warn", []string{"console", "--profile", "dev"}, "", logrus.WarnLevel},
		{"--debug flag", []string{"process", "--profile", "dev", "--debug"}, "", logrus.DebugLevel},
		{"env 1", []string{"export", "--profile", "dev"}, "1", logrus.DebugLevel},
		{"env true", []string{"import", "--profile", "dev"}, "TRUE", logrus.DebugLevel},
		{"env other value ignored", []string{"process", "--profile", "dev"}, "yes", logrus.WarnLevel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupAWSEnv(t, legacyConfig)
			t.Setenv("AWS_SSO_LOGIN_DEBUG", tc.env)
			captureLogs(t)

			_, err := runResolved(t, tc.args...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, logrus.GetLevel())
		})
	}
}

// TestSingleErrorReport covers specs/cli-output "Missing profile reported
// once".
func TestSingleErrorReport(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	logs := captureLogs(t)
	out := captureStdout(t)

	code, err := executeCode(t, "process", "--profile", "missing")
	require.Error(t, err)

	lines := nonEmptyLines(logs.String())
	require.Len(t, lines, 1, "exactly one error line: %q", logs.String())
	assert.True(t, strings.HasPrefix(lines[0], "error: "), lines[0])
	assert.Contains(t, lines[0], `profile "missing" not found`)
	assert.Empty(t, out.String())
	assert.Equal(t, exitFailure, code)
}

// TestQuietProcessAndInformativeImport covers specs/cli-output "Default log
// levels".
func TestQuietProcessAndInformativeImport(t *testing.T) {
	env := setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))

	logs := captureLogs(t)
	out := captureStdout(t)
	require.NoError(t, execute(t, "process", "--profile", "dev"))
	assert.Empty(t, logs.String(), "process must not log on success")
	assert.True(t, strings.HasPrefix(out.String(), `{"Version":1`))

	logs.Reset()
	out.Reset()
	require.NoError(t, execute(t, "import", "--profile", "dev"))
	assert.Contains(t, logs.String(), "~/.aws/credentials")
	assert.NotContains(t, logs.String(), env.Home)
	assert.Empty(t, out.String(), "import writes nothing to stdout")
}

// TestNoSecretsInDebugLogs covers specs/cli-output "No secrets in debug
// logs": every credential-handling command runs with debug enabled through a
// refresh, and stderr never contains a credential value.
func TestNoSecretsInDebugLogs(t *testing.T) {
	for _, command := range []string{"export", "process", "import"} {
		t.Run(command, func(t *testing.T) {
			setupAWSEnv(t, legacyConfig)
			p := mustResolve(t, "dev")
			writeCachedCreds(t, p, time.Now().Add(time.Minute)) // forces a refresh
			installFakeRunner(t, func(string, []string) fakeResult {
				return fakeResult{
					Stdout: "arn:aws:sts::123456789012:assumed-role/Admin/user",
					Stderr: "debug noise from the AWS CLI",
					Before: func() { writeCachedCreds(t, p, time.Now().Add(time.Hour)) },
				}
			})
			logs := captureLogs(t)
			out := captureStdout(t)

			require.NoError(t, execute(t, command, "--profile", "dev", "--debug"))

			assert.NotEmpty(t, logs.String(), "debug run should log something")
			for _, secret := range []string{fakeAccessKey, fakeSecretKey, fakeSessionToken} {
				assert.NotContains(t, logs.String(), secret)
			}
			if command != "import" {
				assert.Contains(t, out.String(), fakeAccessKey)
			}
		})
	}
}

// TestDebugShowsAWSCLIStderr covers specs/cli-output "Debug flag": the raw
// AWS CLI stderr is logged at debug level when a refresh fails.
func TestDebugShowsAWSCLIStderr(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, func(string, []string) fakeResult {
		return fakeResult{Stderr: "Token has expired and refresh failed", Exit: 255}
	})
	logs := captureLogs(t)
	out := captureStdout(t)

	require.Error(t, execute(t, "process", "--profile", "dev", "--debug"))
	assert.Contains(t, logs.String(), "Token has expired and refresh failed")
	assert.Empty(t, out.String())
}

// TestVersionOutput covers specs/cli-output "version output".
func TestVersionOutput(t *testing.T) {
	out := captureStdout(t)
	require.NoError(t, execute(t, "version"))
	assert.Equal(t, "Version: dev, Commit: local\n", out.String())
	assert.Len(t, nonEmptyLines(out.String()), 1)
}
