package release

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

// errUsage marks command-line mistakes; Main exits with status 2 for them.
var errUsage = errors.New("usage error")

const usage = `usage: release <command> [flags]

Commands:
  next-version [--rev REV]
      Print "previous=<tag>" and "next=<tag>" for the commits since the last
      vX.Y.Z tag reachable from REV (default HEAD). "next" is empty when no
      commit calls for a release.
`

// Main runs the release helper with args (without the program name) and
// returns the process exit status. Results go to stdout, diagnostics to
// stderr.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer, src Source) int {
	err := run(ctx, args, stdout, stderr, src)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "release: %v\n\n%s", err, usage)
		return 2
	default:
		fmt.Fprintf(stderr, "release: %v\n", err)
		return 1
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, src Source) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: missing command", errUsage)
	}
	switch args[0] {
	case "next-version":
		return runNextVersion(ctx, args[1:], stdout, stderr, src)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		return fmt.Errorf("%w: unknown command %q", errUsage, args[0])
	}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("%w: unexpected argument %q", errUsage, fs.Arg(0))
	}
	return nil
}

func runNextVersion(ctx context.Context, args []string, stdout, stderr io.Writer, src Source) error {
	fs := newFlagSet("next-version", stderr)
	rev := fs.String("rev", "HEAD", "revision to evaluate")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	prevTag, err := src.PreviousTag(ctx, *rev, "")
	if err != nil {
		return err
	}
	prev := Version{}
	if prevTag != "" {
		if prev, err = ParseVersion(prevTag); err != nil {
			return err
		}
	}
	commits, err := src.Commits(ctx, prevTag, *rev)
	if err != nil {
		return err
	}

	for _, c := range commits {
		fmt.Fprintf(stderr, "%-5s %s %s\n", c.Bump(), shortHash(c.Hash), c.Subject)
	}
	next, ok := NextVersion(prev, commits)
	fmt.Fprintf(stdout, "previous=%s\n", prevTag)
	if !ok {
		fmt.Fprintf(stdout, "next=\n")
		fmt.Fprintf(stderr, "No releasable commits since %s: nothing to release.\n", describeTag(prevTag))
		return nil
	}
	fmt.Fprintf(stdout, "next=%s\n", next)
	fmt.Fprintf(stderr, "Next version: %s (%s bump since %s).\n", next, HighestBump(commits), describeTag(prevTag))
	return nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func describeTag(tag string) string {
	if tag == "" {
		return "the first commit"
	}
	return tag
}
