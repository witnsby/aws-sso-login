package cli

import (
	"fmt"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
)

func getVersion() string {
	return fmt.Sprintf("Version: %s, Commit: %s\n", helper.Version, helper.CommitHash)
}
