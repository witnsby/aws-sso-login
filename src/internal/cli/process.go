package cli

import (
	"encoding/json"
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// ----------------------------------------------------------------------------
// Subcommand: process
// ----------------------------------------------------------------------------

func processCreds(profileName string) error {
	profile, err := retrieveProfile(profileName)
	if err != nil {
		return err
	}

	roleCred, err := getRoleCredentials(profileName, profile, true)
	if err != nil {
		return err
	}

	// Add "Version" : 1
	out := map[string]interface{}{
		"Version":         1,
		"AccessKeyId":     roleCred.AccessKeyId,
		"SecretAccessKey": roleCred.SecretAccessKey,
		"SessionToken":    roleCred.SessionToken,
		"Expiration":      roleCred.Expiration,
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return nil
}
