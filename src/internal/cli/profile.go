package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// defaultSelector is the production Selector used when no profile is given.
// It is a package-level seam so tests can swap in a deterministic fake.
var defaultSelector Selector = HuhSelector{}

// resolveProfileName returns the AWS profile name a command works on, in
// order of precedence: a non-empty --profile flag, a non-empty $AWS_PROFILE,
// then the interactive picker. The picker is used only when allowPicker is
// true and both stdin and stderr are terminals; otherwise a missing profile
// is an error asking for --profile.
func resolveProfileName(flagValue string, allowPicker bool) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv("AWS_PROFILE"); env != "" {
		return env, nil
	}
	if !allowPicker || !isInteractive() {
		return "", newNoProfileError()
	}

	configPath, err := helper.AwsConfigPath()
	if err != nil {
		return "", fmt.Errorf("could not determine AWS config path: %w", err)
	}

	ssoProfiles, err := profiles.ListSSOProfiles(configPath)
	if err != nil {
		return "", fmt.Errorf("could not list SSO profiles: %w", err)
	}

	name, err := defaultSelector.Pick(ssoProfiles)
	if errors.Is(err, errSelectionCancelled) {
		return "", newSelectionCancelledError(err)
	}
	if err != nil {
		return "", fmt.Errorf("could not select profile: %w", err)
	}
	return name, nil
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
