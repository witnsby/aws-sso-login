package release

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviousTag(t *testing.T) {
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.1.0\nv0.10.0\nv0.9.1\nv1.0.0-rc.1\nvnext\n")
	tag, err := f.source().PreviousTag(context.Background(), "HEAD", "")
	require.NoError(t, err)
	assert.Equal(t, "v0.10.0", tag, "highest semver tag wins, invalid tags are ignored")
}

func TestPreviousTagExcludesReleasedVersion(t *testing.T) {
	f := newFakeRunner(t).on("git tag --merged v0.3.0 --list v*", "v0.2.1\nv0.3.0\n")
	tag, err := f.source().PreviousTag(context.Background(), "v0.3.0", "v0.3.0")
	require.NoError(t, err)
	assert.Equal(t, "v0.2.1", tag)
}

func TestPreviousTagWithoutTags(t *testing.T) {
	f := newFakeRunner(t).on("git tag --merged HEAD --list v*", "")
	tag, err := f.source().PreviousTag(context.Background(), "HEAD", "")
	require.NoError(t, err)
	assert.Empty(t, tag)
}

func TestPreviousTagError(t *testing.T) {
	f := newFakeRunner(t).fail("git tag --merged nope --list v*", errors.New("bad revision"))
	_, err := f.source().PreviousTag(context.Background(), "nope", "")
	assert.ErrorContains(t, err, "bad revision")
}

func TestCommits(t *testing.T) {
	f := newFakeRunner(t).on(logFormat+" v0.3.0..HEAD", gitLog(
		"aaa|feat(cli): add --region|",
		"bbb|fix: exit codes|Details.\n\nBREAKING CHANGE: changed",
		"ccc|Update README|",
	))
	commits, err := f.source().Commits(context.Background(), "v0.3.0", "HEAD")
	require.NoError(t, err)
	require.Len(t, commits, 3)
	assert.Equal(t, "aaa", commits[0].Hash)
	assert.Equal(t, "cli", commits[0].Scope)
	assert.True(t, commits[1].Breaking)
	assert.Equal(t, "Details.\n\nBREAKING CHANGE: changed", commits[1].Body)
	assert.False(t, commits[2].Conventional)
}

func TestCommitsWithoutPreviousTag(t *testing.T) {
	f := newFakeRunner(t).on(logFormat+" HEAD", gitLog("aaa|feat: initial|"))
	commits, err := f.source().Commits(context.Background(), "", "HEAD")
	require.NoError(t, err)
	require.Len(t, commits, 1)
}

func TestNewSourceFromEnvironment(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_SERVER_URL", "")
	assert.Equal(t, "https://github.com/witnsby/aws-sso-login", NewSource(nil).RepoURL())

	t.Setenv("GITHUB_REPOSITORY", "octo/tool")
	t.Setenv("GITHUB_SERVER_URL", "https://ghe.example.com/")
	assert.Equal(t, "https://ghe.example.com/octo/tool", NewSource(nil).RepoURL())
}

func runMain(t *testing.T, src Source, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), args, &stdout, &stderr, src)
	return code, stdout.String(), stderr.String()
}

func TestNextVersionCommand(t *testing.T) {
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.2.1\nv0.3.0\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("aaa1111|fix: handle empty cache|", "bbb2222|feat(cli): add --region|"))
	code, stdout, stderr := runMain(t, f.source(), "next-version")
	assert.Equal(t, 0, code)
	assert.Equal(t, "previous=v0.3.0\nnext=v0.4.0\n", stdout)
	assert.Contains(t, stderr, "minor bbb2222 feat(cli): add --region")
	assert.Contains(t, stderr, "Next version: v0.4.0")
}

func TestNextVersionCommandNothingToRelease(t *testing.T) {
	f := newFakeRunner(t).
		on("git tag --merged main --list v*", "v0.3.0\n").
		on(logFormat+" v0.3.0..main", gitLog("aaa|docs(openspec): archive change|", "bbb|chore(deps): bump x|"))
	code, stdout, stderr := runMain(t, f.source(), "next-version", "--rev", "main")
	assert.Equal(t, 0, code)
	assert.Equal(t, "previous=v0.3.0\nnext=\n", stdout)
	assert.Contains(t, stderr, "nothing to release")
}

func TestNextVersionCommandWithoutTags(t *testing.T) {
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "").
		on(logFormat+" HEAD", gitLog("aaa|feat: initial|"))
	code, stdout, _ := runMain(t, f.source(), "next-version")
	assert.Equal(t, 0, code)
	assert.Equal(t, "previous=\nnext=v0.1.0\n", stdout)
}

func TestNextVersionCommandGitFailure(t *testing.T) {
	f := newFakeRunner(t).fail("git tag --merged HEAD --list v*", errors.New("not a git repository"))
	code, stdout, stderr := runMain(t, f.source(), "next-version")
	assert.Equal(t, 1, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "not a git repository")
}

func TestMainUsageErrors(t *testing.T) {
	src := newFakeRunner(t).source()
	for _, args := range [][]string{nil, {"bogus"}, {"next-version", "--nope"}, {"next-version", "extra"}} {
		code, _, stderr := runMain(t, src, args...)
		assert.Equal(t, 2, code, args)
		assert.Contains(t, stderr, "usage: release", args)
	}
	code, stdout, _ := runMain(t, src, "help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "next-version")
}
