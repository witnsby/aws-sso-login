package cli

import (
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/mattn/go-isatty"
)

// Package-level seams for external effects. Production code calls these
// variables instead of the underlying functions so tests can swap in fakes
// (see AGENTS.md, Testability).
var (
	// execCommand builds external commands (AWS CLI, browser launchers).
	execCommand = exec.Command
	// lookPath resolves executables in PATH.
	lookPath = exec.LookPath
	// now returns the current time; used for credential expiry checks.
	now = time.Now
	// httpClient performs the federation sign-in request.
	httpClient = &http.Client{Timeout: 15 * time.Second}
	// federationEndpoint is the AWS federation sign-in endpoint.
	federationEndpoint = "https://signin.aws.amazon.com/federation"
	// isInteractive reports whether the interactive picker may be shown.
	isInteractive = func() bool {
		return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
	}
	// goos is the operating system the binary runs on.
	goos = runtime.GOOS
)
