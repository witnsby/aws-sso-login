package cli

import (
	"encoding/json"
	"fmt"

	"github.com/witnsby/aws-sso-login/src/internal/model"
)

// processCreds prints credential_process compatible JSON for a profile.
// stdout carries only the JSON document.
func processCreds(profileName string) error {
	_, roleCred, err := retrieveCredentials(profileName, false)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(createCredentialsPayload(roleCred))
	if err != nil {
		return fmt.Errorf("encoding credential_process output: %w", err)
	}
	fmt.Fprintln(stdout, string(payload))
	return nil
}

// createCredentialsPayload returns the credential_process document (Version 1)
// for the role credentials.
func createCredentialsPayload(roleCred *model.RoleCredential) *model.RoleCredential {
	return &model.RoleCredential{
		Version:         1,
		AccessKeyId:     roleCred.AccessKeyId,
		SecretAccessKey: roleCred.SecretAccessKey,
		SessionToken:    roleCred.SessionToken,
		Expiration:      roleCred.Expiration,
	}
}
