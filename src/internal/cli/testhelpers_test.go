package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// Fake credential values used across tests. They are placeholders, not real
// AWS credentials.
const (
	fakeAccessKey    = "ASIAFAKEACCESSKEY000"
	fakeSecretKey    = "fakeSecretKeyValue0000000000000000000000"
	fakeSessionToken = "fakeSessionTokenValue000000000000"
)

// awsEnv describes the temporary AWS environment created by setupAWSEnv.
type awsEnv struct {
	Home            string
	ConfigPath      string
	CredentialsPath string
}

// setupAWSEnv points HOME, AWS_CONFIG_FILE and AWS_SHARED_CREDENTIALS_FILE
// at a temp dir, clears AWS_PROFILE and writes config as the AWS config file.
// Tests using it must not run in parallel.
func setupAWSEnv(t *testing.T, config string) awsEnv {
	t.Helper()
	home := t.TempDir()
	env := awsEnv{
		Home:            home,
		ConfigPath:      filepath.Join(home, ".aws", "config"),
		CredentialsPath: filepath.Join(home, ".aws", "credentials"),
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(env.ConfigPath), 0o700))
	require.NoError(t, os.WriteFile(env.ConfigPath, []byte(config), 0o600))
	t.Setenv("HOME", home)
	t.Setenv("AWS_CONFIG_FILE", env.ConfigPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", env.CredentialsPath)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_SSO_LOGIN_DEBUG", "")
	return env
}

// writeCachedCreds writes an AWS CLI cache file for profile with the given
// expiration, using the same cache key the production code computes.
func writeCachedCreds(t *testing.T, profile profiles.Profile, expiration time.Time) {
	t.Helper()
	path, err := buildCacheFilePath(profile)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	payload := map[string]any{
		"ProviderType": "sso",
		"Credentials": model.RoleCredential{
			AccessKeyId:     fakeAccessKey,
			SecretAccessKey: fakeSecretKey,
			SessionToken:    fakeSessionToken,
			Expiration:      expiration.UTC().Format(time.RFC3339),
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o600))
}

// mustResolve resolves a profile from the current AWS_CONFIG_FILE.
func mustResolve(t *testing.T, name string) profiles.Profile {
	t.Helper()
	p, err := resolveProfile(name)
	require.NoError(t, err)
	return p
}

// legacyConfig is a config with an SSO-enabled [default] and a legacy profile.
const legacyConfig = `[default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = DefaultRole

[profile dev]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = Admin
region = eu-west-1
`

// fakeResult scripts the outcome of one faked external command.
type fakeResult struct {
	Stdout string
	Stderr string
	Exit   int
	// NotFound makes the command fail as if the executable did not exist.
	NotFound bool
	// Before runs in the test process before the command is returned, e.g.
	// to simulate the AWS CLI writing a fresh cache file.
	Before func()
}

// fakeCall records one invocation of execCommand.
type fakeCall struct {
	Name string
	Args []string
	Cmd  *exec.Cmd
}

// Argv returns the command name followed by its arguments, space separated.
func (c fakeCall) Argv() string {
	return strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
}

// fakeRunner replaces execCommand and records every call. Each call is
// answered by handler, which returns the scripted result.
type fakeRunner struct {
	mu      sync.Mutex
	calls   []fakeCall
	handler func(name string, args []string) fakeResult
}

// installFakeRunner swaps execCommand for a fake for the duration of the test.
func installFakeRunner(t *testing.T, handler func(name string, args []string) fakeResult) *fakeRunner {
	t.Helper()
	f := &fakeRunner{handler: handler}
	origExec, origLook := execCommand, lookPath
	t.Cleanup(func() { execCommand, lookPath = origExec, origLook })
	execCommand = f.command
	// Every executable is "installed" unless a test says otherwise.
	lookPath = func(name string) (string, error) { return "/fake/bin/" + name, nil }
	return f
}

// missingExecutables makes lookPath fail for the given names.
func missingExecutables(t *testing.T, names ...string) {
	t.Helper()
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(name string) (string, error) {
		for _, n := range names {
			if n == name {
				return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
			}
		}
		return orig(name)
	}
}

func (f *fakeRunner) command(name string, args ...string) *exec.Cmd {
	res := fakeResult{}
	if f.handler != nil {
		res = f.handler(name, args)
	}
	if res.Before != nil {
		res.Before()
	}

	var cmd *exec.Cmd
	if res.NotFound {
		cmd = exec.Command("aws-sso-login-test-executable-that-does-not-exist")
	} else {
		cmd = exec.Command(os.Args[0], "-test.run=^TestHelperProcess$", "--")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			"HELPER_STDOUT="+res.Stdout,
			"HELPER_STDERR="+res.Stderr,
			"HELPER_EXIT="+strconv.Itoa(res.Exit),
		)
	}

	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{Name: name, Args: args, Cmd: cmd})
	f.mu.Unlock()
	return cmd
}

// Calls returns a copy of the recorded calls.
func (f *fakeRunner) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

// CallsMatching returns the recorded calls whose argv starts with prefix.
func (f *fakeRunner) CallsMatching(prefix string) []fakeCall {
	var out []fakeCall
	for _, c := range f.Calls() {
		if strings.HasPrefix(c.Argv(), prefix) {
			out = append(out, c)
		}
	}
	return out
}

// TestHelperProcess is not a real test: it is the child process started by
// fakeRunner. It prints the scripted output and exits with the scripted code.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR"))
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

func TestFakeRunner(t *testing.T) {
	f := installFakeRunner(t, func(name string, args []string) fakeResult {
		switch {
		case name == "ok":
			return fakeResult{Stdout: "hello", Stderr: "warn"}
		case name == "missing":
			return fakeResult{NotFound: true}
		default:
			return fakeResult{Stderr: "boom", Exit: 3}
		}
	})

	var out, errOut strings.Builder
	cmd := execCommand("ok", "a", "b")
	cmd.Stdout, cmd.Stderr = &out, &errOut
	require.NoError(t, cmd.Run())
	assert.Equal(t, "hello", out.String())
	assert.Equal(t, "warn", errOut.String())

	err := execCommand("fail").Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 3, exitErr.ExitCode())

	err = execCommand("missing").Run()
	require.ErrorIs(t, err, exec.ErrNotFound)

	calls := f.Calls()
	require.Len(t, calls, 3)
	assert.Equal(t, "ok a b", calls[0].Argv())
	assert.Len(t, f.CallsMatching("ok"), 1)
}
