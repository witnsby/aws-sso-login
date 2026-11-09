package cli

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logMarker matches logrus level markers such as INFO[0014].
var logMarker = regexp.MustCompile(`(?m)^(DEBU|INFO|WARN|ERRO|FATA)\[`)

const callerARN = "arn:aws:sts::123456789012:assumed-role/Admin/user"

// refreshingRunner answers every AWS CLI call with a successful refresh that
// writes fresh credentials for profile p.
func refreshingRunner(t *testing.T, name string) *fakeRunner {
	t.Helper()
	p := mustResolve(t, name)
	writeCachedCreds(t, p, time.Now().Add(time.Minute)) // forces a refresh
	return installFakeRunner(t, func(string, []string) fakeResult {
		return fakeResult{Stdout: callerARN, Before: func() { writeCachedCreds(t, p, time.Now().Add(time.Hour)) }}
	})
}

// TestDebugOnlyLogging covers specs/cli-output "No log prefixes in normal
// output" and "Debug timestamps".
func TestDebugOnlyLogging(t *testing.T) {
	t.Run("no log markers without debug", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		refreshingRunner(t, "dev")
		logs := captureLogs(t)

		require.NoError(t, execute(t, "import", "--profile", "dev"))
		assert.False(t, logMarker.MatchString(logs.String()), logs.String())
		assert.NotContains(t, logs.String(), "level=")
		assert.NotContains(t, logs.String(), "time=")
	})

	t.Run("no log markers in an error", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		installFakeRunner(t, nil)
		logs := captureLogs(t)
		captureStdout(t)

		require.Error(t, execute(t, "export", "--profile", "missing"))
		assert.True(t, strings.HasPrefix(logs.String(), "error: "), logs.String())
		assert.False(t, logMarker.MatchString(logs.String()))
	})

	t.Run("debug lines carry a full timestamp", func(t *testing.T) {
		setStderrTerminal(t, false)
		setupAWSEnv(t, legacyConfig)
		refreshingRunner(t, "dev")
		logs := captureLogs(t)

		require.NoError(t, execute(t, "import", "--profile", "dev", "--debug"))
		debugLines := 0
		for _, l := range nonEmptyLines(logs.String()) {
			if strings.Contains(l, "level=debug") {
				debugLines++
				assert.Regexp(t, `time="\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`, l)
			}
		}
		assert.Positive(t, debugLines)
	})
}

// TestRefreshNotice covers specs/cli-output "Refresh notice without the
// caller ARN".
func TestRefreshNotice(t *testing.T) {
	setStderrTerminal(t, false)
	setupAWSEnv(t, legacyConfig)
	refreshingRunner(t, "dev")
	logs := captureLogs(t)

	require.NoError(t, execute(t, "import", "--profile", "dev"))
	assert.Contains(t, logs.String(), "-> Refreshed credentials for [dev]\n")
	assert.NotContains(t, logs.String(), callerARN)
	assert.NotContains(t, logs.String(), "123456789012")

	logs.Reset()
	refreshingRunner(t, "dev")
	require.NoError(t, execute(t, "import", "--profile", "dev", "--debug"))
	assert.Contains(t, logs.String(), callerARN)
}

// TestLoginIsFramedByStatusLines covers specs/credential-retrieval
// "Expired SSO token during import".
func TestLoginIsFramedByStatusLines(t *testing.T) {
	setStderrTerminal(t, false)
	setupAWSEnv(t, legacyConfig)
	p := mustResolve(t, "dev")
	logs := captureLogs(t)
	var atLogin string
	sts := 0
	runner := installFakeRunner(t, func(name string, args []string) fakeResult {
		if args[0] == "sso" {
			atLogin = logs.String()
			return fakeResult{Stdout: "Open https://device.sso.example/ and enter CODE-1234"}
		}
		sts++
		if sts == 1 {
			return fakeResult{Stderr: expiredTokenStderr, Exit: 255}
		}
		return fakeResult{Stdout: callerARN, Before: func() { writeCachedCreds(t, p, time.Now().Add(time.Hour)) }}
	})

	require.NoError(t, execute(t, "import", "--profile", "dev"))

	require.Len(t, runner.CallsMatching("aws sso login --profile dev"), 1)
	assert.Equal(t, "-> Credentials for [dev] need a new AWS SSO sign-in, running 'aws sso login'\n", atLogin,
		"the step line is written before aws sso login starts")
	after := strings.TrimPrefix(logs.String(), atLogin)
	lines := nonEmptyLines(after)
	require.NotEmpty(t, lines)
	assert.Equal(t, "ok: Signed in to AWS SSO", lines[0])
	assert.True(t, strings.HasPrefix(lines[len(lines)-1], "ok: Credentials for [dev] written to"))
}

