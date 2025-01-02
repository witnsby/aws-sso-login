package helper

import (
	"fmt"
	"os/user"
	"path/filepath"
)

// ----------------------------------------------------------------------------
// Paths & File helpers
// ----------------------------------------------------------------------------

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
