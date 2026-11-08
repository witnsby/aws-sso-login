package cli

import (
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// importCreds retrieves credentials for a profile (logging in once if the SSO
// session has expired) and writes them to the shared credentials file under
// a section named after the profile.
func importCreds(profileName string) error {
	_, roleCred, err := retrieveCredentials(profileName, true)
	if err != nil {
		return err
	}

	path, err := helper.AwsCredentialsPath()
	if err != nil {
		return err
	}
	if err := updateCredentialsFile(path, profileName, roleCred); err != nil {
		return err
	}

	logrus.Infof("Wrote credentials to profile [%s] in %s", profileName, path)
	return nil
}
