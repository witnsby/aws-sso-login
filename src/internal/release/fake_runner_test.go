package release

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeRunner answers commands from a script keyed by the full command line
// ("git log --no-merges ..."). Unscripted commands fail the test.
type fakeRunner struct {
	t       *testing.T
	outputs map[string]string
	errors  map[string]error
	calls   []string
}

func newFakeRunner(t *testing.T) *fakeRunner {
	return &fakeRunner{t: t, outputs: map[string]string{}, errors: map[string]error{}}
}

func (f *fakeRunner) on(cmdline, output string) *fakeRunner {
	f.outputs[cmdline] = output
	return f
}

func (f *fakeRunner) fail(cmdline string, err error) *fakeRunner {
	f.errors[cmdline] = err
	return f
}

func (f *fakeRunner) run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmdline := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, cmdline)
	if err, ok := f.errors[cmdline]; ok {
		return nil, err
	}
	out, ok := f.outputs[cmdline]
	if !ok {
		f.t.Errorf("unexpected command: %s", cmdline)
		return nil, fmt.Errorf("unexpected command: %s", cmdline)
	}
	return []byte(out), nil
}

func (f *fakeRunner) source() Source {
	return Source{Run: f.run, Repo: "witnsby/aws-sso-login", ServerURL: "https://github.com"}
}

// gitLog formats commits the way "git log --format=%H%x1f%s%x1f%b%x1e"
// prints them. Each entry is "hash|subject|body".
func gitLog(entries ...string) string {
	var b strings.Builder
	for i, e := range entries {
		parts := strings.SplitN(e, "|", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(parts[0] + fieldSep + parts[1] + fieldSep + parts[2] + "\n" + recordSep)
	}
	return b.String()
}

const logFormat = "git log --no-merges --format=%H%x1f%s%x1f%b%x1e"
