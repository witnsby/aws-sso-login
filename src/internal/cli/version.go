package cli

import (
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

// getVersion returns the single-line version and commit string.
func getVersion() string {
	return fmt.Sprintf("Version: %s, Commit: %s", helper.Version, helper.CommitHash)
}
