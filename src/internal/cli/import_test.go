package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// recordingSelector wraps fakeSelector with a call counter so tests can assert
// whether the selector was actually invoked.
type recordingSelector struct {
	inner  fakeSelector
	calls  int
	lastIn []profiles.Profile
}

func (r *recordingSelector) Pick(p []profiles.Profile) (string, error) {
	r.calls++
	r.lastIn = p
	return r.inner.Pick(p)
}

// swapDefaultSelector replaces defaultSelector for the duration of the test
// and registers a cleanup hook to restore the original value.
func swapDefaultSelector(t *testing.T, s Selector) {
	t.Helper()
	orig := defaultSelector
	t.Cleanup(func() { defaultSelector = orig })
	defaultSelector = s
}

// writeAwsConfig writes the given content to a temp file and points
// AWS_CONFIG_FILE at it for the duration of the test.
func writeAwsConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("AWS_CONFIG_FILE", path)
	return path
}

// TestResolveProfileName_FlagSet verifies that providing --profile bypasses
// the selector entirely (no fixture, no selector invocation needed).
func TestResolveProfileName_FlagSet(t *testing.T) {
	rec := &recordingSelector{inner: fakeSelector{choice: "should-not-be-used"}}
	swapDefaultSelector(t, rec)

	got, err := resolveProfileName("explicit-profile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "explicit-profile" {
		t.Fatalf("expected %q, got %q", "explicit-profile", got)
	}
	if rec.calls != 0 {
		t.Fatalf("expected selector not to be called when flag is set, got %d calls", rec.calls)
	}
}

// TestResolveProfileName_FromPicker verifies that an empty flag triggers the
// selector against the SSO profiles parsed from AWS_CONFIG_FILE.
func TestResolveProfileName_FromPicker(t *testing.T) {
	cfg := `[profile hipaa-master]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess

[profile sandbox]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = ReadOnly
`
	writeAwsConfig(t, cfg)

	rec := &recordingSelector{inner: fakeSelector{choice: "hipaa-master"}}
	swapDefaultSelector(t, rec)

	got, err := resolveProfileName("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hipaa-master" {
		t.Fatalf("expected %q, got %q", "hipaa-master", got)
	}
	if rec.calls != 1 {
		t.Fatalf("expected selector to be called once, got %d", rec.calls)
	}
	if len(rec.lastIn) != 2 {
		t.Fatalf("expected selector to receive 2 profiles, got %d", len(rec.lastIn))
	}
	names := []string{rec.lastIn[0].Name, rec.lastIn[1].Name}
	if !contains(names, "hipaa-master") || !contains(names, "sandbox") {
		t.Fatalf("expected profiles [hipaa-master sandbox], got %v", names)
	}
}

// TestResolveProfileName_NoSSOProfiles verifies that a config with no SSO
// profiles surfaces a descriptive error and never invokes the selector.
func TestResolveProfileName_NoSSOProfiles(t *testing.T) {
	cfg := `[profile static]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secret

[profile credprocess]
credential_process = /bin/true
`
	writeAwsConfig(t, cfg)

	rec := &recordingSelector{inner: fakeSelector{choice: "should-not-be-used"}}
	swapDefaultSelector(t, rec)

	_, err := resolveProfileName("")
	if err == nil {
		t.Fatal("expected error when no SSO profiles are present, got nil")
	}
	if !strings.Contains(err.Error(), "could not list SSO profiles") {
		t.Fatalf("expected wrapped error with 'could not list SSO profiles', got %v", err)
	}
	if rec.calls != 0 {
		t.Fatalf("expected selector NOT to be called when no SSO profiles exist, got %d calls", rec.calls)
	}
}

// TestResolveProfileName_SelectorError verifies that errors from the selector
// (e.g. user cancelled) are surfaced with descriptive context.
func TestResolveProfileName_SelectorError(t *testing.T) {
	cfg := `[profile hipaa-master]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = AdministratorAccess
`
	writeAwsConfig(t, cfg)

	sentinel := errors.New("user cancelled")
	swapDefaultSelector(t, fakeSelector{err: sentinel})

	_, err := resolveProfileName("")
	if err == nil {
		t.Fatal("expected error from selector, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel error, got %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
