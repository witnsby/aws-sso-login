package cli

import (
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"net/url"
	"time"
)

func console(profileName string, forceLogout bool, logoutWait int) error {
	profile, err := helper.RetrieveProfile(profileName)
	if err != nil {
		return err
	}

	// Retrieve role credentials
	roleCred, err := getRoleCredentials(profileName, profile, false)
	if err != nil {
		return err
	}

	// Federation Sign-In Token
	signinToken, err := getSigninToken(roleCred)
	if err != nil {
		return err
	}

	region := profile.Key("sso_region").String()
	account := profile.Key("sso_account_id").String()
	if region == "" || account == "" {
		return fmt.Errorf("sso_region or sso_account_id is missing for profile %s", profileName)
	}

	// https://<account>.signin.aws.amazon.com/federation
	params := url.Values{}
	params.Set("Action", "login")
	params.Set("Issuer", "-aws-sso-console")
	params.Set("Destination", helper.ConsoleUrl(region))
	params.Set("SigninToken", signinToken)

	// Optional: Log out of old session
	if forceLogout || logoutWait > 0 {
		logoutURL := helper.ConsoleLogout(region)
		openBrowser(logoutURL)
		if logoutWait > 0 {
			time.Sleep(time.Duration(logoutWait) * time.Second)
		}
	}

	// Open new session
	openBrowser(fmt.Sprintf("https://%s.signin.aws.amazon.com/federation?%s", account, params.Encode()))
	return nil
}
