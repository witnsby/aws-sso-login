package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/witnsby/aws-sso-login/src/internal/helper"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// Exit statuses. They are public API: scripts rely on them.
const (
	exitOK          = 0
	exitFailure     = 1
	exitUsage       = 2
	exitNotSignedIn = 3
	exitNoAccess    = 4
	exitNoAWSCLI    = 5
	exitCancelled   = 130
)

// awsCLIInstallURL is the AWS CLI v2 installation guide.
const awsCLIInstallURL = "https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"

// userError is a failure with the wording shown to the user and the exit
// status it maps to. It wraps its cause, so errors.Is and errors.As see
// through it.
type userError struct {
	title   string   // one line saying what failed
	details []string // up to two lines of specifics
	hint    string   // next step, rendered after "Fix:"
	code    int      // exit status
	err     error    // cause, shown only in debug output
}

func (e *userError) Error() string { return e.title }

func (e *userError) Unwrap() error { return e.err }

// newUsageError marks err as a usage error of cmd (exit status 2).
func newUsageError(cmd *cobra.Command, err error) error {
	return &userError{
		title: err.Error(),
		hint:  fmt.Sprintf("Run '%s --help'", cmd.CommandPath()),
		code:  exitUsage,
		err:   err,
	}
}

// newRoleNoAccessError reports that the role of profile p is not assigned
// to the signed-in user (exit status 4).
func newRoleNoAccessError(p profiles.Profile, cause error) error {
	return &userError{
		title: fmt.Sprintf("No access to the SSO role for profile [%s]", p.Name),
		details: []string{
			fmt.Sprintf("You are signed in to AWS SSO, but role %s in account %s is not assigned to your user.", p.RoleName, p.AccountID),
		},
		hint: "Ask your AWS administrator to assign the role, or choose another profile.",
		code: exitNoAccess,
		err:  cause,
	}
}

// newAWSCLIMissingError reports that the aws executable is not in PATH
// (exit status 5).
func newAWSCLIMissingError() error {
	return &userError{
		title: errAWSCLINotFound.Error(),
		hint:  "Install AWS CLI v2: " + awsCLIInstallURL,
		code:  exitNoAWSCLI,
		err:   errAWSCLINotFound,
	}
}

// newNotSignedInError reports that credentials for profileName could not be
// refreshed without a new SSO sign-in (exit status 3).
func newNotSignedInError(profileName string) error {
	return &userError{
		title:   fmt.Sprintf("Not signed in to AWS SSO for profile [%s]", profileName),
		details: []string{"The SSO session has expired or the credentials could not be refreshed."},
		hint:    fmt.Sprintf("Run 'aws sso login --profile %s', or 'aws-sso-login import --profile %s'.", profileName, profileName),
		code:    exitNotSignedIn,
		err:     fmt.Errorf("could not refresh credentials for profile %q", profileName),
	}
}

// newLoginFailedError reports that `aws sso login` failed (exit status 3).
func newLoginFailedError(profileName string, cause error) error {
	return &userError{
		title: fmt.Sprintf("aws sso login failed for profile [%s]", profileName),
		hint:  "Check the AWS CLI output above and try again.",
		code:  exitNotSignedIn,
		err:   cause,
	}
}

// newNoProfileError reports that no profile name was given where the
// picker cannot be shown (exit status 2).
func newNoProfileError() error {
	return &userError{
		title: "No profile specified",
		hint:  "Pass --profile <name> or set AWS_PROFILE.",
		code:  exitUsage,
		err:   errors.New(helper.ErrorProfileSpecification),
	}
}

// newSelectionCancelledError reports that the user cancelled the picker
// (exit status 130).
func newSelectionCancelledError(cause error) error {
	return &userError{title: "Profile selection cancelled", code: exitCancelled, err: cause}
}

// asUserError returns the userError in err's chain, or a generic one that
// uses err's message as the title (exit status 1).
func asUserError(err error) *userError {
	var ue *userError
	if errors.As(err, &ue) {
		return ue
	}
	if errors.Is(err, errSSORoleNoAccess) {
		return &userError{
			title:   "No access to the configured SSO role",
			details: []string{"You are signed in to AWS SSO, but this role is not assigned to your user."},
			hint:    "Ask your AWS administrator to assign the role, or choose another profile.",
			code:    exitNoAccess,
			err:     err,
		}
	}
	return &userError{title: err.Error(), code: exitFailure, err: err}
}

// reportError renders the single user-facing message for a failure of cmd
// and returns the exit status. Commands return wrapped errors without
// logging them, so this is the only place an error is printed. process gets
// a one-line message because SDKs embed credential_process stderr in their
// own errors.
func reportError(cmd *cobra.Command, err error) int {
	name := ""
	if cmd != nil {
		name = cmd.Name()
	}
	r := newReporter(name)
	ue := asUserError(err)
	if name == "process" {
		fmt.Fprintln(stderrOut, r.line(kindError, ue.oneLine()))
	} else {
		fmt.Fprint(stderrOut, ue.block(r))
	}
	logrus.Debugf("error chain: %s", errorChain(err))
	return ue.code
}

// block renders the error as a title line followed by indented detail and
// Fix lines.
func (e *userError) block(r *reporter) string {
	var b strings.Builder
	b.WriteString(r.line(kindError, e.title) + "\n")
	for _, d := range e.details {
		b.WriteString("  " + d + "\n")
	}
	if e.hint != "" {
		b.WriteString("  Fix: " + e.hint + "\n")
	}
	return b.String()
}

// oneLine renders the error on a single line.
func (e *userError) oneLine() string {
	if len(e.details) == 0 && e.hint == "" {
		return e.title
	}
	parts := append([]string{e.title + "."}, e.details...)
	if e.hint != "" {
		parts = append(parts, "Fix: "+e.hint)
	}
	return strings.Join(parts, " ")
}

// errorChain returns every message in err's chain, outermost first.
func errorChain(err error) string {
	var msgs []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, " <- ")
}
