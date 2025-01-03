package cli

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"
)

// performSSOLogin runs the AWS CLI SSO login command for the specified profile and returns an error if the command fails.
func (m *awsCredentialsManager) performSSOLogin() error {
	cmd := exec.Command("aws", "sso", "login", "--profile", m.profileName)
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
func getRoleCredentials(profileName string, profile *ini.Section, silent bool) (*model.RoleCredential, error) {
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
func updateCachedRoleCredentials(profileName string, silent bool) error {
	cmd := exec.Command("aws", "sts", "get-caller-identity",
		"--query", "Arn", "--output", "text", "--profile", profileName)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		logrus.Error(stderr.String())
		return fmt.Errorf("please login with 'aws sso login --profile=%s'", profileName)
	}
	if !silent {
		fmt.Printf("Updated credentials for: %s\n", string(output))
	}
	return nil
}

func buildCacheFilePath(profile *ini.Section) (string, error) {
	cachePath, err := helper.GetAwsCliCachePath()
	if err != nil {
		return "", err
	}

	// Build a cache key (sha1 of sorted JSON: {startUrl, roleName, accountId})
	startURL := profile.Key("sso_start_url").String()
	roleName := profile.Key("sso_role_name").String()
	accountID := profile.Key("sso_account_id").String()

	args := map[string]string{
		"startUrl":  startURL,
		"roleName":  roleName,
		"accountId": accountID,
	}

	b, _ := json.Marshal(args)
	h := sha1.New()
	h.Write(b)
	cacheKey := fmt.Sprintf("%x", h.Sum(nil))

	// Construct the path
	return filepath.Join(cachePath, cacheKey+".json"), nil
}

// getCachedRoleCredentials looks up ~/.aws/cli/cache/<sha1>.json
func getCachedRoleCredentials(profile *ini.Section) (*model.RoleCredential, error) {
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
	return time.Now().After(parsedTime)
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

	fedURL := "https://signin.aws.amazon.com/federation?" + params.Encode()

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
	resp, err := http.Get(raw)
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

// retrieveProfile retrieves and validates an AWS profile from the configuration file.
// It ensures the specified profile contains all required attributes for AWS SSO workflows.
func retrieveProfile(profileName string) (*ini.Section, error) {
	// Get the path to the AWS configuration file.
	configPath, err := GetAwsConfigPath()
	if err != nil {
		logrus.Infof("Cannot locate AWS config file, using default path: %s", configPath)
		return nil, err
	}

	// Load the AWS configuration file.
	configFile, err := ini.Load(configPath)
	if err != nil {
		logrus.Errorf("Failed to load AWS config file at %s: %v", configPath, err)
		return nil, err
	}

	// Build the profile section name based on the input.
	sectionName := fmt.Sprintf("profile %s", profileName)

	// Attempt to retrieve the profile section from the config file.
	section, err := configFile.GetSection(sectionName)
	if err != nil {
		logrus.Errorf("Profile [%s] not found in config file: %s", sectionName, configPath)
		return nil, fmt.Errorf("cannot find profile [%s] in %s", sectionName, configPath)
	}

	// List of required keys for the profile section.
	requiredKeys := []string{"sso_start_url", "sso_account_id", "sso_role_name", "sso_region"}

	// Validate if all required keys are present in the profile section.
	for _, key := range requiredKeys {
		if val := section.Key(key).String(); val == "" {
			return nil, fmt.Errorf("missing required attribute %q in profile %s", key, profileName)
		}
	}

	// Return the valid profile section.
	return section, nil
}

// GetAwsConfigPath determines the file path of the AWS configuration file.
//
// It checks the following sources in order of priority:
//  1. If the "AWS_CONFIG_FILE" environment variable is set, its value is returned.
//  2. If the environment variable is not set, it retrieves the current user's home directory
//     and constructs the default AWS configuration file path: `<HomeDirectory>/.aws/config`.
//
// Returns:
// - The file path as a string if successful.
// - An error if there is an issue retrieving the current user or resolving the path.
func GetAwsConfigPath() (string, error) {
	// Check if the AWS_CONFIG_FILE environment variable is set.
	if configPath := os.Getenv("AWS_CONFIG_FILE"); configPath != "" {
		return configPath, nil
	}

	// Retrieve the current user's home directory.
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve current user: %w", err)
	}

	// Construct the default AWS configuration file path.
	defaultConfigPath := filepath.Join(currentUser.HomeDir, ".aws", "config")
	return defaultConfigPath, nil
}
