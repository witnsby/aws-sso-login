package cli

import (
	"fmt"
)

// browserCommand returns the command and leading arguments used to open a
// URL on the given OS, or "" when the OS has no known launcher.
func browserCommand(osName string) (string, []string) {
	switch osName {
	case "darwin":
		return "open", nil
	case "linux":
		return "xdg-open", nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler"}
	}
	return "", nil
}

// openBrowser opens targetURL with the launcher for the current OS. When the
// launcher is missing or fails to start, the URL is printed to stderr so the
// user can open it manually. stdout is never written.
func openBrowser(targetURL string) {
	if name, args := browserCommand(goos); name != "" {
		if _, err := lookPath(name); err == nil {
			cmd := execCommand(name, append(args, targetURL)...)
			if err := cmd.Start(); err == nil {
				// Do not wait for the launcher (xdg-open can linger).
				_ = cmd.Process.Release()
				return
			}
		}
	}
	fmt.Fprintf(stderrOut, "Please open this URL in your browser: %s\n", targetURL)
}