// TestImportSuccessLine covers specs/credentials-file "Import result
// reporting".
func TestImportSuccessLine(t *testing.T) {
	zone := time.FixedZone("UTC+2", 2*60*60)
	cases := []struct {
		name    string
		now     time.Time
		expires time.Time
		want    []string
	}{
		{"same day", time.Date(2026, 11, 9, 10, 42, 0, 0, zone), time.Date(2026, 11, 9, 18, 42, 0, 0, zone),
			[]string{"valid until 18:42, 8h0m left"}},
		{"next day", time.Date(2026, 11, 9, 23, 30, 0, 0, zone), time.Date(2026, 11, 10, 7, 30, 0, 0, zone),
			[]string{"valid until 2026-11-10 07:30, 8h0m left"}},
		{"under an hour, rounded down", time.Date(2026, 11, 9, 10, 0, 0, 0, zone), time.Date(2026, 11, 9, 10, 45, 59, 0, zone),
			[]string{"valid until 10:45, 45m left"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setStderrTerminal(t, false)
			setupAWSEnv(t, legacyConfig)
			installFakeRunner(t, nil)
			fixClock(t, tc.now)
			writeCachedCreds(t, mustResolve(t, "dev"), tc.expires)
			logs := captureLogs(t)
			out := captureStdout(t)

			require.NoError(t, execute(t, "import", "--profile", "dev"))

			lines := nonEmptyLines(logs.String())
			require.NotEmpty(t, lines)
			last := lines[len(lines)-1]
			assert.True(t, strings.HasPrefix(last, "ok: Credentials for [dev] written to ~/.aws/credentials ("), last)
			for _, w := range tc.want {
				assert.Contains(t, last, w)
			}
			for _, secret := range []string{fakeAccessKey, fakeSecretKey, fakeSessionToken} {
				assert.NotContains(t, logs.String(), secret)
				assert.NotContains(t, out.String(), secret)
			}
			assert.Empty(t, out.String())
		})
	}
}

func TestDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/home/user")
	assert.Equal(t, "~/.aws/credentials", displayPath("/home/user/.aws/credentials"))
	assert.Equal(t, "/tmp/creds", displayPath("/tmp/creds"))
	assert.Equal(t, "/home/username/creds", displayPath("/home/username/creds"))
}

// TestConsoleSuccessLine covers specs/cli-output "Console success" and
// "Console success via the picker".
func TestConsoleSuccessLine(t *testing.T) {
	for _, viaPicker := range []bool{false, true} {
		name := "flag"
		if viaPicker {
			name = "picker"
		}
		t.Run(name, func(t *testing.T) {
			setStderrTerminal(t, false)
			setupAWSEnv(t, legacyConfig)
			installFakeRunner(t, nil)
			writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))
			federationServer(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"SigninToken":"TOKEN"}`))
			})
			installRecordBrowser(t)
			logs := captureLogs(t)
			out := captureStdout(t)

			args := []string{"console", "--profile", "dev"}
			if viaPicker {
				swapDefaultSelector(t, fakeSelector{choice: "dev"})
				setInteractive(t, true)
				args = []string{"console"}
			}
			require.NoError(t, execute(t, args...))

			lines := nonEmptyLines(logs.String())
			require.NotEmpty(t, lines)
			assert.Equal(t, "ok: Opened the AWS console for [dev] in eu-west-1", lines[len(lines)-1])
			assert.Empty(t, out.String())
		})
	}
}

// TestMachineReadableCommandsQuietOnSuccess covers specs/cli-output
// "Quiet process" and "Export success stays quiet", including a refresh.
func TestMachineReadableCommandsQuietOnSuccess(t *testing.T) {
	for _, command := range []string{"export", "process"} {
		t.Run(command, func(t *testing.T) {
			setStderrTerminal(t, true)
			setupAWSEnv(t, legacyConfig)
			refreshingRunner(t, "dev")
			logs := captureLogs(t)
			out := captureStdout(t)

			require.NoError(t, execute(t, command, "--profile", "dev"))
			assert.Empty(t, logs.String())
			assert.NotEmpty(t, out.String())
		})
	}
}
