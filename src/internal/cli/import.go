package cli

import (
	"fmt"
	"github.com/go-ini/ini"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"os"
)

// ----------------------------------------------------------------------------
// Subcommand: import
// ----------------------------------------------------------------------------

func importCreds(profileName string) error {
	profile, err := helper.RetrieveProfile(profileName)
	if err != nil {
		return err
	}

	roleCred, err := getRoleCredentials(profileName, profile, false)
	if err != nil {
		return err
	}

	credentialsPath, err := helper.GetAwsCredentialsPath()
	if err != nil {
		return err
	}

	credsFile, err := ini.Load(credentialsPath)
	if err != nil {
		// If the file doesn't exist, we create a new one
		if os.IsNotExist(err) {
			credsFile = ini.Empty()
		} else {
			return err
		}
	}

	section, err := credsFile.GetSection(profileName)
	if err != nil {
		section, _ = credsFile.NewSection(profileName)
	}

	section.Key("aws_access_key_id").SetValue(roleCred.AccessKeyId)
	section.Key("aws_secret_access_key").SetValue(roleCred.SecretAccessKey)
	section.Key("aws_session_token").SetValue(roleCred.SessionToken)
	section.Key("aws_security_token").SetValue(roleCred.SessionToken)

	err = credsFile.SaveTo(credentialsPath)
	if err != nil {
		return err
	}

	fmt.Printf("Wrote credentials to profile [%s] in %s\n", profileName, credentialsPath)
	return nil
}
