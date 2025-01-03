package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestOpenBrowser(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		output  string
		wantErr bool
	}{
		{
			name:    "Valid command - xdg-open",
			command: "xdg-open",
			args:    []string{"https://example.com"},
			output:  "",
			wantErr: false,
		},
		{
			name:    "Valid command - open",
			command: "open",
			args:    []string{"https://example.com"},
			output:  "",
			wantErr: false,
		},
		{
			name:    "Valid command - rundll32",
			command: "rundll32",
			args:    []string{"url.dll,FileProtocolHandler", "https://example.com"},
			output:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock exec.Command
			execCommand = func(name string, arg ...string) *exec.Cmd {
				if tt.wantErr || name != tt.command || !strings.EqualFold(strings.Join(arg, " "), strings.Join(tt.args, " ")) {
					return exec.Command("false")
				}
				return exec.Command("true")
			}
			defer func() { execCommand = exec.Command }()

			// Capture stdout
			var stdout bytes.Buffer
			printFunc = func(format string, a ...any) (n int, err error) {
				return stdout.WriteString(fmt.Sprintf(format, a...))
			}
			defer func() { printFunc = fmt.Printf }()

			openBrowser("https://example.com")

			// Check output
			if tt.output != "" && stdout.String() != tt.output {
				t.Errorf("expected output %q, got %q", tt.output, stdout.String())
			}
		})
	}
}
