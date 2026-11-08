package cli

import (
	"fmt"
	"strings"
)

// exportCredsToOutput retrieves AWS credentials and region for a given profile
// and prints them in a shell-exportable format.
//
// Parameters:
// - profileName: The name of the AWS profile to export.
//
// Behavior:
// 1. Retrieves the AWS profile and associated region.
// 2. Fetches role-based credentials (Access Key, Secret Key, Session Token).
// 3. Prints the credentials and region as environment variables:
//
//   - AWS_ACCESS_KEY_ID
//   - AWS_SECRET_ACCESS_KEY
//   - AWS_SESSION_TOKEN
//   - AWS_SECURITY_TOKEN
//   - AWS_DEFAULT_REGION (if a region is set).
//
// Returns:
// - An error if profile retrieval or credential generation fails.
func exportCredsToOutput(profileName string) error {
	profile, roleCred, err := retrieveCredentials(profileName, false)
	if err != nil {
		return err
	}
	awsRegion := profile.Region

	// Print in a shell-exportable format
	printEnvVariable("AWS_ACCESS_KEY_ID", roleCred.AccessKeyId)
	printEnvVariable("AWS_SECRET_ACCESS_KEY", roleCred.SecretAccessKey)
	printEnvVariable("AWS_SESSION_TOKEN", roleCred.SessionToken)
	printEnvVariable("AWS_SECURITY_TOKEN", roleCred.SessionToken)

	if awsRegion != "" {
		printEnvVariable("AWS_DEFAULT_REGION", awsRegion)
	}
	return nil
}

func printEnvVariable(name, value string) {
	if value != "" {
		fmt.Fprintf(stdout, "export %s=%s\n", name, shellQuote(value))
	}
}

// shellQuote wraps value in double quotes for POSIX shells, escaping the
// characters that keep a special meaning inside double quotes ($, `, " and
// \), so `eval` assigns the value verbatim. Go's %q is not enough: it leaves
// $ and ` unescaped and uses escapes such as \x00 that shells do not
// understand.
func shellQuote(value string) string {
	var b strings.Builder
	b.Grow(len(value) + 2)
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '$', '`', '"', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
