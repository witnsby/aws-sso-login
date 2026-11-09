// Package release holds the logic behind the release automation: it reads
// Conventional Commits, computes the next version, promotes the changelog and
// renders release notes. It is used by src/cmd/release in CI and is not part
// of the aws-sso-login binary.
package release

import (
	"regexp"
	"strings"
)

// Bump is the semantic version increment a set of commits calls for.
type Bump int

// Bumps in increasing order, so the highest one wins with a plain comparison.
const (
	BumpNone Bump = iota
	BumpPatch
	BumpMinor
	BumpMajor
)

// String returns the bump name used in diagnostics.
func (b Bump) String() string {
	switch b {
	case BumpPatch:
		return "patch"
	case BumpMinor:
		return "minor"
	case BumpMajor:
		return "major"
	default:
		return "none"
	}
}

// Commit is one non-merge commit, parsed as a Conventional Commit when its
// subject follows the format.
type Commit struct {
	Hash    string
	Subject string
	Body    string

	// Conventional reports whether Subject is "type(scope)!: description".
	Conventional bool
	Type         string // lower-case type, e.g. "feat"
	Scope        string
	Description  string
	Breaking     bool
}

var (
	subjectPattern  = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^()]*)\))?(!)?: +(\S.*)$`)
	breakingFooters = []string{"BREAKING CHANGE:", "BREAKING-CHANGE:"}
)

// ParseCommit parses a commit subject and body.
func ParseCommit(hash, subject, body string) Commit {
	c := Commit{Hash: hash, Subject: strings.TrimSpace(subject), Body: body}
	m := subjectPattern.FindStringSubmatch(c.Subject)
	if m == nil {
		return c
	}
	c.Conventional = true
	c.Type = strings.ToLower(m[1])
	c.Scope = strings.TrimSpace(m[2])
	c.Breaking = m[3] == "!" || hasBreakingFooter(body)
	c.Description = strings.TrimSpace(m[4])
	return c
}

func hasBreakingFooter(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		for _, footer := range breakingFooters {
			if strings.HasPrefix(line, footer) {
				return true
			}
		}
	}
	return false
}

// IsReleaseCommit reports whether c is a commit made by the release
// automation itself ("chore(release): vX.Y.Z").
func (c Commit) IsReleaseCommit() bool {
	return c.Conventional && c.Type == "chore" && c.Scope == "release"
}

// Bump returns the version increment this commit calls for on its own.
func (c Commit) Bump() Bump {
	if !c.Conventional || c.IsReleaseCommit() {
		return BumpNone
	}
	if c.Breaking {
		return BumpMajor
	}
	switch c.Type {
	case "feat":
		return BumpMinor
	case "fix", "perf", "revert":
		return BumpPatch
	default:
		return BumpNone
	}
}

// Releasing reports whether the commit alone would trigger a release.
func (c Commit) Releasing() bool { return c.Bump() != BumpNone }

// Text returns the description prefixed with the bold scope, as used in
// release notes and generated changelog entries.
func (c Commit) Text() string {
	if c.Scope == "" {
		return c.Description
	}
	return "**" + c.Scope + ":** " + c.Description
}
