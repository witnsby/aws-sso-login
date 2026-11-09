package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runResolved executes args on a fresh root command whose subcommand RunE is
// replaced by a probe, and returns the profile the command would run with.
func runResolved(t *testing.T, args ...string) (string, error) {
	t.Helper()
	keepReporter(t)
	root := newRootCmd()
	var got string
	for _, c := range root.Commands() {
		c.RunE = func(cmd *cobra.Command, _ []string) error {
			v, err := cmd.Flags().GetString("profile")
			got = v
			return err
		}
	}
	root.SetArgs(args)
	err := root.Execute()
	return got, err
}

// TestProfileFlagPrecedence covers specs/profile-resolution "Profile name
// source precedence" and "Interactive picker availability" for every command.
func TestProfileFlagPrecedence(t *testing.T) {
	commands := []struct {
		name        string
		allowPicker bool
	}{
		{"console", true},
		{"export", true},
		{"import", true},
		{"process", false},
	}

	for _, c := range commands {
		t.Run(c.name, func(t *testing.T) {
			t.Run("flag wins over AWS_PROFILE", func(t *testing.T) {
				setupAWSEnv(t, legacyConfig)
				t.Setenv("AWS_PROFILE", "prod")
				rec := &recordingSelector{inner: fakeSelector{choice: "unused"}}
				swapDefaultSelector(t, rec)
				setInteractive(t, true)

				got, err := runResolved(t, c.name, "--profile", "dev")
				require.NoError(t, err)
				assert.Equal(t, "dev", got)
				assert.Zero(t, rec.calls)
			})

			t.Run("AWS_PROFILE used when flag omitted", func(t *testing.T) {
				setupAWSEnv(t, legacyConfig)
				t.Setenv("AWS_PROFILE", "dev")
				rec := &recordingSelector{inner: fakeSelector{choice: "unused"}}
				swapDefaultSelector(t, rec)
				setInteractive(t, true)

				got, err := runResolved(t, c.name)
				require.NoError(t, err)
				assert.Equal(t, "dev", got)
				assert.Zero(t, rec.calls)
			})

			t.Run("terminal without profile", func(t *testing.T) {
				setupAWSEnv(t, legacyConfig)
				rec := &recordingSelector{inner: fakeSelector{choice: "default"}}
				swapDefaultSelector(t, rec)
				setInteractive(t, true)

				got, err := runResolved(t, c.name)
				if c.allowPicker {
					require.NoError(t, err)
					assert.Equal(t, "default", got)
					assert.Equal(t, 1, rec.calls)
					return
				}
				require.Error(t, err)
				assertNoProfileError(t, err)
				assert.Zero(t, rec.calls, "process never prompts")
			})

			t.Run("non-interactive without profile", func(t *testing.T) {
				setupAWSEnv(t, legacyConfig)
				rec := &recordingSelector{inner: fakeSelector{choice: "unused"}}
				swapDefaultSelector(t, rec)
				setInteractive(t, false)

				_, err := runResolved(t, c.name)
				require.Error(t, err)
				assertNoProfileError(t, err)
				assert.Zero(t, rec.calls)
			})

			if c.allowPicker {
				t.Run("picker cancelled", func(t *testing.T) {
					setupAWSEnv(t, legacyConfig)
					swapDefaultSelector(t, fakeSelector{err: errSelectionCancelled})
					setInteractive(t, true)

					_, err := runResolved(t, c.name)
					require.Error(t, err)
					assert.Contains(t, err.Error(), "Profile selection cancelled")
					assert.Equal(t, exitCancelled, asUserError(err).code)
				})
			}
		})
	}
}

func TestVersionNeedsNoProfile(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	setInteractive(t, false)
	_, err := runResolved(t, "version")
	require.NoError(t, err)
}

// assertNoProfileError checks the usage error for a missing profile.
func assertNoProfileError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, exitUsage, asUserError(err).code)
	assert.Contains(t, rendered(err), "--profile <name>")
	assert.Contains(t, rendered(err), "AWS_PROFILE")
}
