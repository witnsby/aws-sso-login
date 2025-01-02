package cli

import (
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// ----------------------------------------------------------------------------
// Subcommand: export
// ----------------------------------------------------------------------------
func exportCredsToOutput(profileName string) error {
	profile, err := retrieveProfile(profileName)
	if err != nil {
		return err
	}

	awsRegion := profile.Key("region").String()
	roleCred, err := getRoleCredentials(profileName, profile, true)
	if err != nil {
		return err
	}

	// Print in a shell-exportable format

	printEnvVariable("AWS_ACCESS_KEY_ID", roleCred.AccessKeyId)
	printEnvVariable("AWS_SECRET_ACCESS_KEY", roleCred.SecretAccessKey)
	printEnvVariable("AWS_SESSION_TOKEN", roleCred.SessionToken)
	printEnvVariable("AWS_SECURITY_TOKEN", roleCred.SessionToken)

	if awsRegion != "" {
		printEnvVariable("AWS_DEFAULT_REGION", awsRegion)
	}
	return nil
}

func printEnvVariable(name, value string) {
	if value != "" {
		fmt.Printf("export %s=%q\n", name, value)
	}
}
