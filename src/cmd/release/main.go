// Command release is the helper behind the release automation workflows. It
// computes the next version from Conventional Commits, promotes CHANGELOG.md
// and renders release notes. Run "go run ./src/cmd/release help" for usage.
package main

import (
	"context"
	"os"

	"github.com/witnsby/aws-sso-login/src/internal/release"
)

func main() {
	src := release.NewSource(release.ExecRunner)
	os.Exit(release.Main(context.Background(), os.Args[1:], os.Stdout, os.Stderr, src))
}
