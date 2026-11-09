package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner runs an external command and returns its standard output. It is a
// seam so tests can script git and gh without a repository or network.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecRunner runs commands with os/exec. Standard error is included in the
// returned error.
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, msg)
	}
	return out, nil
}

// DefaultRepo is used when GITHUB_REPOSITORY is not set, e.g. locally.
const DefaultRepo = "witnsby/aws-sso-login"

// Source reads release data from git and the GitHub CLI.
type Source struct {
	Run Runner
	// Repo is "owner/name".
	Repo string
	// ServerURL is the GitHub base URL, e.g. "https://github.com".
	ServerURL string
}

// NewSource returns a Source for the repository named by GITHUB_REPOSITORY
// and GITHUB_SERVER_URL, defaulting to DefaultRepo on github.com.
func NewSource(run Runner) Source {
	s := Source{Run: run, Repo: os.Getenv("GITHUB_REPOSITORY"), ServerURL: os.Getenv("GITHUB_SERVER_URL")}
	if s.Repo == "" {
		s.Repo = DefaultRepo
	}
	if s.ServerURL == "" {
		s.ServerURL = "https://github.com"
	}
	return s
}

// RepoURL returns the repository web URL.
func (s Source) RepoURL() string {
	return strings.TrimRight(s.ServerURL, "/") + "/" + s.Repo
}

// PreviousTag returns the highest vX.Y.Z tag reachable from rev, ignoring
// the tag named exclude (the version being released). It returns "" when
// there is none.
func (s Source) PreviousTag(ctx context.Context, rev, exclude string) (string, error) {
	out, err := s.Run(ctx, "git", "tag", "--merged", rev, "--list", "v*")
	if err != nil {
		return "", fmt.Errorf("list tags merged into %s: %w", rev, err)
	}
	var best string
	var bestVersion Version
	for _, tag := range strings.Fields(string(out)) {
		if tag == exclude {
			continue
		}
		v, err := ParseVersion(tag)
		if err != nil {
			continue
		}
		if best == "" || bestVersion.Less(v) {
			best, bestVersion = tag, v
		}
	}
	return best, nil
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// Commits returns the non-merge commits in from..rev, newest first. With an
// empty from, it returns every commit reachable from rev.
func (s Source) Commits(ctx context.Context, from, rev string) ([]Commit, error) {
	out, err := s.Run(ctx, "git", "log", "--no-merges", "--format=%H%x1f%s%x1f%b%x1e", revRange(from, rev))
	if err != nil {
		return nil, fmt.Errorf("read commits %s: %w", revRange(from, rev), err)
	}
	var commits []Commit
	for _, record := range strings.Split(string(out), recordSep) {
		record = strings.TrimLeft(record, "\n")
		if strings.TrimSpace(record) == "" {
			continue
		}
		fields := strings.SplitN(record, fieldSep, 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected git log record %q", record)
		}
		commits = append(commits, ParseCommit(fields[0], fields[1], strings.TrimSpace(fields[2])))
	}
	return commits, nil
}

func revRange(from, rev string) string {
	if from == "" {
		return rev
	}
	return from + ".." + rev
}
