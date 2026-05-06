package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// init initializes flags and options for various commands: consoleCmd, exportCmd, importCmd, and processCmd.
func init() {
	consoleCmd.Flags().String("profile", "", "Name of the AWS profile")
	consoleCmd.Flags().Bool("force-logout", true, "Force logout of any existing session in the browser first")
	consoleCmd.Flags().Int("logout-wait", 1, "Number of seconds to wait after forcing logout before logging in")

	exportCmd.Flags().String("profile", "", "Name of the AWS profile")

	importCmd.Flags().String("profile", "", "AWS profile name (omit to choose interactively)")

	processCmd.Flags().String("profile", "", "Name of the AWS profile")
}

// consoleCmd represents a Cobra command to log into AWS Web Console using SSO, opening it in the default browser.
var consoleCmd = &cobra.Command{
	Use:   "console --profile [profile-name]",
	Short: "Opens the default browser and logs into AWS Web Console using SSO",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		forceLogout, _ := cmd.Flags().GetBool("force-logout")
		logoutWait, _ := cmd.Flags().GetInt("logout-wait")

		if profileName == "" {
			logrus.Error(helper.ErrorPofileSpecification)
			return errors.New(helper.ErrorPofileSpecification)
		}
		return console(profileName, forceLogout, logoutWait)
	},
}

// exportCmd defines a Cobra command to export AWS credentials for a specified profile in a shell-exportable format.
var exportCmd = &cobra.Command{
	Use:   "export --profile [profile-name]",
	Short: "Prints credentials for exporting to your shell",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			logrus.Error(helper.ErrorPofileSpecification)
			return errors.New(helper.ErrorPofileSpecification)
		}
		return exportCredsToOutput(profileName)
	},
}

// importCmd defines a Cobra command to fetch AWS credentials for a profile and
// write them to the local credentials file. The --profile flag is optional:
// when omitted, the user is prompted to pick an SSO-enabled profile from
// ~/.aws/config via the package-level defaultSelector.
var importCmd = &cobra.Command{
	Use:   "import [--profile profile-name]",
	Short: "Fetches new credentials and writes them to the local credentials file",
	RunE: func(cmd *cobra.Command, args []string) error {
		flagValue, _ := cmd.Flags().GetString("profile")
		profileName, err := resolveProfileName(flagValue)
		if err != nil {
			return err
		}
		return importCreds(profileName)
	},
}

// processCmd defines a Cobra command used to fetch and output credential process compatible JSON for a specified profile.
var processCmd = &cobra.Command{
	Use:   "process --profile [profile-name]",
	Short: "Fetches credential process compatible JSON output",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			logrus.Error(helper.ErrorPofileSpecification)
			return errors.New(helper.ErrorPofileSpecification)
		}
		return processCreds(profileName)
	},
}

// processCmd defines a Cobra command used to fetch and output credential process compatible JSON for a specified profile.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getVersion())
	},
}

// Run initializes the root command for the AWS SSO utility and adds subcommands before executing the CLI application.
func Run() {
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
	}
	rootCmd.AddCommand(consoleCmd, exportCmd, importCmd, processCmd, versionCmd)
	if err := rootCmd.Execute(); err != nil {
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
