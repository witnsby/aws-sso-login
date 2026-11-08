package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// profileAnnotation marks commands that need a profile name. Its value says
// whether the interactive picker may be used to choose one.
const (
	profileAnnotation  = "aws-sso-login/profile"
	profileInteractive = "interactive"
	profileRequired    = "required"
)

// newRootCmd builds the command tree. A fresh tree per call keeps flag state
// from leaking between tests.
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "aws-sso-login",
		Short: "AWS SSO utility",
		// SilenceUsage avoids dumping the Usage block on runtime errors
		// (it is only useful for flag/argument parsing failures, which
		// cobra still prints separately). SilenceErrors prevents the
		// duplicate "Error: ..." line — Run() owns user-facing error
		// rendering below.
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: resolveProfileFlag,
	}
	rootCmd.PersistentFlags().String("profile", "",
		"AWS profile name (defaults to $AWS_PROFILE; omit both to choose interactively)")
	rootCmd.AddCommand(newConsoleCmd(), newExportCmd(), newImportCmd(), newProcessCmd(), newVersionCmd())
	return rootCmd
}

// resolveProfileFlag resolves the profile name once for every command that
// needs one (flag, then $AWS_PROFILE, then the picker where allowed) and
// stores the result back in the --profile flag.
func resolveProfileFlag(cmd *cobra.Command, _ []string) error {
	mode, ok := cmd.Annotations[profileAnnotation]
	if !ok {
		return nil
	}
	flagValue, err := cmd.Flags().GetString("profile")
	if err != nil {
		return fmt.Errorf("reading --profile: %w", err)
	}
	name, err := resolveProfileName(flagValue, mode == profileInteractive)
	if err != nil {
		return err
	}
	return cmd.Flags().Set("profile", name)
}

// newConsoleCmd builds the command that logs into the AWS Web Console using
// SSO and opens it in the default browser.
func newConsoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "console [--profile profile-name]",
		Short:       "Opens the default browser and logs into AWS Web Console using SSO",
		Annotations: map[string]string{profileAnnotation: profileInteractive},
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := cmd.Flags().GetString("profile")
			if err != nil {
				return fmt.Errorf("reading --profile: %w", err)
			}
			forceLogout, err := cmd.Flags().GetBool("force-logout")
			if err != nil {
				return fmt.Errorf("reading --force-logout: %w", err)
			}
			logoutWait, err := cmd.Flags().GetInt("logout-wait")
			if err != nil {
				return fmt.Errorf("reading --logout-wait: %w", err)
			}
			return console(profileName, forceLogout, logoutWait)
		},
	}
	cmd.Flags().Bool("force-logout", true, "Force logout of any existing session in the browser first")
	cmd.Flags().Int("logout-wait", 1, "Number of seconds to wait after forcing logout before logging in")
	return cmd
}

// newExportCmd builds the command that prints credentials for a profile in a
// shell-exportable format.
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "export [--profile profile-name]",
		Short:       "Prints credentials for exporting to your shell",
		Annotations: map[string]string{profileAnnotation: profileInteractive},
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := cmd.Flags().GetString("profile")
			if err != nil {
				return fmt.Errorf("reading --profile: %w", err)
			}
			return exportCredsToOutput(profileName)
		},
	}
}

// newImportCmd builds the command that fetches credentials for a profile and
// writes them to the shared credentials file.
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "import [--profile profile-name]",
		Short:       "Fetches new credentials and writes them to the local credentials file",
		Annotations: map[string]string{profileAnnotation: profileInteractive},
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := cmd.Flags().GetString("profile")
			if err != nil {
				return fmt.Errorf("reading --profile: %w", err)
			}
			return importCreds(profileName)
		},
	}
}

// newProcessCmd builds the command that prints credential_process compatible
// JSON for a profile. It never prompts.
func newProcessCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "process [--profile profile-name]",
		Short:       "Fetches credential process compatible JSON output",
		Annotations: map[string]string{profileAnnotation: profileRequired},
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName, err := cmd.Flags().GetString("profile")
			if err != nil {
				return fmt.Errorf("reading --profile: %w", err)
			}
			return processCreds(profileName)
		},
	}
}

// newVersionCmd builds the command that prints version information.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(getVersion())
		},
	}
}

// Run builds the root command and executes the CLI application.
func Run() {
	if err := newRootCmd().Execute(); err != nil {
		reportAndExit(err)
	}
}

// reportAndExit prints a single concise message for known failure modes and
// exits with status 1. Detailed/verbose context is left to debug-level logs.
func reportAndExit(err error) {
	if errors.Is(err, errSSORoleNoAccess) {
		logrus.Errorf("No access: the configured SSO role is not assigned to your user. "+
			"Ask your AWS administrator to grant access. (%v)", err)
		os.Exit(1)
	}
	logrus.Error(err)
	os.Exit(1)
}
