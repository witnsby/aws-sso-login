package cli

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"net/url"
	"time"
)

// console generates a sign-in URL for an AWS SSO console session and opens it in a browser.
// It optionally logs out of an existing session before opening the new one.
func console(profile string, forceLogout bool, logoutWait int) error {
	// Retrieve profile details
	manager := awsCredentialsManager{profileName: profile}
	// Retrieve AWS profile and credentials
	if err := manager.retrieveAndSetProfile(); err != nil {
		return err
	}
	signinToken, err := getSigninToken(manager.roleCred)
	if err != nil {
		return err
	}
	manager.signinToken = signinToken

	manager.region = manager.profile.Key("sso_region").String()
	manager.account = manager.profile.Key("sso_account_id").String()
	if err = manager.validateProfileParams(); err != nil {
		logrus.Error(err)
		return err
	}
	// Construct the sign-in URL
	signinURL := manager.generateSigninURL()
	// Handle optional logout
	manager.handleLogout(forceLogout, logoutWait)
	// Open the new session in a browser
	openBrowser(signinURL)
	return nil
}

// generateSigninURL builds the AWS console sign-in URL.
func (m *awsCredentialsManager) generateSigninURL() string {
	params := url.Values{}
	params.Set("Action", "login")
	params.Set("Issuer", "-aws-sso-console")
	params.Set("Destination", helper.ConsoleUrl(m.region))
	params.Set("SigninToken", m.signinToken)
	return fmt.Sprintf("https://%s.signin.aws.amazon.com/federation?%s", m.account, params.Encode())
}

// handleLogout optionally logs out of the existing session.
func (m *awsCredentialsManager) handleLogout(forceLogout bool, logoutWait int) {
	if forceLogout || logoutWait > 0 {
		logoutURL := helper.ConsoleLogout(m.region)
		openBrowser(logoutURL)
		if logoutWait > 0 {
			time.Sleep(time.Duration(logoutWait) * time.Second)
		}
	}
}

// validateProfileParams checks required profile parameters.
func (m *awsCredentialsManager) validateProfileParams() error {
	if m.region == "" || m.account == "" {
		return fmt.Errorf("sso_region or sso_account_id is missing for profile %s", m.profileName)
	}
	return nil
}
