package cli

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// errSSORoleNoAccess indicates that the AWS SSO identity is authenticated but
// the configured role is not assigned to the user (e.g. AWS SSO returned
// ForbiddenException / AccessDeniedException). Re-running `aws sso login`
// cannot resolve this; the user must obtain access from an administrator.
var errSSORoleNoAccess = errors.New("no access to the configured SSO role")

// errAWSCLINotFound indicates that the aws executable is not in PATH. Running
// `aws sso login` cannot help, so the login retry is skipped.
var errAWSCLINotFound = errors.New(`AWS CLI v2 is required but "aws" was not found in PATH`)

// awsCommand builds an AWS CLI command, or returns errAWSCLINotFound when
// the aws executable is not in PATH.
func awsCommand(args ...string) (*exec.Cmd, error) {
	if _, err := lookPath("aws"); err != nil {
		return nil, errAWSCLINotFound
	}
	return execCommand("aws", args...), nil
}

// mapAWSExecError turns a "not found" error from running the AWS CLI into
// errAWSCLINotFound and returns any other error unchanged.
func mapAWSExecError(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return errAWSCLINotFound
	}
	return err
}

// retrieveCredentials resolves profileName and returns its role credentials.
// With allowLogin (import, console), a recoverable failure such as an
// expired SSO token triggers a single `aws sso login` and one retry. A role
// without access or a missing AWS CLI fails immediately, since logging in
// cannot fix either.
func retrieveCredentials(profileName string, allowLogin bool) (profiles.Profile, *model.RoleCredential, error) {
	profile, err := resolveProfile(profileName)
	if err != nil {
		return profiles.Profile{}, nil, err
	}

	roleCred, err := getRoleCredentials(profileName, profile)
	if err == nil {
		return profile, roleCred, nil
	}
	if !allowLogin || errors.Is(err, errSSORoleNoAccess) || errors.Is(err, errAWSCLINotFound) {
		return profiles.Profile{}, nil, err
	}

	logrus.Infof("Credentials for profile [%s] not available, attempting `aws sso login`...", profileName)
	if err := ssoLogin(profileName); err != nil {
		return profiles.Profile{}, nil, err
	}

	roleCred, err = getRoleCredentials(profileName, profile)
	if err != nil {
		return profiles.Profile{}, nil, err
	}
	return profile, roleCred, nil
}

// ssoLogin runs `aws sso login` for the profile. The AWS CLI prints the
// authorization URL and verification code, so stdin is the terminal and both
// output streams go to stderr: the user sees the instructions and stdout
// stays clean.
func ssoLogin(profileName string) error {
	cmd, err := awsCommand("sso", "login", "--profile", profileName)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aws sso login failed for profile %q: %w", profileName, mapAWSExecError(err))
	}
	return nil
}

// getRoleCredentials either reads from the local AWS CLI cache or triggers
// the AWS CLI to refresh the cache by calling `aws sts get-caller-identity`.
func getRoleCredentials(profileName string, profile profiles.Profile) (*model.RoleCredential, error) {
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
	if err = updateCachedRoleCredentials(profileName); err != nil {
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
func updateCachedRoleCredentials(profileName string) error {
	cmd, err := awsCommand("sts", "get-caller-identity",
		"--query", "Arn", "--output", "text", "--profile", profileName)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if mapped := mapAWSExecError(err); errors.Is(mapped, errAWSCLINotFound) {
			return mapped
		}
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			logrus.Debug(stderrStr)
		}
		if isSSORoleNoAccessStderr(stderrStr) {
			return fmt.Errorf("%w for profile %q", errSSORoleNoAccess, profileName)
		}
		return fmt.Errorf("could not refresh credentials for profile %q; run 'aws sso login --profile %s'", profileName, profileName)
	}
	logrus.Infof("Updated credentials for %s", strings.TrimSpace(string(output)))
	return nil
}

// isSSORoleNoAccessStderr returns true when the AWS CLI stderr names
// ForbiddenException or AccessDeniedException, which means the authenticated
// user has no access to the configured SSO role and re-running
// `aws sso login` cannot help. Plain "no access" text without an exception
// name is not enough: it also appears in unrelated proxy or network errors.
func isSSORoleNoAccessStderr(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "forbiddenexception") ||
		strings.Contains(lower, "accessdeniedexception")
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

// minCredentialValidity is how long credentials must remain valid to be
// returned. Credentials closer to expiry are refreshed. botocore refreshes
// cached SSO credentials within 15 minutes of expiry, so a refresh always
// replaces credentials rejected by this margin.
const minCredentialValidity = 5 * time.Minute

// isExpired reports whether credentials with the given expiration time must
// be refreshed: the time cannot be parsed or is less than
// minCredentialValidity away.
func isExpired(expirationTime string) bool {
	parsedTime, err := parseExpirationTime(expirationTime)
	if err != nil {
		return true
	}
	return parsedTime.Sub(now()) < minCredentialValidity
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
