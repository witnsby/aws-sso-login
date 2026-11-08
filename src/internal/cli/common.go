package cli

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// errSSORoleNoAccess indicates that the AWS SSO identity is authenticated but
// the configured role is not assigned to the user (e.g. AWS SSO returned
// ForbiddenException / AccessDeniedException). Re-running `aws sso login`
// cannot resolve this; the user must obtain access from an administrator.
var errSSORoleNoAccess = errors.New("no access to the configured SSO role")

// importCreds retrieves AWS credentials for the specified profile,
// writes them to the AWS credentials file under that profile section,
// and ensures the credentials are saved properly.
type awsCredentialsManager struct {
	profileName     string
	credentialsPath string
	credsFile       *ini.File
	roleCred        *model.RoleCredential
	profile         profiles.Profile
	region          string
	account         string
	signinToken     string
}

// retrieveAndSetProfile retrieves an AWS profile, fetches role credentials, and sets them for the credentials manager.
// On a recoverable failure (e.g. expired SSO token) it performs a single
// `aws sso login` attempt and retries credential retrieval once. On an
// unrecoverable failure (the SSO role is not assigned to the user, surfaced
// as errSSORoleNoAccess) it returns immediately without invoking the login
// flow, to avoid an infinite re-prompt loop.
func (m *awsCredentialsManager) retrieveAndSetProfile() error {
	profile, err := resolveProfile(m.profileName)
	if err != nil {
		logrus.Info(err)
		return err
	}
	m.profile = profile

	roleCred, err := getRoleCredentials(m.profileName, m.profile, false)
	if err == nil {
		m.roleCred = roleCred
		return nil
	}

	if errors.Is(err, errSSORoleNoAccess) {
		return err
	}

	logrus.Infof("Credentials for profile [%s] not available, attempting `aws sso login`...", m.profileName)
	if loginErr := m.performSSOLogin(); loginErr != nil {
		return loginErr
	}

	roleCred, err = getRoleCredentials(m.profileName, m.profile, false)
	if err != nil {
		return err
	}
	m.roleCred = roleCred
	return nil
}

// performSSOLogin runs the AWS CLI SSO login command for the specified profile and returns an error if the command fails.
func (m *awsCredentialsManager) performSSOLogin() error {
	cmd := execCommand("aws", "sso", "login", "--profile", m.profileName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute aws sso login: %v - %s", err, stderr.String())
	}

	return nil
}

// getRoleCredentials either reads from the local AWS CLI cache or triggers
// the AWS CLI to refresh the cache by calling `aws sts get-caller-identity`.
func getRoleCredentials(profileName string, profile profiles.Profile, silent bool) (*model.RoleCredential, error) {
	// Try reading from the CLI cache
	roleCred, err := getCachedRoleCredentials(profile)
	if err == nil && roleCred != nil {
		// If credentials are unexpired, return them
		if !isExpired(roleCred.Expiration) {
			return roleCred, nil
		}
		// Otherwise, we'll refresh
	}

	// If we couldn't read them, or they're expired, attempt to refresh
	if err = updateCachedRoleCredentials(profileName, silent); err != nil {
		return nil, err
	}

	// Then try again
	roleCred, err = getCachedRoleCredentials(profile)
	if err != nil {
		return nil, err
	}
	if roleCred == nil {
		return nil, fmt.Errorf("could not retrieve credentials for '%s'", profileName)
	}
	// Final check
	if isExpired(roleCred.Expiration) {
		return nil, fmt.Errorf("credentials for '%s' are expired", profileName)
	}
	return roleCred, nil
}

// updateCachedRoleCredentials calls `aws sts get-caller-identity --profile=XYZ`.
// This triggers the AWS CLI's SSO logic to refresh ~/.aws/cli/cache.
//
// If the AWS CLI exits with a "no access" condition (ForbiddenException /
// AccessDeniedException returned by the SSO GetRoleCredentials API), the
// returned error wraps errSSORoleNoAccess so callers can distinguish it from
// transient/expired-token failures.
func updateCachedRoleCredentials(profileName string, silent bool) error {
	cmd := execCommand("aws", "sts", "get-caller-identity",
		"--query", "Arn", "--output", "text", "--profile", profileName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			logrus.Debug(stderrStr)
		}
		if isSSORoleNoAccessStderr(stderrStr) {
			return fmt.Errorf("%w for profile %q", errSSORoleNoAccess, profileName)
		}
		return fmt.Errorf("please login with 'aws sso login --profile=%s'", profileName)
	}
	if !silent {
		fmt.Printf("Updated credentials for: %s\n", string(output))
	}
	return nil
}

