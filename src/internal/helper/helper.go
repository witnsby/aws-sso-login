package helper

import (
	"fmt"
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"os"
	"os/user"
	"path/filepath"
)

// ----------------------------------------------------------------------------
// Paths & File helpers
// ----------------------------------------------------------------------------

func GetAwsConfigPath() (string, error) {
	if val := os.Getenv("AWS_CONFIG_FILE"); val != "" {
		return val, nil
	}

	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(currentUser.HomeDir, ".aws", "config"), nil
}

func GetAwsCredentialsPath() (string, error) {
	// By default: ~/.aws/credentials
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".aws", "credentials"), nil
}

func GetAwsCliCachePath() (string, error) {
	// By default: ~/.aws/cli/cache
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, ".aws", "cli", "cache"), nil
}

func ConsoleUrl(region string) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/", region)
}

func ConsoleLogout(region string) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/console/logout!doLogout", region)
}

func RetrieveProfile(profileName string) (*ini.Section, error) {
	getAwsConfigPath, err := GetAwsConfigPath()
	if err != nil {
		return nil, err
	}

	loadAWSConfigFile, err := ini.Load(getAwsConfigPath)
	if err != nil {
		logrus.Errorf("cannot load AWS config file %s: %w", getAwsConfigPath, err)
		return nil, err
	}

	sectionName := fmt.Sprintf("profile %s", profileName)
	section, err := loadAWSConfigFile.GetSection(sectionName)
	if err != nil {
		logrus.Errorf("cannot find profile [%s] in %s", sectionName, getAwsConfigPath)
		return nil, fmt.Errorf("cannot find profile [%s] in %s", sectionName, getAwsConfigPath)
	}

	// Basic checks
	reqKeys := []string{"sso_start_url", "sso_account_id", "sso_role_name", "sso_region"}
	for _, rk := range reqKeys {
		if val := section.Key(rk).String(); val == "" {
			return nil, fmt.Errorf("missing required attribute %q in profile %s", rk, profileName)
		}
	}
	return section, nil
}
