package cli

import (
	"fmt"
)

// printFunc defaults to fmt.Printf; tests replace it to capture output.
var printFunc = fmt.Printf

// openBrowser tries to open a browser using `xdg-open`, `open`, or `start`.
func openBrowser(targetURL string) {
	// For Linux
	if execCommand("xdg-open", targetURL).Start() == nil {
		return
	}
	// For macOS
	if execCommand("open", targetURL).Start() == nil {
		return
	}
	// For Windows
	if execCommand("rundll32", "url.dll,FileProtocolHandler", targetURL).Start() == nil {
		return
	}
	printFunc("Please open your browser and navigate to: %s\n", targetURL)
}
