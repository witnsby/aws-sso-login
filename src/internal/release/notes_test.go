package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prQueryCmd(number int) string {
	return "gh api graphql -f query=" + pullRequestQuery +
		" -f owner=witnsby -f name=aws-sso-login -F number=" + fmt.Sprint(number)
}

func prJSON(number int, title string, oids []string, issues ...Issue) string {
	var commits, closes []string
	for _, oid := range oids {
		commits = append(commits, fmt.Sprintf(`{"commit":{"oid":%q}}`, oid))
	}
	for _, i := range issues {
		closes = append(closes, fmt.Sprintf(`{"number":%d,"title":%q,"url":%q}`, i.Number, i.Title, i.URL))
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"number":%d,"title":%q,"url":"%s/pull/%d",`+
		`"commits":{"nodes":[%s]},"closingIssuesReferences":{"nodes":[%s]}}}}}`,
		number, title, repoURL, number, strings.Join(commits, ","), strings.Join(closes, ","))
}

func issue(n int, title string) Issue {
	return Issue{Number: n, Title: title, URL: fmt.Sprintf("%s/issues/%d", repoURL, n)}
}

func TestPullRequestNumbers(t *testing.T) {
	f := newFakeRunner(t).on("git log --merges --format=%s v0.3.0..HEAD",
		"Merge pull request #52 from witnsby/docs\nMerge remote-tracking branch 'origin/main' into x\nMerge pull request #50 from witnsby/picker\n")
	commits := commitsOf("fix: squashed change (#55)", "feat: plain")
	numbers, err := f.source().PullRequestNumbers(context.Background(), "v0.3.0", "HEAD", commits)
	require.NoError(t, err)
	assert.Equal(t, []int{50, 52, 55}, numbers)
}

func TestPullRequest(t *testing.T) {
	f := newFakeRunner(t).on(prQueryCmd(46), prJSON(46, "fix: release hardening", []string{"aaa", "bbb"}, issue(42, "Pin actions"), issue(43, "Notes")))
	pr, err := f.source().PullRequest(context.Background(), 46)
	require.NoError(t, err)
	assert.Equal(t, 46, pr.Number)
	assert.Equal(t, "fix: release hardening", pr.Title)
	assert.Equal(t, repoURL+"/pull/46", pr.URL)
	assert.Equal(t, []string{"aaa", "bbb"}, pr.Commits)
	assert.Equal(t, []Issue{issue(42, "Pin actions"), issue(43, "Notes")}, pr.Closes)
}

func TestPullRequestErrors(t *testing.T) {
	f := newFakeRunner(t).
		on(prQueryCmd(1), `{"data":{"repository":{"pullRequest":null}}}`).
		on(prQueryCmd(2), `not json`).
		fail(prQueryCmd(3), errors.New("HTTP 401"))
	_, err := f.source().PullRequest(context.Background(), 1)
	assert.ErrorContains(t, err, "pull request #1 not found")
	_, err = f.source().PullRequest(context.Background(), 2)
	assert.ErrorContains(t, err, "decode pull request #2")
	_, err = f.source().PullRequest(context.Background(), 3)
	assert.ErrorContains(t, err, "HTTP 401")

	bad := f.source()
	bad.Repo = "no-slash"
	_, err = bad.PullRequest(context.Background(), 1)
	assert.ErrorContains(t, err, "invalid repository")
}

// notesFixture covers every section: a breaking change, features from a
// merged pull request, a fix pushed without a pull request, a squash-merged
// perf commit, a documentation-only pull request and an issue closed by two
// pull requests.
func notesFixture() NotesInput {
	commits := []Commit{
		ParseCommit("c1aaaaaaaaaa", "feat(cli)!: render errors as blocks", ""),
		ParseCommit("c2bbbbbbbbbb", "feat(cli): filter the profile picker as you type", ""),
		ParseCommit("c3cccccccccc", "test(cli): cover the picker", ""),
		ParseCommit("c4dddddddddd", "fix: handle empty cache", ""),
		ParseCommit("c5eeeeeeeeee", "perf: cache profile list (#53)", ""),
		ParseCommit("c6ffffffffff", "docs(openspec): archive change", ""),
		ParseCommit("c7000000000a", "revert: undo picker height", ""),
	}
	prs := []PullRequest{
		{Number: 49, Title: "feat(cli)!: unified status output", URL: repoURL + "/pull/49", Commits: []string{"c1aaaaaaaaaa"}, Closes: []Issue{issue(34, "Harden pipeline")}},
		{Number: 50, Title: "feat: type-to-filter picker", URL: repoURL + "/pull/50", Commits: []string{"c2bbbbbbbbbb", "c3cccccccccc", "c7000000000a"}, Closes: []Issue{issue(34, "Harden pipeline"), issue(12, "Long profile lists")}},
		{Number: 52, Title: "docs(openspec): archive v0.3.0 changes", URL: repoURL + "/pull/52", Commits: []string{"c6ffffffffff"}},
		{Number: 53, Title: "perf: cache profile list", URL: repoURL + "/pull/53", Commits: []string{"squash-branch-commit"}},
	}
	return NotesInput{Version: "v0.4.0", Previous: "v0.3.0", RepoURL: repoURL, Commits: commits, PRs: prs}
}

func TestRenderNotes(t *testing.T) {
	golden(t, "notes_v0.4.0.golden", RenderNotes(notesFixture()))
}

func TestRenderNotesFirstReleaseHasNoCompareLink(t *testing.T) {
	in := NotesInput{Version: "v0.1.0", RepoURL: repoURL, Commits: commitsOf("feat: initial")}
	in.Commits[0].Hash = "0123456789abcdef"
	got := RenderNotes(in)
	assert.Equal(t, "## Features\n\n- initial ([0123456](https://github.com/witnsby/aws-sso-login/commit/0123456789abcdef))\n", got)
}

func TestRenderNotesWithoutChanges(t *testing.T) {
	got := RenderNotes(NotesInput{Version: "v0.3.1", Previous: "v0.3.0", RepoURL: repoURL, Commits: commitsOf("Update README")})
	assert.Equal(t, "No user-facing changes.\n\n**Full changelog**: [v0.3.0...v0.3.1](https://github.com/witnsby/aws-sso-login/compare/v0.3.0...v0.3.1)\n", got)
}

func TestNotesCommand(t *testing.T) {
	out := filepath.Join(t.TempDir(), "notes.md")
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.3.0\nv0.4.0\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("c2bbbbbbbbbb|feat(cli): filter the profile picker as you type|", "c9|fix: direct push|")).
		on("git log --merges --format=%s v0.3.0..HEAD", "Merge pull request #50 from witnsby/picker\n").
		on(prQueryCmd(50), prJSON(50, "feat: type-to-filter picker", []string{"c2bbbbbbbbbb"}, issue(12, "Long profile lists")))

	code, stdout, stderr := runMain(t, f.source(), "notes", "--version", "v0.4.0", "--out", out)
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "1 pull requests since v0.3.0")
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, `## Features

