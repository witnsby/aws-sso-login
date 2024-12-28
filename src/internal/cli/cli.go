package cli

import (
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

func init() {
	consoleCmd.Flags().String("profile", "", "Name of the AWS profile")
	consoleCmd.Flags().Bool("force-logout", true, "Force logout of any existing session in the browser first")
	consoleCmd.Flags().Int("logout-wait", 1, "Number of seconds to wait after forcing logout before logging in")

	exportCmd.Flags().String("profile", "", "Name of the AWS profile")

	importCmd.Flags().String("profile", "", "Name of the AWS profile")

	processCmd.Flags().String("profile", "", "Name of the AWS profile")
}

// ----------------------------------------------------------------------------
// Subcommands
// ----------------------------------------------------------------------------

var consoleCmd = &cobra.Command{
	Use:   "console --profile [profile-name]",
	Short: "Opens the default browser and logs into AWS Web Console using SSO",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		forceLogout, _ := cmd.Flags().GetBool("force-logout")
		logoutWait, _ := cmd.Flags().GetInt("logout-wait")

		if profileName == "" {
			return errors.New(helper.ErrorPofileSpecification)
		}
		return console(profileName, forceLogout, logoutWait)
	},
}

var exportCmd = &cobra.Command{
	Use:   "export --profile [profile-name]",
	Short: "Prints credentials for exporting to your shell",
	RunE: func(cmd *cobra.Command, args []string) error {

		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			return errors.New(helper.ErrorPofileSpecification)
		}

		return exportCredsToOutput(profileName)

	},
}

var importCmd = &cobra.Command{
	Use:   "import --profile [profile-name]",
	Short: "Fetches new credentials and writes them to the local credentials file",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			return errors.New(helper.ErrorPofileSpecification)
		}
		return importCreds(profileName)
	},
}

var processCmd = &cobra.Command{
	Use:   "process.go --profile [profile-name]",
	Short: "Fetches credential_process-compatible JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Flags().GetString("profile")
		if profileName == "" {
			return errors.New(helper.ErrorPofileSpecification)
		}
		return processCreds(profileName)
	},
}

func Run() {
	rootCmd := &cobra.Command{
		Use:   "aws-sso-login",
		Short: "AWS SSO utility",
	}

	rootCmd.AddCommand(consoleCmd, exportCmd, importCmd, processCmd)

	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
