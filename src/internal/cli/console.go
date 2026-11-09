package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// consoleSignin holds what is needed to open an AWS console session.
type consoleSignin struct {
	region      string
	account     string
	signinToken string
}

// console signs into the AWS Management Console for a profile and opens it
// in the browser, optionally logging out of an existing session first.
func console(profileName string, forceLogout bool, logoutWait int) error {
	profile, roleCred, err := retrieveCredentials(profileName, true)
	if err != nil {
		return err
	}
	signinToken, err := getSigninToken(roleCred)
	if err != nil {
		return err
	}

	s := consoleSignin{
		region:      consoleRegion(profile),
		account:     profile.AccountID,
		signinToken: signinToken,
	}
	signinURL := s.signinURL()
	s.handleLogout(forceLogout, logoutWait)
	openURL(signinURL)
	report.Success("Opened the AWS console for [%s] in %s", profileName, s.region)
	return nil
}

// signinURL builds the AWS console sign-in URL.
func (s consoleSignin) signinURL() string {
	params := url.Values{}
	params.Set("Action", "login")
	params.Set("Issuer", "-aws-sso-console")
	params.Set("Destination", helper.ConsoleUrl(s.region))
	params.Set("SigninToken", s.signinToken)
	return fmt.Sprintf("https://%s.signin.aws.amazon.com/federation?%s", s.account, params.Encode())
}

// handleLogout opens the console logout page when forceLogout is true and
// then waits logoutWait seconds. logoutWait alone never triggers a logout.
func (s consoleSignin) handleLogout(forceLogout bool, logoutWait int) {
	if !forceLogout {
		return
	}
	openURL(helper.ConsoleLogout(s.region))
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

// maxFederationResponse caps how much of the federation response is read.
const maxFederationResponse = 1 << 20

// getSigninToken obtains a federation sign-in token for the role credentials.
// The request URL carries the credentials, so no error returned from here
// contains the URL or the response body.
func getSigninToken(rc *model.RoleCredential) (string, error) {
	params := url.Values{}
	params.Set("Action", "getSigninToken")
	params.Set("SessionDuration", helper.SessionDuration)
	sess := map[string]string{
		"sessionId":    rc.AccessKeyId,
		"sessionKey":   rc.SecretAccessKey,
		"sessionToken": rc.SessionToken,
	}
	sessB, err := json.Marshal(sess)
	if err != nil {
		return "", fmt.Errorf("encoding federation session: %w", err)
	}
	params.Set("Session", string(sessB))

	req, err := http.NewRequest(http.MethodGet, federationEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", errors.New("building federation request failed")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return "", fmt.Errorf("federation request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("federation endpoint returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFederationResponse))
	if err != nil {
		return "", fmt.Errorf("reading federation response: %w", err)
	}

	var out struct {
		SigninToken string `json:"SigninToken"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", errors.New("federation endpoint returned an invalid response")
	}
	if out.SigninToken == "" {
		return "", errors.New("federation endpoint returned no sign-in token")
	}
	return out.SigninToken, nil
}
