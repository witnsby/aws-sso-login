package release

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// errUsage marks command-line mistakes; Main exits with status 2 for them.
var errUsage = errors.New("usage error")

const usage = `usage: release <command> [flags]

Commands:
  next-version [--rev REV]
      Print "previous=<tag>" and "next=<tag>" for the commits since the last
      vX.Y.Z tag reachable from REV (default HEAD). "next" is empty when no
      commit calls for a release.

  changelog --version vX.Y.Z [--date YYYY-MM-DD] [--file CHANGELOG.md] [--rev REV]
      Move the [Unreleased] section of the changelog into a new section for
      the version (date defaults to today in UTC) and update the compare
      links. An empty [Unreleased] section is filled from the commits since
      the previous tag.

  notes --version vX.Y.Z [--rev REV] [--out FILE]
      Render the release description for the commits and pull requests
      since the previous tag: breaking changes, features, fixes,
      improvements, other pull requests, closed issues and a compare link.
      Writes to FILE, or to stdout without --out. Needs an authenticated gh.
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
	case "changelog":
		return runChangelog(ctx, args[1:], stderr, src)
	case "notes":
		return runNotes(ctx, args[1:], stdout, stderr, src)
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

// releaseRange returns the previous tag (ignoring version itself) and the
// commits between it and rev.
func releaseRange(ctx context.Context, src Source, rev, version string) (string, []Commit, error) {
	prevTag, err := src.PreviousTag(ctx, rev, version)
	if err != nil {
		return "", nil, err
	}
	commits, err := src.Commits(ctx, prevTag, rev)
	if err != nil {
		return "", nil, err
	}
	return prevTag, commits, nil
}

func requireVersion(version string) error {
	if version == "" {
		return fmt.Errorf("%w: --version is required", errUsage)
	}
	if _, err := ParseVersion(version); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	return nil
}

func runChangelog(ctx context.Context, args []string, stderr io.Writer, src Source) error {
	fs := newFlagSet("changelog", stderr)
	version := fs.String("version", "", "version being released, e.g. v0.4.0")
	date := fs.String("date", time.Now().UTC().Format(time.DateOnly), "release date (YYYY-MM-DD)")
	file := fs.String("file", "CHANGELOG.md", "changelog to update in place")
	rev := fs.String("rev", "HEAD", "last commit of the release")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireVersion(*version); err != nil {
		return err
	}

	prevTag, commits, err := releaseRange(ctx, src, *rev, *version)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read changelog: %w", err)
	}
	updated, err := PromoteChangelog(string(content), PromoteOptions{
		Version:  *version,
		Previous: prevTag,
		Date:     *date,
		RepoURL:  src.RepoURL(),
		Commits:  commits,
	})
	if err != nil {
		return fmt.Errorf("promote %s: %w", *file, err)
	}
	if err := os.WriteFile(*file, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write changelog: %w", err)
	}
	fmt.Fprintf(stderr, "Added the %s section to %s.\n", *version, *file)
	return nil
}

func runNotes(ctx context.Context, args []string, stdout, stderr io.Writer, src Source) error {
	fs := newFlagSet("notes", stderr)
	version := fs.String("version", "", "version being released, e.g. v0.4.0")
	rev := fs.String("rev", "HEAD", "last commit of the release")
	out := fs.String("out", "", "file to write the notes to (default stdout)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := requireVersion(*version); err != nil {
		return err
	}

	prevTag, commits, err := releaseRange(ctx, src, *rev, *version)
	if err != nil {
		return err
	}
	prs, err := src.PullRequests(ctx, prevTag, *rev, commits)
	if err != nil {
		return err
	}
	notes := RenderNotes(NotesInput{
		Version:  *version,
		Previous: prevTag,
		RepoURL:  src.RepoURL(),
		Commits:  commits,
		PRs:      prs,
	})
	if *out == "" {
		_, err := io.WriteString(stdout, notes)
		return err
	}
	if err := os.WriteFile(*out, []byte(notes), 0o644); err != nil {
		return fmt.Errorf("write release notes: %w", err)
	}
	fmt.Fprintf(stderr, "Release notes for %s written to %s (%d commits, %d pull requests since %s).\n",
		*version, *out, len(commits), len(prs), describeTag(prevTag))
	return nil
}
