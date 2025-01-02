package cli

import (
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"os"
)

// importCreds retrieves AWS credentials for the specified profile,
// writes them to the AWS credentials file under that profile section,
// and ensures the credentials are saved properly.
type awsCredentialsManager struct {
	profileName     string
	credentialsPath string
	credsFile       *ini.File
	roleCred        *model.RoleCredential
}

// importCreds retrieves AWS credentials for a profile,
// writes them in the credentials file, and saves the updated file.
func importCreds(profileName string) error {
	// Initialize the credentials manager
	manager := awsCredentialsManager{profileName: profileName}

	// Retrieve AWS profile and credentials
	if err := manager.retrieveAndSetProfile(); err != nil {
		return err
	}

	// Load or initialize the credentials file
	if err := manager.loadOrInitCredsFile(); err != nil {
		return err
	}

	// Update the credentials in the profile section
	if err := manager.updateProfileWithCreds(); err != nil {
		return err
	}

	// Save the credentials file
	if err := manager.saveCredsFile(); err != nil {
		return err
	}

	logrus.Infof("Wrote credentials to profile [%s] in %s\n", manager.profileName, manager.credentialsPath)
	return nil
}

// retrieveAndSetProfile retrieves the profile and corresponding role credentials.
func (m *awsCredentialsManager) retrieveAndSetProfile() error {
	profile, err := retrieveProfile(m.profileName)
	if err != nil {
		logrus.Info(err)
		return err
	}

	roleCred, err := getRoleCredentials(m.profileName, profile, false)
	if err != nil {
		return err
	}
	m.roleCred = roleCred
	return nil
}

// loadOrInitCredsFile loads the credentials file or initializes an empty one if it doesn't exist.
func (m *awsCredentialsManager) loadOrInitCredsFile() error {
	path, err := helper.GetAwsCredentialsPath()
	if err != nil {
		return err
	}
	m.credentialsPath = path

	credsFile, err := ini.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.credsFile = ini.Empty()
			return nil
		}
		return err
	}
	m.credsFile = credsFile
	return nil
}

// updateProfileWithCreds updates (or creates) the profile section with role credentials.
func (m *awsCredentialsManager) updateProfileWithCreds() error {
	section, err := m.credsFile.GetSection(m.profileName)
	if err != nil {
		section, _ = m.credsFile.NewSection(m.profileName)
	}

	section.Key("aws_access_key_id").SetValue(m.roleCred.AccessKeyId)
	section.Key("aws_secret_access_key").SetValue(m.roleCred.SecretAccessKey)
	section.Key("aws_session_token").SetValue(m.roleCred.SessionToken)
	section.Key("aws_security_token").SetValue(m.roleCred.SessionToken)

	return nil
}

// saveCredsFile saves the updated credentials file back to the filesystem.
func (m *awsCredentialsManager) saveCredsFile() error {
	return m.credsFile.SaveTo(m.credentialsPath)
}
