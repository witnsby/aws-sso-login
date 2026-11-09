package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

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
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := configureLogging(cmd); err != nil {
				return err
			}
			report = newReporter(cmd.Name())
			return resolveProfileFlag(cmd, args)
		},
	}
	rootCmd.PersistentFlags().String("profile", "",
		"AWS profile name (defaults to $AWS_PROFILE; omit both to choose interactively)")
	rootCmd.PersistentFlags().Bool("debug", false,
		"Enable debug logs on stderr (also enabled by AWS_SSO_LOGIN_DEBUG=1)")
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return newUsageError(cmd, err)
	})
	rootCmd.AddCommand(newConsoleCmd(), newExportCmd(), newImportCmd(), newProcessCmd(), newVersionCmd())
	for _, c := range rootCmd.Commands() {
		c.Args = noArgs
	}
	return rootCmd
}

// configureLogging sets the log level for the command being run: debug when
// --debug or AWS_SSO_LOGIN_DEBUG is set, warn for the machine-readable
// commands (process, export), info otherwise. Logs go to stderr.
func configureLogging(cmd *cobra.Command) error {
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return fmt.Errorf("reading --debug: %w", err)
	}
	switch {
	case debug || debugFromEnv():
		logrus.SetLevel(logrus.DebugLevel)
	case cmd.Name() == "process" || cmd.Name() == "export":
		logrus.SetLevel(logrus.WarnLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
	return nil
}

// debugFromEnv reports whether AWS_SSO_LOGIN_DEBUG is "1" or "true".
func debugFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AWS_SSO_LOGIN_DEBUG")))
	return v == "1" || v == "true"
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
			fmt.Fprintln(stdout, getVersion())
		},
	}
}

// Run builds the root command, executes the CLI application and exits with
// the status of the failure, if any.
func Run() {
	if code, _ := runCLI(newRootCmd()); code != exitOK {
		exit(code)
	}
}

// runCLI executes root and reports a failure once. It returns the exit
// status and the error, if any.
func runCLI(root *cobra.Command) (int, error) {
	cmd, err := root.ExecuteC()
	if err == nil {
		return exitOK, nil
	}
	// The root command has no action of its own, so an error attributed to
	// it comes from parsing, such as an unknown subcommand.
	var ue *userError
	if cmd == root && !errors.As(err, &ue) {
		err = newUsageError(root, err)
	}
	return reportError(cmd, err), err
}

// noArgs rejects positional arguments with a usage error.
func noArgs(cmd *cobra.Command, args []string) error {
	if err := cobra.NoArgs(cmd, args); err != nil {
		return newUsageError(cmd, err)
	}
	return nil
}
