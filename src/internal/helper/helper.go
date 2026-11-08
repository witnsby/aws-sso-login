package helper

import (
	"fmt"
	"os"
	"path/filepath"
)

// ----------------------------------------------------------------------------
// Paths & File helpers
// ----------------------------------------------------------------------------

// homeDir returns the current user's home directory. os.UserHomeDir honors
// $HOME, which lets tests point it at a temporary directory.
func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return home, nil
}

// AwsConfigPath returns the AWS config file path: $AWS_CONFIG_FILE when set,
// otherwise ~/.aws/config.
func AwsConfigPath() (string, error) {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aws", "config"), nil
}

// AwsCredentialsPath returns the AWS shared credentials file path:
// $AWS_SHARED_CREDENTIALS_FILE when set, otherwise ~/.aws/credentials.
func AwsCredentialsPath() (string, error) {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aws", "credentials"), nil
}

// AwsCliCachePath returns the directory where AWS CLI v2 caches SSO role
// credentials (~/.aws/cli/cache).
func AwsCliCachePath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aws", "cli", "cache"), nil
}

func ConsoleUrl(region string) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/", region)
}

func ConsoleLogout(region string) string {
	return fmt.Sprintf("https://%s.console.aws.amazon.com/console/logout!doLogout", region)
}