- **cli:** filter the profile picker as you type ([#50](https://github.com/witnsby/aws-sso-login/pull/50))

## Fixes

- direct push ([c9](https://github.com/witnsby/aws-sso-login/commit/c9))

## Closed issues

- [#12](https://github.com/witnsby/aws-sso-login/issues/12) Long profile lists

**Full changelog**: [v0.3.0...v0.4.0](https://github.com/witnsby/aws-sso-login/compare/v0.3.0...v0.4.0)
`, string(got))
}

func TestNotesCommandToStdout(t *testing.T) {
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "").
		on(logFormat+" HEAD", gitLog("abc|feat: initial|")).
		on("git log --merges --format=%s HEAD", "")
	code, stdout, stderr := runMain(t, f.source(), "notes", "--version", "v0.1.0")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "## Features\n\n- initial")
}

func TestNotesCommandErrors(t *testing.T) {
	src := newFakeRunner(t).source()
	code, _, stderr := runMain(t, src, "notes")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--version is required")

	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.3.0\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("abc|feat: x|")).
		on("git log --merges --format=%s v0.3.0..HEAD", "Merge pull request #7 from a/b\n").
		fail(prQueryCmd(7), errors.New("gh: authentication required"))
	code, _, stderr = runMain(t, f.source(), "notes", "--version", "v0.4.0")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "authentication required")
}
