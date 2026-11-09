package release

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata")

const repoURL = "https://github.com/witnsby/aws-sso-login"

// golden compares got with testdata/name, rewriting it with -update.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

const curatedChangelog = `# Changelog

## [Unreleased]

### Fixed

- ` + "`import`" + ` keeps comments in the credentials file.

## [v0.3.0] - 2026-11-09

### Added

- Picker filter.

[Unreleased]: https://github.com/witnsby/aws-sso-login/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/witnsby/aws-sso-login/compare/v0.2.1...v0.3.0
`

func TestPromoteChangelogKeepsCuratedEntries(t *testing.T) {
	got, err := PromoteChangelog(curatedChangelog, PromoteOptions{
		Version:  "v0.3.1",
		Previous: "v0.3.0",
		Date:     "2026-11-10",
		RepoURL:  repoURL,
		Commits:  commitsOf("feat: ignored because curated entries exist"),
	})
	require.NoError(t, err)
	golden(t, "changelog_curated_v0.3.1.golden", got)
}

func TestPromoteChangelogGeneratesEntries(t *testing.T) {
	in := "# Changelog\n\n## [Unreleased]\n\n## [v0.3.0] - 2026-11-09\n\n- Old.\n\n" +
		"[Unreleased]: " + repoURL + "/compare/v0.3.0...HEAD\n"
	got, err := PromoteChangelog(in, PromoteOptions{
		Version:  "v0.4.0",
		Previous: "v0.3.0",
		Date:     "2026-11-10",
		RepoURL:  repoURL,
		Commits: []Commit{
			ParseCommit("1", "feat: add --region", ""),
			ParseCommit("2", "fix(cli): handle empty cache", ""),
			ParseCommit("3", "feat(cli)!: rename --profile", ""),
			ParseCommit("4", "perf: cache profile list", ""),
			ParseCommit("5", "docs: not listed", ""),
			ParseCommit("6", "Update README", ""),
		},
	})
	require.NoError(t, err)
	golden(t, "changelog_generated_v0.4.0.golden", got)
}

func TestPromoteChangelogFirstRelease(t *testing.T) {
	in := "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- Everything.\n"
	got, err := PromoteChangelog(in, PromoteOptions{
		Version: "v0.1.0",
		Date:    "2026-11-10",
		RepoURL: repoURL,
	})
	require.NoError(t, err)
	golden(t, "changelog_first_v0.1.0.golden", got)
}

func TestPromoteCurrentChangelog(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "changelog_current.md"))
	require.NoError(t, err)
	got, err := PromoteChangelog(string(in), PromoteOptions{
		Version:  "v0.3.1",
		Previous: "v0.3.0",
		Date:     "2026-11-10",
		RepoURL:  repoURL,
		Commits:  commitsOf("fix: handle empty cache", "docs: update README"),
	})
	require.NoError(t, err)
	golden(t, "changelog_current_v0.3.1.golden", got)
}

func TestPromoteChangelogErrors(t *testing.T) {
	base := PromoteOptions{Version: "v0.3.1", Previous: "v0.3.0", Date: "2026-11-10", RepoURL: repoURL}

	_, err := PromoteChangelog("# Changelog\n\n## [v0.3.0] - x\n", base)
	assert.ErrorContains(t, err, "no \"## [Unreleased]\" section")

	_, err = PromoteChangelog("# Changelog\n\n## [Unreleased]\n\n## [v0.3.0] - x\n", base)
	assert.ErrorContains(t, err, "[Unreleased] is empty and no commit is releasable")

	_, err = PromoteChangelog("# Changelog\n\n## [Unreleased]\n\n- x\n\n## [v0.3.1] - 2026-11-10\n", base)
	assert.ErrorContains(t, err, "already has a section for v0.3.1")

	bad := base
	bad.Date = "10.11.2026"
	_, err = PromoteChangelog(curatedChangelog, bad)
	assert.ErrorContains(t, err, "invalid release date")

	bad = base
	bad.Version = "0.3.1"
	_, err = PromoteChangelog(curatedChangelog, bad)
	assert.ErrorContains(t, err, "invalid version")
}

func TestChangelogCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	require.NoError(t, os.WriteFile(path, []byte(curatedChangelog), 0o644))
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.2.1\nv0.3.0\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("aaa|fix: keep comments|"))

	code, stdout, stderr := runMain(t, f.source(), "changelog", "--version", "v0.3.1", "--date", "2026-11-10", "--file", path)
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stdout)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	golden(t, "changelog_curated_v0.3.1.golden", string(got))
}

func TestChangelogCommandIgnoresTagOfReleasedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	require.NoError(t, os.WriteFile(path, []byte(curatedChangelog), 0o644))
	// A rerun after the tag exists must still compare against v0.3.0.
	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.3.0\nv0.3.1\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("aaa|fix: keep comments|"))
	code, _, stderr := runMain(t, f.source(), "changelog", "--version", "v0.3.1", "--date", "2026-11-10", "--file", path)
	assert.Equal(t, 0, code, stderr)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(got), "[v0.3.1]: "+repoURL+"/compare/v0.3.0...v0.3.1")
}

func TestChangelogCommandErrors(t *testing.T) {
	src := newFakeRunner(t).source()
	code, _, stderr := runMain(t, src, "changelog")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--version is required")

	code, _, _ = runMain(t, src, "changelog", "--version", "1.0")
	assert.Equal(t, 2, code)

	f := newFakeRunner(t).
		on("git tag --merged HEAD --list v*", "v0.3.0\n").
		on(logFormat+" v0.3.0..HEAD", gitLog("aaa|fix: x|"))
	code, _, stderr = runMain(t, f.source(), "changelog", "--version", "v0.3.1", "--file", filepath.Join(t.TempDir(), "missing.md"))
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "read changelog")
}
