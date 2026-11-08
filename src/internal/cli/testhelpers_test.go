package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = f.command
	return f
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
