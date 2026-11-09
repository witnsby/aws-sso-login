package release

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
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

// Issue is an issue closed by a pull request.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// PullRequest is a pull request merged in the release range.
type PullRequest struct {
	Number  int
	Title   string
	URL     string
	Commits []string // commit OIDs
	Closes  []Issue
}

var (
	mergeSubject   = regexp.MustCompile(`^Merge pull request #([0-9]+) `)
	squashedSuffix = regexp.MustCompile(`\(#([0-9]+)\)\s*$`)
)

// pullRequestQuery fetches what release notes need about one pull request.
const pullRequestQuery = `query($owner: String!, $name: String!, $number: Int!) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      number
      title
      url
      commits(first: 250) { nodes { commit { oid } } }
      closingIssuesReferences(first: 50) { nodes { number title url } }
    }
  }
}`

type pullRequestResponse struct {
	Data struct {
		Repository struct {
			PullRequest *struct {
				Number  int    `json:"number"`
				Title   string `json:"title"`
				URL     string `json:"url"`
				Commits struct {
					Nodes []struct {
						Commit struct {
							OID string `json:"oid"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
				ClosingIssuesReferences struct {
					Nodes []Issue `json:"nodes"`
				} `json:"closingIssuesReferences"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// PullRequestNumbers returns the numbers of the pull requests merged in
// from..rev, from "Merge pull request #N" merge commits and "(#N)" suffixes
// of squash-merged commits, in ascending order.
func (s Source) PullRequestNumbers(ctx context.Context, from, rev string, commits []Commit) ([]int, error) {
	out, err := s.Run(ctx, "git", "log", "--merges", "--format=%s", revRange(from, rev))
	if err != nil {
		return nil, fmt.Errorf("read merge commits %s: %w", revRange(from, rev), err)
	}
	seen := map[int]bool{}
	add := func(m []string) {
		if m == nil {
			return
		}
		if n, err := strconv.Atoi(m[1]); err == nil {
			seen[n] = true
		}
	}
	for _, subject := range strings.Split(string(out), "\n") {
		add(mergeSubject.FindStringSubmatch(strings.TrimSpace(subject)))
	}
	for _, c := range commits {
		add(squashedSuffix.FindStringSubmatch(c.Subject))
	}
	numbers := make([]int, 0, len(seen))
	for n := range seen {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	return numbers, nil
}

// PullRequest fetches one pull request with its commits and closed issues.
func (s Source) PullRequest(ctx context.Context, number int) (PullRequest, error) {
	owner, name, ok := strings.Cut(s.Repo, "/")
	if !ok {
		return PullRequest{}, fmt.Errorf("invalid repository %q: want owner/name", s.Repo)
	}
	out, err := s.Run(ctx, "gh", "api", "graphql",
		"-f", "query="+pullRequestQuery,
		"-f", "owner="+owner,
		"-f", "name="+name,
		"-F", "number="+strconv.Itoa(number))
	if err != nil {
		return PullRequest{}, fmt.Errorf("fetch pull request #%d: %w", number, err)
	}
	var resp pullRequestResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return PullRequest{}, fmt.Errorf("decode pull request #%d: %w", number, err)
	}
	raw := resp.Data.Repository.PullRequest
	if raw == nil {
		return PullRequest{}, fmt.Errorf("pull request #%d not found in %s", number, s.Repo)
	}
	pr := PullRequest{Number: raw.Number, Title: raw.Title, URL: raw.URL, Closes: raw.ClosingIssuesReferences.Nodes}
	for _, n := range raw.Commits.Nodes {
		pr.Commits = append(pr.Commits, n.Commit.OID)
	}
	return pr, nil
}

// PullRequests fetches every pull request merged in from..rev.
func (s Source) PullRequests(ctx context.Context, from, rev string, commits []Commit) ([]PullRequest, error) {
	numbers, err := s.PullRequestNumbers(ctx, from, rev, commits)
	if err != nil {
		return nil, err
	}
	prs := make([]PullRequest, 0, len(numbers))
	for _, n := range numbers {
		pr, err := s.PullRequest(ctx, n)
		if err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}
