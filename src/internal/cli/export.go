package cli

import (
	"fmt"
)

// exportCredsToOutput retrieves AWS credentials and region for a given profile
// and prints them in a shell-exportable format.
//
// Parameters:
// - profileName: The name of the AWS profile to export.
//
// Behavior:
// 1. Retrieves the AWS profile and associated region.
// 2. Fetches role-based credentials (Access Key, Secret Key, Session Token).
// 3. Prints the credentials and region as environment variables:
//
//   - AWS_ACCESS_KEY_ID
//   - AWS_SECRET_ACCESS_KEY
//   - AWS_SESSION_TOKEN
//   - AWS_SECURITY_TOKEN
//   - AWS_DEFAULT_REGION (if a region is set).
//
// Returns:
// - An error if profile retrieval or credential generation fails.
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
