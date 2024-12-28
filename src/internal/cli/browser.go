package cli

import (
	"fmt"
	"os/exec"
)

// openBrowser tries to open a browser using `xdg-open`, `open`, or `start`.
func openBrowser(targetURL string) {
	// For Linux
	if exec.Command("xdg-open", targetURL).Start() == nil {
		return
	}
	// For macOS
	if exec.Command("open", targetURL).Start() == nil {
		return
	}
	// For Windows
	if exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL).Start() == nil {
		return
	}
	fmt.Printf("Please open your browser and navigate to: %s\n", targetURL)
}
