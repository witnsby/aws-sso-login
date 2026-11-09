package release

import (
	"fmt"
	"regexp"
	"strconv"
)

// Version is a release version vMAJOR.MINOR.PATCH.
type Version struct {
	Major, Minor, Patch int
}

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// ParseVersion parses a tag such as "v1.2.3". Pre-release and build
// metadata suffixes are rejected.
func ParseVersion(s string) (Version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid version %q: want vMAJOR.MINOR.PATCH", s)
	}
	var v Version
	var err error
	if v.Major, err = strconv.Atoi(m[1]); err != nil {
		return Version{}, fmt.Errorf("invalid major version in %q: %w", s, err)
	}
	if v.Minor, err = strconv.Atoi(m[2]); err != nil {
		return Version{}, fmt.Errorf("invalid minor version in %q: %w", s, err)
	}
	if v.Patch, err = strconv.Atoi(m[3]); err != nil {
		return Version{}, fmt.Errorf("invalid patch version in %q: %w", s, err)
	}
	return v, nil
}

// String formats the version as a tag, e.g. "v1.2.3".
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less reports whether v sorts before o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// Apply returns v incremented by b. While the major version is 0, a major
// bump increments the minor version instead.
func (v Version) Apply(b Bump) Version {
	if b == BumpMajor && v.Major == 0 {
		b = BumpMinor
	}
	switch b {
	case BumpMajor:
		return Version{Major: v.Major + 1}
	case BumpMinor:
		return Version{Major: v.Major, Minor: v.Minor + 1}
	case BumpPatch:
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}

// HighestBump returns the largest bump any of the commits calls for.
func HighestBump(commits []Commit) Bump {
	highest := BumpNone
	for _, c := range commits {
		if b := c.Bump(); b > highest {
			highest = b
		}
	}
	return highest
}

// NextVersion returns the version after prev for the given commits and
// whether a release is due at all.
func NextVersion(prev Version, commits []Commit) (Version, bool) {
	b := HighestBump(commits)
	if b == BumpNone {
		return prev, false
	}
	return prev.Apply(b), true
}
