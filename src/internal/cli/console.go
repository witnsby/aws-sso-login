package cli

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
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

	manager.region = consoleRegion(manager.profile)
	manager.account = manager.profile.AccountID
	if err = manager.validateProfileParams(); err != nil {
		logrus.Error(err)
		return err
	}
	// Construct the sign-in URL
	signinURL := manager.generateSigninURL()
	// Handle optional logout
	manager.handleLogout(forceLogout, logoutWait)
	// Open the new session in a browser
	openURL(signinURL)
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

// handleLogout opens the console logout page when forceLogout is true and
// then waits logoutWait seconds. logoutWait alone never triggers a logout.
func (m *awsCredentialsManager) handleLogout(forceLogout bool, logoutWait int) {
	if !forceLogout {
		return
	}
	openURL(helper.ConsoleLogout(m.region))
	if logoutWait > 0 {
		sleep(time.Duration(logoutWait) * time.Second)
	}
}

// consoleRegion returns the region the console opens in: the profile's
// region when set, otherwise its sso_region.
func consoleRegion(p profiles.Profile) string {
	if p.Region != "" {
		return p.Region
	}
	return p.SSORegion
}

// validateProfileParams checks required profile parameters.
func (m *awsCredentialsManager) validateProfileParams() error {
	if m.region == "" || m.account == "" {
		return fmt.Errorf("sso_region or sso_account_id is missing for profile %s", m.profileName)
	}
	return nil
}