// isSSORoleNoAccessStderr returns true when the AWS CLI stderr indicates the
// authenticated user has no access to the configured SSO role. These cases
// cannot be resolved by re-running `aws sso login`.
func isSSORoleNoAccessStderr(stderr string) bool {
	if stderr == "" {
		return false
	}
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "forbiddenexception") ||
		strings.Contains(lower, "accessdeniedexception") ||
		strings.Contains(lower, "no access")
}

// buildCacheFilePath returns the AWS CLI cache file that holds the role
// credentials of profile.
func buildCacheFilePath(profile profiles.Profile) (string, error) {
	cachePath, err := helper.AwsCliCachePath()
	if err != nil {
		return "", err
	}
	key, err := cacheKey(profile)
	if err != nil {
		return "", err
	}
	return filepath.Join(cachePath, key+".json"), nil
}

// cacheKey reproduces botocore's SSOCredentialFetcher._create_cache_key:
// the SHA-1 hex digest of compact JSON with sorted keys holding accountId,
// roleName and either sessionName (sso-session profiles) or startUrl
// (legacy profiles). encoding/json sorts map keys and emits compact output;
// HTML escaping is disabled to match Python's json.dumps.
func cacheKey(profile profiles.Profile) (string, error) {
	args := map[string]string{
		"accountId": profile.AccountID,
		"roleName":  profile.RoleName,
	}
	if profile.SessionName != "" {
		args["sessionName"] = profile.SessionName
	} else {
		args["startUrl"] = profile.StartURL
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(args); err != nil {
		return "", fmt.Errorf("encoding cache key: %w", err)
	}
	sum := sha1.Sum(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:]), nil
}

// getCachedRoleCredentials looks up ~/.aws/cli/cache/<sha1>.json
func getCachedRoleCredentials(profile profiles.Profile) (*model.RoleCredential, error) {
	fullPath, err := buildCacheFilePath(profile)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	// Attempt to read
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Credentials model.RoleCredential `json:"Credentials"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &raw.Credentials, nil
}

func isExpired(expirationTime string) bool {
	parsedTime, err := parseExpirationTime(expirationTime)
	if err != nil {
		// If parsing fails, assume it's expired so user can refresh
		return true
	}
	return now().After(parsedTime)
}

// parseExpirationTime attempts to parse a string into a time.Time using multiple layouts
func parseExpirationTime(expirationTime string) (time.Time, error) {
	for _, layout := range helper.TimeLayouts {
		if t, err := time.Parse(layout, expirationTime); err == nil {
			return t, nil
		}
	}
	// Return zero time if parsing fails
	return time.Time{}, fmt.Errorf("unable to parse expiration time")
}

// getSigninToken obtains a federation sign-in token from AWS Federation endpoint
func getSigninToken(rc *model.RoleCredential) (string, error) {
	params := url.Values{}
	params.Set("Action", "getSigninToken")
	params.Set("SessionDuration", helper.SessionDuration)
	sess := map[string]string{
		"sessionId":    rc.AccessKeyId,
		"sessionKey":   rc.SecretAccessKey,
		"sessionToken": rc.SessionToken,
	}
	sessB, _ := json.Marshal(sess)
	params.Set("Session", string(sessB))

	fedURL := federationEndpoint + "?" + params.Encode()

	out, err := getURL(fedURL)
	if err != nil {
		return "", err
	}

	var resp struct {
		SigninToken string `json:"SigninToken"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	return resp.SigninToken, nil
}

func getURL(raw string) ([]byte, error) {
	// Perform an HTTP GET request
	resp, err := httpClient.Get(raw)
	if err != nil {
		return nil, err
	}
	// Ensure the response body is closed
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {

		}
	}(resp.Body)

	// Read the body of the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// resolveProfile loads the named profile from the AWS config file and
// validates its SSO configuration (legacy or sso-session).
func resolveProfile(profileName string) (profiles.Profile, error) {
	configPath, err := helper.AwsConfigPath()
	if err != nil {
		return profiles.Profile{}, err
	}
	return profiles.Resolve(configPath, profileName)
}
