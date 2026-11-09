package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commitsOf(subjects ...string) []Commit {
	out := make([]Commit, 0, len(subjects))
	for _, s := range subjects {
		out = append(out, ParseCommit("", s, ""))
	}
	return out
}

func TestParseVersion(t *testing.T) {
	v, err := ParseVersion("v1.24.3")
	require.NoError(t, err)
	assert.Equal(t, Version{1, 24, 3}, v)
	assert.Equal(t, "v1.24.3", v.String())

	for _, bad := range []string{"1.2.3", "v1.2", "v1.2.3-rc.1", "v01.2.3", "v1.2.3+meta", "latest", ""} {
		_, err := ParseVersion(bad)
		assert.Error(t, err, bad)
	}
}

func TestVersionLess(t *testing.T) {
	assert.True(t, Version{0, 9, 9}.Less(Version{1, 0, 0}))
	assert.True(t, Version{1, 2, 3}.Less(Version{1, 10, 0}))
	assert.True(t, Version{1, 2, 3}.Less(Version{1, 2, 4}))
	assert.False(t, Version{1, 2, 3}.Less(Version{1, 2, 3}))
}

func TestNextVersion(t *testing.T) {
	tests := []struct {
		name    string
		prev    string
		commits []Commit
		want    string
		release bool
	}{
		{
			name:    "feature and fix",
			prev:    "v1.4.2",
			commits: commitsOf("fix: handle empty cache", "feat(cli): add --region"),
			want:    "v1.5.0",
			release: true,
		},
		{
			name:    "breaking change on 1.x",
			prev:    "v1.4.2",
			commits: commitsOf("feat(cli)!: rename --profile"),
			want:    "v2.0.0",
			release: true,
		},
		{
			name:    "breaking footer while major is 0",
			prev:    "v0.3.0",
			commits: []Commit{ParseCommit("", "fix: exit codes", "BREAKING CHANGE: exit codes changed")},
			want:    "v0.4.0",
			release: true,
		},
		{
			name:    "patch only",
			prev:    "v0.3.0",
			commits: commitsOf("perf: cache profile list", "docs: update README"),
			want:    "v0.3.1",
			release: true,
		},
		{
			name:    "first release",
			prev:    "v0.0.0",
			commits: commitsOf("feat: initial"),
			want:    "v0.1.0",
			release: true,
		},
		{
			name:    "documentation and dependency merges",
			prev:    "v0.3.0",
			commits: commitsOf("docs(openspec): archive change", "chore(deps): bump actions/checkout"),
			want:    "v0.3.0",
		},
		{
			name:    "non-conventional subject",
			prev:    "v0.3.0",
			commits: commitsOf("Update README"),
			want:    "v0.3.0",
		},
		{
			name:    "release commit only",
			prev:    "v0.4.0",
			commits: commitsOf("chore(release): v0.4.0"),
			want:    "v0.4.0",
		},
		{
			name: "no commits",
			prev: "v0.4.0",
			want: "v0.4.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev, err := ParseVersion(tt.prev)
			require.NoError(t, err)
			got, release := NextVersion(prev, tt.commits)
			assert.Equal(t, tt.want, got.String())
			assert.Equal(t, tt.release, release)
		})
	}
}

func TestApplyResetsLowerParts(t *testing.T) {
	assert.Equal(t, "v2.0.0", Version{1, 4, 2}.Apply(BumpMajor).String())
	assert.Equal(t, "v1.5.0", Version{1, 4, 2}.Apply(BumpMinor).String())
	assert.Equal(t, "v1.4.3", Version{1, 4, 2}.Apply(BumpPatch).String())
	assert.Equal(t, "v0.4.0", Version{0, 3, 7}.Apply(BumpMajor).String())
	assert.Equal(t, "v1.4.2", Version{1, 4, 2}.Apply(BumpNone).String())
}
