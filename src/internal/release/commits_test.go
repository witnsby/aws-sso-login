package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommit(t *testing.T) {
	tests := []struct {
		name         string
		subject      string
		body         string
		conventional bool
		typ          string
		scope        string
		description  string
		breaking     bool
		bump         Bump
	}{
		{name: "feature", subject: "feat: add --region", conventional: true, typ: "feat", description: "add --region", bump: BumpMinor},
		{name: "scoped fix", subject: "fix(cli): handle empty cache", conventional: true, typ: "fix", scope: "cli", description: "handle empty cache", bump: BumpPatch},
		{name: "perf", subject: "perf: cache profile list", conventional: true, typ: "perf", description: "cache profile list", bump: BumpPatch},
		{name: "revert", subject: "revert: undo picker height", conventional: true, typ: "revert", description: "undo picker height", bump: BumpPatch},
		{name: "upper-case type", subject: "Fix: typo in message", conventional: true, typ: "fix", description: "typo in message", bump: BumpPatch},
		{name: "breaking bang", subject: "feat(cli)!: rename --profile", conventional: true, typ: "feat", scope: "cli", description: "rename --profile", breaking: true, bump: BumpMajor},
		{name: "breaking footer", subject: "fix: exit codes", body: "Details.\n\nBREAKING CHANGE: exit codes changed", conventional: true, typ: "fix", description: "exit codes", breaking: true, bump: BumpMajor},
		{name: "breaking hyphen footer", subject: "refactor: drop flag", body: "BREAKING-CHANGE: --old removed", conventional: true, typ: "refactor", description: "drop flag", breaking: true, bump: BumpMajor},
		{name: "breaking docs", subject: "docs!: move docs", conventional: true, typ: "docs", description: "move docs", breaking: true, bump: BumpMajor},
		{name: "docs", subject: "docs(openspec): archive change", conventional: true, typ: "docs", scope: "openspec", description: "archive change", bump: BumpNone},
		{name: "dependabot", subject: "chore(deps): bump actions/checkout from 4 to 7", conventional: true, typ: "chore", scope: "deps", description: "bump actions/checkout from 4 to 7", bump: BumpNone},
		{name: "ci", subject: "ci: publish release notes", conventional: true, typ: "ci", description: "publish release notes", bump: BumpNone},
		{name: "release commit", subject: "chore(release): v0.4.0", conventional: true, typ: "chore", scope: "release", description: "v0.4.0", bump: BumpNone},
		{name: "breaking release commit", subject: "chore(release)!: v1.0.0", conventional: true, typ: "chore", scope: "release", description: "v1.0.0", breaking: true, bump: BumpNone},
		{name: "plain subject", subject: "Update README", bump: BumpNone},
		{name: "merge subject", subject: "Merge pull request #50 from witnsby/branch", bump: BumpNone},
		{name: "missing space", subject: "feat:add flag", bump: BumpNone},
		{name: "empty description", subject: "feat: ", bump: BumpNone},
		{name: "footer in plain commit ignored", subject: "Update README", body: "BREAKING CHANGE: none", bump: BumpNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ParseCommit("abc", tt.subject, tt.body)
			assert.Equal(t, tt.conventional, c.Conventional)
			assert.Equal(t, tt.typ, c.Type)
			assert.Equal(t, tt.scope, c.Scope)
			assert.Equal(t, tt.description, c.Description)
			assert.Equal(t, tt.breaking, c.Breaking)
			assert.Equal(t, tt.bump, c.Bump())
			assert.Equal(t, tt.bump != BumpNone, c.Releasing())
		})
	}
}

func TestCommitText(t *testing.T) {
	assert.Equal(t, "**cli:** add flag", ParseCommit("", "feat(cli): add flag", "").Text())
	assert.Equal(t, "add flag", ParseCommit("", "feat: add flag", "").Text())
}

func TestBumpString(t *testing.T) {
	assert.Equal(t, "none", BumpNone.String())
	assert.Equal(t, "patch", BumpPatch.String())
	assert.Equal(t, "minor", BumpMinor.String())
	assert.Equal(t, "major", BumpMajor.String())
}
