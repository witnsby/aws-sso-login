package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStderrOut redirects stderrOut to a buffer for the test.
func captureStderrOut(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := stderrOut
	t.Cleanup(func() { stderrOut = orig })
	stderrOut = &buf
	return &buf
}

func setGOOS(t *testing.T, value string) {
	t.Helper()
	orig := goos
	t.Cleanup(func() { goos = orig })
	goos = value
}

func TestOpenBrowser(t *testing.T) {
	const url = "https://example.com/federation?SigninToken=x"

	t.Run("linux uses xdg-open", func(t *testing.T) {
		setGOOS(t, "linux")
		runner := installFakeRunner(t, nil)
		out := captureStderrOut(t)

		openBrowser(url)

		calls := runner.Calls()
		require.Len(t, calls, 1)
		assert.Equal(t, "xdg-open "+url, calls[0].Argv())
		assert.Empty(t, out.String())
	})

	t.Run("linux without xdg-open prints the URL and never tries open", func(t *testing.T) {
		setGOOS(t, "linux")
		runner := installFakeRunner(t, nil)
		missingExecutables(t, "xdg-open")
		out := captureStderrOut(t)

		openBrowser(url)

		assert.Empty(t, runner.Calls(), "no launcher may run")
		assert.Contains(t, out.String(), "Please open this URL in your browser: "+url)
	})

	t.Run("launcher fails to start prints the URL", func(t *testing.T) {
		setGOOS(t, "linux")
		installFakeRunner(t, func(string, []string) fakeResult { return fakeResult{NotFound: true} })
		out := captureStderrOut(t)

		openBrowser(url)

		assert.Contains(t, out.String(), url)
	})

	t.Run("darwin uses open", func(t *testing.T) {
		setGOOS(t, "darwin")
		runner := installFakeRunner(t, nil)
		captureStderrOut(t)

		openBrowser(url)

		require.Len(t, runner.Calls(), 1)
		assert.Equal(t, "open "+url, runner.Calls()[0].Argv())
	})

	t.Run("windows uses rundll32", func(t *testing.T) {
		setGOOS(t, "windows")
		runner := installFakeRunner(t, nil)
		captureStderrOut(t)

		openBrowser(url)

		require.Len(t, runner.Calls(), 1)
		assert.Equal(t, "rundll32 url.dll,FileProtocolHandler "+url, runner.Calls()[0].Argv())
	})

	t.Run("unknown OS prints the URL", func(t *testing.T) {
		setGOOS(t, "plan9")
		runner := installFakeRunner(t, nil)
		out := captureStderrOut(t)

		openBrowser(url)

		assert.Empty(t, runner.Calls())
		assert.Contains(t, out.String(), url)
	})
}
