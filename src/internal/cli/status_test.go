package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setStderrTerminal makes the stderr terminal check return tty.
func setStderrTerminal(t *testing.T, tty bool) {
	t.Helper()
	orig := stderrIsTerminal
	t.Cleanup(func() { stderrIsTerminal = orig })
	stderrIsTerminal = func() bool { return tty }
}

// keepReporter restores the package reporter after the test.
func keepReporter(t *testing.T) {
	t.Helper()
	orig := report
	t.Cleanup(func() { report = orig })
}

// TestStatusLinePrefixes covers specs/cli-output "Status line kinds".
func TestStatusLinePrefixes(t *testing.T) {
	cases := []struct {
		name    string
		tty     bool
		noColor string
		term    string
		want    string
	}{
		{"terminal with color", true, "", "xterm-256color",
			"\x1b[2m→\x1b[0m step\n\x1b[32m✓\x1b[0m done\n\x1b[33m!\x1b[0m careful\n"},
		{"terminal with NO_COLOR", true, "1", "xterm-256color",
			"→ step\n✓ done\n! careful\n"},
		{"terminal with TERM=dumb", true, "", "dumb",
			"→ step\n✓ done\n! careful\n"},
		{"not a terminal", false, "", "xterm-256color",
			"-> step\nok: done\nwarning: careful\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setStderrTerminal(t, tc.tty)
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("TERM", tc.term)
			buf := captureStderrOut(t)

			r := newReporter("import")
			r.Step("step")
			r.Success("done")
			r.Warn("careful")

			assert.Equal(t, tc.want, buf.String())
			if !tc.tty || tc.noColor != "" || tc.term == "dumb" {
				assert.NotContains(t, buf.String(), "\x1b[")
			}
		})
	}
}

func TestErrorPrefix(t *testing.T) {
	setStderrTerminal(t, false)
	assert.Equal(t, "error: boom", newReporter("import").line(kindError, "boom"))

	setStderrTerminal(t, true)
	t.Setenv("NO_COLOR", "1")
	assert.Equal(t, "✗ boom", newReporter("import").line(kindError, "boom"))
}

func TestQuietReporterKeepsWarnings(t *testing.T) {
	setStderrTerminal(t, false)
	buf := captureStderrOut(t)

	r := &reporter{quiet: true}
	r.Step("step")
	r.Success("done")
	r.Warn("careful")

	assert.Equal(t, "warning: careful\n", buf.String())
}

// TestReporterPerCommand checks that process and export get a quiet
// reporter while import and console do not.
func TestReporterPerCommand(t *testing.T) {
	for cmd, quiet := range map[string]bool{"process": true, "export": true, "import": false, "console": false} {
		t.Run(cmd, func(t *testing.T) {
			setupAWSEnv(t, legacyConfig)
			keepReporter(t)
			captureLogs(t)

			_, err := runResolved(t, cmd, "--profile", "dev")
			require.NoError(t, err)
			assert.Equal(t, quiet, report.quiet)
		})
	}
}
