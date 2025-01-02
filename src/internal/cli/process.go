package cli

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/model"
)

// processCreds processes credentials for a given profile and return a JSON output.
func processCreds(profileName string) error {
	logrus.Infof("Processing credentials for profile: %s", profileName)

	// Retrieve the profile
	profile, err := retrieveProfile(profileName)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to retrieve profile for %s", profileName)
		return fmt.Errorf("failed to retrieve profile: %w", err)
	}
	logrus.Infof("Successfully retrieved profile for %s", profileName)

	// Get role credentials
	roleCred, err := getRoleCredentials(profileName, profile, true)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to get role credentials for profile: %s", profileName)
		return fmt.Errorf("failed to get role credentials: %w", err)
	}
	logrus.Infof("Successfully retrieved role credentials for profile %s", profileName)

	// Create and marshal the credentials payload
	credentials := createCredentialsPayload(roleCred)
	payload, err := json.Marshal(credentials)
	if err != nil {
		logrus.WithError(err).Errorf("Failed to marshal credentials for profile: %s", profileName)
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	logrus.Infof("Successfully marshaled credentials for profile: %s", profileName)
	fmt.Println(string(payload))

	return nil
}

// createCredentialsPayload creates and returns a RoleCredential with all necessary fields, including a predefined version.
func createCredentialsPayload(roleCred *model.RoleCredential) *model.RoleCredential {
	logrus.Info("Creating credentials payload")

	// Assign Version and return the updated RoleCredential structure
	return &model.RoleCredential{
		Version:         1, // Static version field
		AccessKeyId:     roleCred.AccessKeyId,
		SecretAccessKey: roleCred.SecretAccessKey,
		SessionToken:    roleCred.SessionToken,
		Expiration:      roleCred.Expiration,
	}
}
