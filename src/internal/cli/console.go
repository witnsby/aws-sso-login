package cli

import (
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"net/url"
	"time"
)

// console generates a sign-in URL for an AWS SSO console session and opens it in a browser.
// It optionally logs out of an existing session before opening the new one.
func console(profileName string, forceLogout bool, logoutWait int) error {
	// Retrieve profile details
	profile, err := retrieveProfile(profileName)
	if err != nil {
		return err
	}

	// Fetch credentials and sign-in token
	roleCred, err := getRoleCredentials(profileName, profile, false)
	if err != nil {
		return err
	}
	signinToken, err := getSigninToken(roleCred)
	if err != nil {
		return err
	}

	// Extract required parameters
	region := profile.Key("sso_region").String()
	account := profile.Key("sso_account_id").String()
	if err := validateProfileParams(profileName, region, account); err != nil {
		return err
	}

	// Construct the sign-in URL
	signinURL := generateSigninURL(region, account, signinToken)

	// Handle optional logout
	handleLogout(region, forceLogout, logoutWait)

	// Open the new session in a browser
	openBrowser(signinURL)
	return nil
}

// validateProfileParams checks required profile parameters.
func validateProfileParams(profileName, region, account string) error {
	if region == "" || account == "" {
		return fmt.Errorf("sso_region or sso_account_id is missing for profile %s", profileName)
	}
	return nil
}

// generateSigninURL builds the AWS console sign-in URL.
func generateSigninURL(region, account, signinToken string) string {
	params := url.Values{}
	params.Set("Action", "login")
	params.Set("Issuer", "-aws-sso-console")
	params.Set("Destination", helper.ConsoleUrl(region))
	params.Set("SigninToken", signinToken)
	return fmt.Sprintf("https://%s.signin.aws.amazon.com/federation?%s", account, params.Encode())
}

// handleLogout optionally logs out of the existing session.
func handleLogout(region string, forceLogout bool, logoutWait int) {
	if forceLogout || logoutWait > 0 {
		logoutURL := helper.ConsoleLogout(region)
		openBrowser(logoutURL)
		if logoutWait > 0 {
			time.Sleep(time.Duration(logoutWait) * time.Second)
		}
	}
}
