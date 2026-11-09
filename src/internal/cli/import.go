package cli

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// importCreds retrieves credentials for a profile (logging in once if the SSO
// session has expired) and writes them to the shared credentials file under
// a section named after the profile.
func importCreds(profileName string) error {
	_, roleCred, err := retrieveCredentials(profileName, true)
	if err != nil {
		return err
	}

	path, err := helper.AwsCredentialsPath()
	if err != nil {
		return err
	}
	if err := updateCredentialsFile(path, profileName, roleCred); err != nil {
		return err
	}

	report.Success("Credentials for [%s] written to %s (%s)", profileName, displayPath(path), expiryText(roleCred.Expiration))
	return nil
}

// displayPath shows path with the user's home directory as "~".
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rel, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rel
	}
	return path
}

// expiryText describes when credentials expire: the local time (with the
// date when it is not today) and the time left, truncated to minutes. The
// location comes from the now seam so tests can pin it.
func expiryText(expiration string) string {
	exp, err := parseExpirationTime(expiration)
	if err != nil {
		return "expiry unknown"
	}
	current := now()
	exp = exp.In(current.Location())
	layout := "15:04"
	if y, m, d := exp.Date(); y != current.Year() || m != current.Month() || d != current.Day() {
		layout = "2006-01-02 15:04"
	}
	return "valid until " + exp.Format(layout) + ", " + formatRemaining(exp.Sub(current)) + " left"
}

// formatRemaining formats d truncated to minutes, such as "8h0m" or "45m".
func formatRemaining(d time.Duration) string {
	d = d.Truncate(time.Minute)
	if d <= 0 {
		return "0m"
	}
	return strings.TrimSuffix(d.String(), "0s")
}
